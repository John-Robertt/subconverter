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
)

// defaultFetchOrder is the fallback traversal order when Sources.FetchOrder
// is empty (in-memory Config instances from tests that skip YAML unmarshal).
// YAML-loaded configs populate FetchOrder from declaration order instead.
var defaultFetchOrder = []string{"subscriptions", "snell", "vless"}

// Source executes pipeline stage 3: fetch subscriptions / Snell / VLESS
// sources, convert custom proxies, and deduplicate names.
//
// Traversal order within the three fetch-kinds follows cfg.Sources.FetchOrder,
// which reflects the YAML declaration order (e.g. `snell:` then `vless:`
// then `subscriptions:` yields that exact node ordering in the output). When
// FetchOrder is empty (unit-test Configs built in memory), a deterministic
// default is used.
//
// Sources are processed sequentially to guarantee deterministic dedup
// suffixes. Fetch errors are fail-fast; individual malformed SS URIs within
// a subscription are skipped, but a subscription yielding zero valid nodes
// is an error. VLESS and Snell sources abort on any single-line parse
// failure (small hand-curated lists — strict failure surfaces typos early).
func Source(ctx context.Context, cfg *config.Config, fetcher fetch.Fetcher) ([]model.Proxy, error) {
	var subProxies []model.Proxy

	order := cfg.Sources.FetchOrder
	if len(order) == 0 {
		order = defaultFetchOrder
	}

	for _, kind := range order {
		switch kind {
		case "subscriptions":
			for _, sub := range cfg.Sources.Subscriptions {
				proxies, err := fetchSubscription(ctx, fetcher, sub.URL)
				if err != nil {
					return nil, err
				}
				subProxies = append(subProxies, proxies...)
			}
		case "snell":
			for _, s := range cfg.Sources.Snell {
				proxies, err := fetchSnellSource(ctx, fetcher, s.URL)
				if err != nil {
					return nil, err
				}
				subProxies = append(subProxies, proxies...)
			}
		case "vless":
			for _, s := range cfg.Sources.VLess {
				proxies, err := fetchVLessSource(ctx, fetcher, s.URL)
				if err != nil {
					return nil, err
				}
				subProxies = append(subProxies, proxies...)
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

	subProxies = deduplicateNames(subProxies)

	// Verify name conflicts against the *original* custom_proxies (including
	// chain templates with relay_through set), since chain templates do not
	// produce KindCustom proxies but their name still claims a slot in the
	// shared namespace as a chain group name.
	if err := checkNameConflicts(subProxies, cfg.Sources.CustomProxies); err != nil {
		return nil, err
	}

	customProxies := convertCustomProxies(cfg.Sources.CustomProxies)

	return append(subProxies, customProxies...), nil
}

// fetchSubscription fetches a single subscription URL, decodes the base64
// response, and parses each line as an SS URI.
func fetchSubscription(ctx context.Context, fetcher fetch.Fetcher, rawURL string) ([]model.Proxy, error) {
	body, err := fetcher.Fetch(ctx, rawURL)
	if err != nil {
		return nil, err
	}

	decoded, err := decodeSubscriptionBody(body)
	if err != nil {
		return nil, &errtype.FetchError{
			Code:    errtype.CodeFetchSubscriptionBase64Invalid,
			URL:     fetch.SanitizeURL(rawURL),
			Message: "订阅内容不是合法的 Base64",
			Cause:   err,
		}
	}

	lines := splitLines(decoded)
	var proxies []model.Proxy
	for _, line := range lines {
		proxy, err := ParseSSURI(line)
		if err != nil {
			// Skip individual malformed URIs.
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

// decodeSubscriptionBody decodes a base64-encoded subscription response body.
func decodeSubscriptionBody(body []byte) (string, error) {
	s := strings.TrimSpace(string(body))
	if s == "" {
		return "", nil
	}
	return decodeBase64(s)
}

// splitLines splits text on newlines, trims whitespace, and skips empty lines.
func splitLines(text string) []string {
	raw := strings.Split(text, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// deduplicateNames renames duplicate proxy names by appending circled number
// suffixes (②, ③, ...) for the 2nd through 10th occurrence, then (N) beyond.
// Candidate names are always derived from the original base name so collisions
// with natural names still advance to the next logical suffix (e.g. "HK-01",
// "HK-01②", "HK-01③").
func deduplicateNames(proxies []model.Proxy) []model.Proxy {
	nextSeq := make(map[string]int, len(proxies))
	used := make(map[string]struct{}, len(proxies))
	result := make([]model.Proxy, 0, len(proxies))

	for _, p := range proxies {
		seq := nextSeq[p.Name] + 1
		candidate := deduplicatedName(p.Name, seq)
		for {
			if _, exists := used[candidate]; !exists {
				break
			}
			seq++
			candidate = deduplicatedName(p.Name, seq)
		}

		deduped := p
		deduped.Name = candidate
		result = append(result, deduped)

		nextSeq[p.Name] = seq
		used[candidate] = struct{}{}
	}

	return result
}

func deduplicatedName(base string, seq int) string {
	if seq <= 1 {
		return base
	}

	circled := []string{"②", "③", "④", "⑤", "⑥", "⑦", "⑧", "⑨", "⑩"}
	if seq-2 < len(circled) {
		return base + circled[seq-2]
	}

	return fmt.Sprintf("%s(%d)", base, seq)
}

// convertCustomProxies converts config.CustomProxy entries to model.Proxy objects.
//
// Entries with RelayThrough set are *skipped*: they serve only as chain
// templates in the Group stage (which generates chained nodes + a chain group
// named cp.Name). Emitting a KindCustom proxy for them would collide with the
// chain group name in the shared namespace.
func convertCustomProxies(cps []config.CustomProxy) []model.Proxy {
	proxies := make([]model.Proxy, 0, len(cps))
	for _, cp := range cps {
		if cp.RelayThrough != nil {
			continue
		}
		proxies = append(proxies, model.Proxy{
			Name:   cp.Name,
			Type:   cp.Type,
			Server: cp.Server,
			Port:   cp.Port,
			Params: copyParams(cp.Params),
			Plugin: copyPlugin(cp.Plugin),
			Kind:   model.KindCustom,
		})
	}
	return proxies
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

// checkNameConflicts verifies that no custom_proxy name collides with a
// fetched node name (subscription / Snell / VLESS). The error message
// distinguishes two cases because they belong to *different* namespaces and
// the user has to fix different sections of the YAML:
//
//   - cp without relay_through: cp.Name is a direct KindCustom proxy name,
//     colliding with a fetched proxy name.
//   - cp with relay_through: cp.Name is a chain group name (no KindCustom
//     proxy is emitted), colliding with a fetched proxy name — the fix is
//     to rename either the custom_proxy or filter out the fetched node.
func checkNameConflicts(fetched []model.Proxy, customProxies []config.CustomProxy) error {
	fetchedIndex := make(map[string]model.ProxyKind, len(fetched))
	for _, p := range fetched {
		fetchedIndex[p.Name] = p.Kind
	}

	for _, cp := range customProxies {
		kind, exists := fetchedIndex[cp.Name]
		if !exists {
			continue
		}
		label := "自定义代理名"
		if cp.RelayThrough != nil {
			label = "链式组名"
		}
		return &errtype.BuildError{
			Code:    errtype.CodeBuildCustomNameConflict,
			Phase:   "source",
			Message: fmt.Sprintf("%s %q 与%s节点重名", label, cp.Name, describeFetchedKind(kind)),
		}
	}
	return nil
}

// describeFetchedKind returns a Chinese label for a fetched-kind proxy used
// in user-facing error messages.
func describeFetchedKind(k model.ProxyKind) string {
	switch k {
	case model.KindSubscription:
		return "订阅"
	case model.KindSnell:
		return "Snell 来源"
	case model.KindVLess:
		return "VLESS 来源"
	default:
		return "远程拉取"
	}
}
