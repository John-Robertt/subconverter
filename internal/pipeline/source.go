package pipeline

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/fetch"
	"github.com/John-Robertt/subconverter/internal/model"
	"github.com/John-Robertt/subconverter/internal/proxyparse"
)

// defaultFetchOrder is the fallback traversal order when Sources.FetchOrder
// is empty (in-memory Config instances from tests that skip YAML unmarshal).
// YAML-loaded configs populate FetchOrder from declaration order instead.
var defaultFetchOrder = []string{"subscriptions", "snell", "vless"}

// ChainTemplate is the runtime form of a relay_through custom proxy. It is
// produced by Source and expanded into chained proxies by Group.
type ChainTemplate struct {
	Name         string
	Type         string
	Server       string
	Port         int
	Params       map[string]string
	Plugin       *model.Plugin
	RelayThrough *config.PreparedRelayThrough
}

// SourceResult is the output of the Source stage. Proxies contains all
// original proxies (fetched + standalone custom); ChainTemplates carries
// relay_through declarations for the Group stage. NextProxyID is the first
// identity not allocated by Source and survives filtering so Group never
// reuses an identity that belonged to an excluded original proxy.
type SourceResult struct {
	Proxies        []model.Proxy
	ChainTemplates []ChainTemplate
	NextProxyID    model.ProxyID
}

// sourcePrepared executes the request-time Source stage against startup-
// prepared source declarations.
//
// Traversal order within the three fetch-kinds follows sources.FetchOrder,
// which reflects the YAML declaration order (e.g. `snell:` then `vless:`
// then `subscriptions:` yields that exact node ordering in the output). When
// FetchOrder is empty (unit-test Configs built in memory), a deterministic
// default is used.
//
// Sources are processed sequentially to guarantee deterministic ProxyID
// assignment. Fetch errors are fail-fast; individual malformed SS/AnyTLS lines
// within a subscription are skipped, but a subscription yielding zero valid nodes
// is an error. VLESS and Snell sources abort on any single-line parse
// failure (small hand-curated lists — strict failure surfaces typos early).
func sourcePrepared(ctx context.Context, sources config.PreparedSources, fetcher fetch.Fetcher) (*SourceResult, error) {
	var fetchedProxies []model.Proxy

	order := sources.FetchOrder
	if len(order) == 0 {
		order = defaultFetchOrder
	}

	for _, kind := range order {
		switch kind {
		case "subscriptions":
			for index, sub := range sources.Subscriptions {
				proxies, err := loadSubscription(ctx, fetcher, sub)
				if err != nil {
					if sub.Content != "" {
						var sourceErr *errtype.FetchError
						if errors.As(err, &sourceErr) {
							located := *sourceErr
							located.Message = fmt.Sprintf("sources.subscriptions[%d].content：%s", index, sourceErr.Message)
							return nil, &located
						}
					}
					return nil, err
				}
				fetchedProxies = append(fetchedProxies, proxies...)
			}
		case "snell", "vless":
			entries := sources.Snell
			if kind == "vless" {
				entries = sources.VLess
			}
			for index, source := range entries {
				location, body, err := loadSourceBody(ctx, fetcher, source, fmt.Sprintf("sources.%s[%d].content", kind, index))
				if err != nil {
					return nil, err
				}
				proxies, err := parseNodePool(kind, location, body)
				if err != nil {
					return nil, err
				}
				fetchedProxies = append(fetchedProxies, proxies...)
			}
		default:
			// FetchOrder should only ever contain keys from sourceFetchKeys
			// (enforced by Sources.UnmarshalYAML). Reaching this branch
			// means an in-memory Config set FetchOrder directly with a
			// typo/unknown kind — surface it instead of silently dropping
			// the source.
			return nil, &errtype.BuildError{
				Code:    errtype.CodeBuildValidationFailed,
				Phase:   "source",
				Message: fmt.Sprintf("unknown fetch-kind %q in Sources.FetchOrder (check Sources.UnmarshalYAML and defaultFetchOrder)", kind),
			}
		}
	}

	customProxies, chainTemplates := convertCustomProxies(sources.CustomProxies)

	proxies := make([]model.Proxy, 0, len(fetchedProxies)+len(customProxies))
	proxies = append(proxies, fetchedProxies...)
	proxies = append(proxies, customProxies...)
	proxies = assignProxyIDs(proxies)
	if err := validateGeneratedProxies("source", proxies); err != nil {
		return nil, err
	}
	return &SourceResult{
		Proxies:        proxies,
		ChainTemplates: chainTemplates,
		NextProxyID:    model.ProxyID(len(proxies) + 1),
	}, nil
}

func assignProxyIDs(proxies []model.Proxy) []model.Proxy {
	result := make([]model.Proxy, len(proxies))
	for index, proxy := range proxies {
		proxy.ID = model.ProxyID(index + 1)
		result[index] = proxy
	}
	return result
}

// loadSubscription reads inline/local content or fetches a subscription URL. Structured Clash YAML and
// Surge CONF / Loon documents are recognized before the legacy Base64/plain-text
// subscription list formats.
func loadSubscription(ctx context.Context, fetcher fetch.Fetcher, sub config.Source) ([]model.Proxy, error) {
	rawURL, body, err := loadSourceBody(ctx, fetcher, sub, "inline:subscription")
	if err != nil {
		return nil, err
	}
	proxies, _, err := parseSubscriptionBody(rawURL, body, sub.Format, sub.Content != "" || sub.ContentFile != "")
	return proxies, err
}

// loadSourceBody selects exactly one prepared input; inline and local sources never use the remote Fetcher.
func loadSourceBody(ctx context.Context, fetcher fetch.Fetcher, source config.Source, inlineLocation string) (string, []byte, error) {
	if source.ContentFile != "" {
		body, err := fetch.LoadResource(ctx, source.ContentFile, fetcher)
		return source.ContentFile, body, err
	}
	if source.Content != "" {
		return inlineLocation, []byte(source.Content), nil
	}
	body, err := fetcher.Fetch(ctx, source.URL)
	return source.URL, body, err
}

// parseNodePool applies the same strict protocol parser to URL, local and edited bodies.
func parseNodePool(kind, location string, body []byte) ([]model.Proxy, error) {
	switch kind {
	case "snell":
		return parseSnellSource(location, body)
	case "vless":
		return parseVLessSource(location, body)
	default:
		return nil, &errtype.BuildError{Code: errtype.CodeBuildValidationFailed, Phase: "source", Message: "节点池类型无效：" + kind}
	}
}

func parseSubscriptionBody(rawURL string, body []byte, format string, allowFragment bool) ([]model.Proxy, string, error) {
	// Pasted INI node declarations need no document wrapper, including saved copies.
	if allowFragment && (format == "loon" || format == "surge") && !hasINISection(string(body)) {
		body = append([]byte("[Proxy]\n"), body...)
	}

	document, err := parseSubscriptionDocument(rawURL, body, format)
	if document != nil {
		if err != nil {
			return nil, "", &errtype.FetchError{
				Code:    errtype.CodeFetchSubscriptionFormatInvalid,
				URL:     fetch.SanitizeURL(rawURL),
				Message: subscriptionDocumentFormatLabel(document.Format) + " 订阅格式无效：" + err.Error(),
				Cause:   err,
			}
		}
		if len(document.Proxies) == 0 {
			return nil, "", &errtype.FetchError{
				Code:    errtype.CodeFetchSubscriptionEmpty,
				URL:     fetch.SanitizeURL(rawURL),
				Message: subscriptionDocumentFormatLabel(document.Format) + " 订阅未产生任何有效节点",
			}
		}
		return document.Proxies, string(document.Format), nil
	}

	text, err := subscriptionBodyText(body)
	if err != nil {
		return nil, "", &errtype.FetchError{
			Code:    errtype.CodeFetchSubscriptionBase64Invalid,
			URL:     fetch.SanitizeURL(rawURL),
			Message: "订阅内容不是合法的 Clash YAML、Surge CONF、Loon CONF、Base64，也不是明文 SS/AnyTLS 订阅列表",
			Cause:   err,
		}
	}

	lines := splitSubscriptionLines(text)
	var proxies []model.Proxy
	for _, line := range lines {
		proxy, err := ParseSubscriptionLine(line)
		if err != nil {
			// Skip individual malformed subscription lines.
			continue
		}
		proxies = append(proxies, proxy)
	}

	if len(proxies) == 0 {
		return nil, "", &errtype.FetchError{
			Code:    errtype.CodeFetchSubscriptionEmpty,
			URL:     fetch.SanitizeURL(rawURL),
			Message: "订阅未产生任何有效节点",
		}
	}

	return proxies, "text", nil
}

func subscriptionDocumentFormatLabel(format model.NativeProxyFormat) string {
	switch format {
	case model.NativeProxyFormatClash:
		return "Clash YAML"
	case model.NativeProxyFormatSurge:
		return "Surge CONF"
	case model.NativeProxyFormatLoon:
		return "Loon CONF"
	default:
		return "结构化"
	}
}

// parseSnellSource parses one Surge-style Snell declaration per line.
// Blank/comment lines are skipped; any other invalid line aborts the source and retains its physical line number.
func parseSnellSource(location string, body []byte) ([]model.Proxy, error) {
	var proxies []model.Proxy
	sanitizedLocation := fetch.SanitizeURL(location)
	for lineNo, rawLine := range strings.Split(string(body), "\n") {
		line := strings.TrimSpace(rawLine)
		proxy, parseErr := ParseSnellSurgeLine(line)
		if parseErr != nil {
			if errors.Is(parseErr, errSnellLineSkip) {
				continue
			}
			return nil, wrapSnellSourceParseError(sanitizedLocation, lineNo+1, parseErr)
		}
		proxies = append(proxies, proxy)
	}

	if len(proxies) == 0 {
		return nil, &errtype.FetchError{
			Code:    errtype.CodeFetchSubscriptionEmpty,
			URL:     fetch.SanitizeURL(location),
			Message: "Snell 来源未产生任何有效节点",
		}
	}

	return proxies, nil
}

func wrapSnellSourceParseError(sanitizedLocation string, lineNo int, parseErr error) error {
	detail := parseErr.Error()
	var buildErr *errtype.BuildError
	if errors.As(parseErr, &buildErr) {
		detail = buildErr.Message
	}

	return &errtype.BuildError{
		Code:    errtype.CodeBuildSnellLineInvalid,
		Phase:   "source",
		Message: fmt.Sprintf(`Snell 来源 %q 第 %d 行解析失败：%s`, sanitizedLocation, lineNo, detail),
		Cause:   parseErr,
	}
}

// parseVLessSource parses one VLESS URI per line with the same strict failure policy as Snell.
func parseVLessSource(location string, body []byte) ([]model.Proxy, error) {
	var proxies []model.Proxy
	sanitizedLocation := fetch.SanitizeURL(location)
	for lineNo, rawLine := range strings.Split(string(body), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		proxy, parseErr := ParseVLessURI(line)
		if parseErr != nil {
			return nil, wrapVLessSourceParseError(sanitizedLocation, lineNo+1, parseErr)
		}
		proxies = append(proxies, proxy)
	}

	if len(proxies) == 0 {
		return nil, &errtype.FetchError{
			Code:    errtype.CodeFetchSubscriptionEmpty,
			URL:     fetch.SanitizeURL(location),
			Message: "VLESS 来源未产生任何有效节点",
		}
	}

	return proxies, nil
}

func wrapVLessSourceParseError(sanitizedLocation string, lineNo int, parseErr error) error {
	detail := parseErr.Error()
	var buildErr *errtype.BuildError
	if errors.As(parseErr, &buildErr) {
		detail = buildErr.Message
	}

	return &errtype.BuildError{
		Code:    errtype.CodeBuildVLessSourceLineInvalid,
		Phase:   "source",
		Message: fmt.Sprintf(`VLESS 来源 %q 第 %d 行解析失败：%s`, sanitizedLocation, lineNo, detail),
		Cause:   parseErr,
	}
}

// subscriptionBodyText accepts both conventional base64 subscriptions and
// plain-text lists in the supported SS/AnyTLS line formats.
func subscriptionBodyText(body []byte) (string, error) {
	s := strings.TrimSpace(string(body))
	if s == "" {
		return "", nil
	}
	decoded, err := decodeBase64(s)
	if err == nil {
		return decoded, nil
	}
	if looksLikePlainSubscription(s) {
		return s, nil
	}
	return "", err
}

func looksLikePlainSubscription(text string) bool {
	for _, rawLine := range strings.Split(text, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		if isPlainSubscriptionLine(line) {
			return true
		}
	}
	return false
}

// splitSubscriptionLines trims whitespace and skips empty/comment lines.
func splitSubscriptionLines(text string) []string {
	raw := strings.Split(text, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

// convertCustomProxies converts startup-prepared custom proxies into either
// standalone custom proxies or chain templates for later expansion in Group.
func convertCustomProxies(cps []config.PreparedCustomProxy) ([]model.Proxy, []ChainTemplate) {
	proxies := make([]model.Proxy, 0, len(cps))
	chainTemplates := make([]ChainTemplate, 0, len(cps))
	for _, cp := range cps {
		if cp.RelayThrough != nil {
			chainTemplates = append(chainTemplates, ChainTemplate{
				Name:         cp.Name,
				Type:         cp.Parsed.Type,
				Server:       cp.Parsed.Server,
				Port:         cp.Parsed.Port,
				Params:       copyParams(cp.Parsed.Params),
				Plugin:       convertPluginSpec(cp.Parsed.Plugin),
				RelayThrough: cp.RelayThrough,
			})
			continue
		}
		proxies = append(proxies, model.Proxy{
			Name:   cp.Name,
			Type:   cp.Parsed.Type,
			Server: cp.Parsed.Server,
			Port:   cp.Parsed.Port,
			Params: copyParams(cp.Parsed.Params),
			Plugin: convertPluginSpec(cp.Parsed.Plugin),
			Kind:   model.KindCustom,
		})
	}
	return proxies, chainTemplates
}

// copyParams returns a shallow copy of a Params map to prevent sharing
// between proxies. Used by both Source stage and Group stage.
func copyParams(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func copyPlugin(src *model.Plugin) *model.Plugin {
	if src == nil {
		return nil
	}
	dst := &model.Plugin{Name: src.Name}
	if len(src.Opts) > 0 {
		dst.Opts = make(map[string]string, len(src.Opts))
		for k, v := range src.Opts {
			dst.Opts[k] = v
		}
	}
	return dst
}

func convertPluginSpec(src *proxyparse.PluginSpec) *model.Plugin {
	if src == nil {
		return nil
	}
	dst := &model.Plugin{Name: src.Name}
	if len(src.Opts) > 0 {
		dst.Opts = make(map[string]string, len(src.Opts))
		for k, v := range src.Opts {
			dst.Opts[k] = v
		}
	}
	return dst
}

func hasINISection(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		if isINISectionHeader(strings.TrimPrefix(strings.TrimSpace(line), "\ufeff")) {
			return true
		}
	}
	return false
}
