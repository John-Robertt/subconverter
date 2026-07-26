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
// relay_through declarations for the Group stage.
type SourceResult struct {
	Proxies        []model.Proxy
	ChainTemplates []ChainTemplate
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
			for _, sub := range sources.Subscriptions {
				proxies, err := fetchSubscription(ctx, fetcher, sub.URL)
				if err != nil {
					return nil, err
				}
				fetchedProxies = append(fetchedProxies, proxies...)
			}
		case "snell":
			for _, s := range sources.Snell {
				proxies, err := fetchSnellSource(ctx, fetcher, s.URL)
				if err != nil {
					return nil, err
				}
				fetchedProxies = append(fetchedProxies, proxies...)
			}
		case "vless":
			for _, s := range sources.VLess {
				proxies, err := fetchVLessSource(ctx, fetcher, s.URL)
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
	return &SourceResult{Proxies: proxies, ChainTemplates: chainTemplates}, nil
}

func assignProxyIDs(proxies []model.Proxy) []model.Proxy {
	result := make([]model.Proxy, len(proxies))
	for index, proxy := range proxies {
		proxy.ID = model.ProxyID(index + 1)
		result[index] = proxy
	}
	return result
}

// fetchSubscription fetches one subscription URL. Structured Clash YAML and
// Surge CONF documents are recognized before the legacy Base64/plain-text
// subscription list formats.
func fetchSubscription(ctx context.Context, fetcher fetch.Fetcher, rawURL string) ([]model.Proxy, error) {
	body, err := fetcher.Fetch(ctx, rawURL)
	if err != nil {
		return nil, err
	}

	document, err := parseSubscriptionDocument(rawURL, body)
	if document != nil {
		if err != nil {
			return nil, &errtype.FetchError{
				Code:    errtype.CodeFetchSubscriptionFormatInvalid,
				URL:     fetch.SanitizeURL(rawURL),
				Message: subscriptionDocumentFormatLabel(document.Format) + " 订阅格式无效：" + err.Error(),
				Cause:   err,
			}
		}
		if len(document.Proxies) == 0 {
			return nil, &errtype.FetchError{
				Code:    errtype.CodeFetchSubscriptionEmpty,
				URL:     fetch.SanitizeURL(rawURL),
				Message: subscriptionDocumentFormatLabel(document.Format) + " 订阅未产生任何有效节点",
			}
		}
		return document.Proxies, nil
	}

	text, err := subscriptionBodyText(body)
	if err != nil {
		return nil, &errtype.FetchError{
			Code:    errtype.CodeFetchSubscriptionBase64Invalid,
			URL:     fetch.SanitizeURL(rawURL),
			Message: "订阅内容不是合法的 Clash YAML、Surge CONF、Base64，也不是明文 SS/AnyTLS 订阅列表",
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
		return nil, &errtype.FetchError{
			Code:    errtype.CodeFetchSubscriptionEmpty,
			URL:     fetch.SanitizeURL(rawURL),
			Message: "订阅未产生任何有效节点",
		}
	}

	return proxies, nil
}

func subscriptionDocumentFormatLabel(format model.NativeProxyFormat) string {
	switch format {
	case model.NativeProxyFormatClash:
		return "Clash YAML"
	case model.NativeProxyFormatSurge:
		return "Surge CONF"
	default:
		return "结构化"
	}
}

// fetchSnellSource fetches a URL whose body is plain text with one Surge-style
// Snell proxy declaration per line. Blank lines and comments (#, //) are
// skipped; malformed Snell lines abort the entire source with a BuildError,
// because Snell sources are small hand-curated lists where silently dropping
// a line is more likely to mask a typo than a real transient issue.
//
// A source yielding zero valid nodes is reported as CodeFetchSubscriptionEmpty,
// mirroring the subscription path.
//
// The body is scanned line-by-line so parse failures can report physical line
// numbers from the original source text. Each line is still trimmed before
// parsing; blank lines and comments are skipped via errSnellLineSkip.
func fetchSnellSource(ctx context.Context, fetcher fetch.Fetcher, rawURL string) ([]model.Proxy, error) {
	body, err := fetcher.Fetch(ctx, rawURL)
	if err != nil {
		return nil, err
	}

	var proxies []model.Proxy
	sanitizedURL := fetch.SanitizeURL(rawURL)
	for lineNo, rawLine := range strings.Split(string(body), "\n") {
		line := strings.TrimSpace(rawLine)
		proxy, parseErr := ParseSnellSurgeLine(line)
		if parseErr != nil {
			if errors.Is(parseErr, errSnellLineSkip) {
				continue
			}
			return nil, wrapSnellSourceParseError(sanitizedURL, lineNo+1, parseErr)
		}
		proxies = append(proxies, proxy)
	}

	if len(proxies) == 0 {
		return nil, &errtype.FetchError{
			Code:    errtype.CodeFetchSubscriptionEmpty,
			URL:     fetch.SanitizeURL(rawURL),
			Message: "Snell 来源未产生任何有效节点",
		}
	}

	return proxies, nil
}

func wrapSnellSourceParseError(sanitizedURL string, lineNo int, parseErr error) error {
	detail := parseErr.Error()
	var buildErr *errtype.BuildError
	if errors.As(parseErr, &buildErr) {
		detail = buildErr.Message
	}

	return &errtype.BuildError{
		Code:    errtype.CodeBuildSnellLineInvalid,
		Phase:   "source",
		Message: fmt.Sprintf(`Snell 来源 %q 第 %d 行解析失败：%s`, sanitizedURL, lineNo, detail),
		Cause:   parseErr,
	}
}

// fetchVLessSource fetches a URL whose body is plain text with one
// `vless://` URI per line. Blank lines and comment lines (`#`, `//`) are
// skipped; any other line that fails to parse aborts the entire source
// with a BuildError, matching the strict Snell behaviour.
//
// A source yielding zero valid nodes is reported as CodeFetchSubscriptionEmpty,
// mirroring the subscription and Snell paths.
//
// Parse-failure errors carry the 1-based physical line number from the
// original body and the sanitized URL (subscription tokens redacted).
func fetchVLessSource(ctx context.Context, fetcher fetch.Fetcher, rawURL string) ([]model.Proxy, error) {
	body, err := fetcher.Fetch(ctx, rawURL)
	if err != nil {
		return nil, err
	}

	var proxies []model.Proxy
	sanitizedURL := fetch.SanitizeURL(rawURL)
	for lineNo, rawLine := range strings.Split(string(body), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		proxy, parseErr := ParseVLessURI(line)
		if parseErr != nil {
			return nil, wrapVLessSourceParseError(sanitizedURL, lineNo+1, parseErr)
		}
		proxies = append(proxies, proxy)
	}

	if len(proxies) == 0 {
		return nil, &errtype.FetchError{
			Code:    errtype.CodeFetchSubscriptionEmpty,
			URL:     fetch.SanitizeURL(rawURL),
			Message: "VLESS 来源未产生任何有效节点",
		}
	}

	return proxies, nil
}

func wrapVLessSourceParseError(sanitizedURL string, lineNo int, parseErr error) error {
	detail := parseErr.Error()
	var buildErr *errtype.BuildError
	if errors.As(parseErr, &buildErr) {
		detail = buildErr.Message
	}

	return &errtype.BuildError{
		Code:    errtype.CodeBuildVLessSourceLineInvalid,
		Phase:   "source",
		Message: fmt.Sprintf(`VLESS 来源 %q 第 %d 行解析失败：%s`, sanitizedURL, lineNo, detail),
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
