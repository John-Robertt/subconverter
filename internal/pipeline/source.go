package pipeline

import (
	"context"
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
			URL:     fetch.SanitizeURL(rawURL),
			Message: "invalid base64 response",
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
		return nil, &errtype.BuildError{
			Phase:   "source",
			Message: fmt.Sprintf("subscription %s yielded 0 valid nodes", fetch.SanitizeURL(rawURL)),
		}
	}

	return proxies, nil
}

// decodeSubscriptionBody decodes a base64-encoded subscription response body.
func decodeSubscriptionBody(body []byte) (string, error) {
	s := strings.TrimSpace(string(body))
	if s == "" {
		return "", fmt.Errorf("empty response body")
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
// After initial dedup, a second pass resolves any collisions between generated
// suffixed names and original names (e.g. "HK-01②" already exists in subscription).
func deduplicateNames(proxies []model.Proxy) []model.Proxy {
	circled := []string{"②", "③", "④", "⑤", "⑥", "⑦", "⑧", "⑨", "⑩"}

	count := make(map[string]int, len(proxies))
	result := make([]model.Proxy, 0, len(proxies))

	for _, p := range proxies {
		count[p.Name]++
		n := count[p.Name]
		if n == 1 {
			result = append(result, p)
		} else {
			dup := p
			if n-1 <= len(circled) {
				dup.Name = p.Name + circled[n-2]
			} else {
				dup.Name = fmt.Sprintf("%s(%d)", p.Name, n)
			}
			result = append(result, dup)
		}
	}

	// Second pass: resolve collisions between generated names and original names.
	seen := make(map[string]struct{}, len(result))
	for i, p := range result {
		if _, exists := seen[p.Name]; exists {
			// Collision detected — append incrementing suffix until unique.
			for seq := 2; ; seq++ {
				var candidate string
				if seq-1 <= len(circled) {
					candidate = p.Name + circled[seq-2]
				} else {
					candidate = fmt.Sprintf("%s(%d)", p.Name, seq)
				}
				if _, taken := seen[candidate]; !taken {
					result[i].Name = candidate
					break
				}
			}
		}
		seen[result[i].Name] = struct{}{}
	}
	return result
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

// checkNameConflicts verifies that no custom proxy name collides
// with a subscription node name.
func checkNameConflicts(subProxies, customProxies []model.Proxy) error {
	subNames := make(map[string]struct{}, len(subProxies))
	for _, p := range subProxies {
		subNames[p.Name] = struct{}{}
	}

	for _, cp := range customProxies {
		if _, exists := subNames[cp.Name]; exists {
			return &errtype.BuildError{
				Phase:   "source",
				Message: fmt.Sprintf("custom proxy name %q conflicts with subscription node", cp.Name),
			}
		}
	}
	return nil
}
