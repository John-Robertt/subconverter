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

// Source executes pipeline stage 3: fetch subscriptions, parse SS URIs,
// convert custom proxies, and deduplicate names.
//
// It processes subscriptions sequentially in declaration order to guarantee
// deterministic deduplication suffixes. Fetch errors are fail-fast; individual
// malformed SS URIs within a subscription are skipped, but a subscription
// yielding zero valid nodes is an error.
func Source(ctx context.Context, cfg *config.Config, fetcher fetch.Fetcher) ([]model.Proxy, error) {
	var subProxies []model.Proxy

	for _, sub := range cfg.Sources.Subscriptions {
		proxies, err := fetchSubscription(ctx, fetcher, sub.URL)
		if err != nil {
			return nil, err
		}
		subProxies = append(subProxies, proxies...)
	}

	for _, s := range cfg.Sources.Snell {
		proxies, err := fetchSnellSource(ctx, fetcher, s.URL)
		if err != nil {
			return nil, err
		}
		subProxies = append(subProxies, proxies...)
	}

	subProxies = deduplicateNames(subProxies)

	customProxies := convertCustomProxies(cfg.Sources.CustomProxies)

	if err := checkNameConflicts(subProxies, customProxies); err != nil {
		return nil, err
	}

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
// RelayThrough is not processed here (deferred to Group stage in M3).
func convertCustomProxies(cps []config.CustomProxy) []model.Proxy {
	proxies := make([]model.Proxy, 0, len(cps))
	for _, cp := range cps {
		proxies = append(proxies, model.Proxy{
			Name:   cp.Name,
			Type:   cp.Type,
			Server: cp.Server,
			Port:   cp.Port,
			Params: buildCustomParams(cp),
			Kind:   model.KindCustom,
		})
	}
	return proxies
}

// buildCustomParams builds a fresh Params map from a CustomProxy config.
// Each call allocates a new map to prevent sharing between proxies.
// Used by both Source stage (convertCustomProxies) and Group stage (chained node generation).
func buildCustomParams(cp config.CustomProxy) map[string]string {
	params := make(map[string]string)
	if cp.Username != "" {
		params["username"] = cp.Username
	}
	if cp.Password != "" {
		params["password"] = cp.Password
	}
	return params
}

// checkNameConflicts verifies that no custom proxy name collides with a
// fetched node name (subscription or Snell source). The error message
// identifies which kind of fetched source owns the conflicting name so users
// can fix the right side.
func checkNameConflicts(fetched, customProxies []model.Proxy) error {
	fetchedIndex := make(map[string]model.ProxyKind, len(fetched))
	for _, p := range fetched {
		fetchedIndex[p.Name] = p.Kind
	}

	for _, cp := range customProxies {
		if kind, exists := fetchedIndex[cp.Name]; exists {
			return &errtype.BuildError{
				Code:    errtype.CodeBuildCustomNameConflict,
				Phase:   "source",
				Message: fmt.Sprintf("自定义代理名 %q 与%s节点重名", cp.Name, describeFetchedKind(kind)),
			}
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
	default:
		return "远程拉取"
	}
}
