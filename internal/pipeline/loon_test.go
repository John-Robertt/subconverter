package pipeline

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
	"gopkg.in/yaml.v3"
)

const loonSubscriptionFixture = `[General]
dns-server = system
[Proxy]
# synthetic nodes; the provider's groups and rules must not be imported
HK = anytls, hk.example.com, 443, "pw,=value", tls-name=edge.example.com, udp=true
HK = Shadowsocks, hk2.example.com, 8443, aes-128-gcm, "pw"
DROP = anytls, dropped.example.com, 443, "pw"
[Proxy Group]
PROVIDER = select, MISSING
[Rule]
FINAL,PROVIDER
[Host]
hk.example.com = alias.example.com
`

// T-SRC-LOON-001: explicit format overrides suffix; auto detection preserves legacy .conf behavior.
func TestSource_LoonDocumentDetection(t *testing.T) {
	cases := []struct {
		name, suffix, format string
		want                 model.NativeProxyFormat
	}{
		{"explicit loon conf", "conf", "loon", model.NativeProxyFormatLoon},
		{"explicit loon yaml", "yaml", "loon", model.NativeProxyFormatLoon},
		{"lcf suffix", "lcf", "", model.NativeProxyFormatLoon},
		{"legacy conf", "conf", "", model.NativeProxyFormatSurge},
		{"explicit surge", "lcf", "surge", model.NativeProxyFormatSurge},
		{"content", "txt", "auto", model.NativeProxyFormatLoon},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			url := "https://example.com/sub." + tt.suffix + "?token=secret"
			body := strings.Replace(loonSubscriptionFixture, "[General]", "[Remote Filter]\n[General]", 1)
			proxies, err := loadSubscription(context.Background(), &fakeFetcher{responses: map[string][]byte{url: []byte(body)}}, config.Source{URL: url, Format: tt.format})
			if err != nil {
				t.Fatalf("loadSubscription: %v", err)
			}
			if len(proxies) != 3 {
				t.Fatalf("proxies count = %d, want 3", len(proxies))
			}
			for _, proxy := range proxies {
				if proxy.Native == nil || proxy.Native.Format != tt.want || proxy.Kind != model.KindSubscription {
					t.Fatalf("proxy = %+v, want subscription/%s", proxy, tt.want)
				}
			}
			if !strings.Contains(proxies[0].Native.Payload, `"pw,=value"`) || !strings.Contains(proxies[0].Native.Payload, "tls-name=edge.example.com") {
				t.Fatalf("native declaration changed: %q", proxies[0].Native.Payload)
			}
		})
	}
}

// T-SRC-LOON-002: malformed and empty Loon documents fail without exposing credentials.
func TestSource_LoonDocumentFailures(t *testing.T) {
	for _, body := range []string{"[General]\ndns-server=system", "[Proxy]\n# empty\n", "[Proxy]\n= anytls, secret.example.com, 443, private-password\n"} {
		t.Run(body[:7], func(t *testing.T) {
			url := "https://example.com/sub.conf?token=private-token"
			_, err := loadSubscription(context.Background(), &fakeFetcher{responses: map[string][]byte{url: []byte(body)}}, config.Source{URL: url, Format: "loon"})
			var fetchErr *errtype.FetchError
			if !errors.As(err, &fetchErr) {
				t.Fatalf("error = %v, want FetchError", err)
			}
			if strings.Contains(err.Error(), "private-token") || strings.Contains(err.Error(), "private-password") {
				t.Fatalf("error leaked credentials: %v", err)
			}
		})
	}
}

// T-SRC-LOON-003: Loon nodes share IDs, name filters, region groups and chaining with other sources.
func TestExecute_LoonNodesUseConfiguredGroups(t *testing.T) {
	var cfg config.Config
	if err := yaml.Unmarshal([]byte(`sources:
  subscriptions:
    - {url: https://example.com/loon.conf, format: loon}
    - {url: https://example.com/nodes.txt}
  custom_proxies:
    - name: CHAIN
      url: socks5://user:pass@exit.example.com:1080
      relay_through: {type: group, name: HK, strategy: select}
filters: {exclude: DROP}
groups:
  HK: {match: HK, strategy: select}
routing:
  OUT: [HK, CHAIN, '@all', DIRECT]
rulesets: {}
rules: []
fallback: OUT
`), &cfg); err != nil {
		t.Fatal(err)
	}
	p, err := Execute(context.Background(), &cfg, &fakeFetcher{responses: map[string][]byte{
		"https://example.com/loon.conf": []byte(loonSubscriptionFixture),
		"https://example.com/nodes.txt": []byte("anytls://pw@hk.example.com:443#HK"),
	}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(p.Proxies) != 6 || len(p.AllProxies) != 3 || len(p.NodeGroups) != 2 || len(p.NodeGroups[0].Members) != 3 {
		t.Fatalf("pipeline counts: proxies=%d all=%d groups=%+v", len(p.Proxies), len(p.AllProxies), p.NodeGroups)
	}
	seen := map[model.ProxyID]bool{}
	for _, proxy := range p.Proxies {
		if proxy.ID == 0 || seen[proxy.ID] {
			t.Fatalf("duplicate or zero ID: %d", proxy.ID)
		}
		seen[proxy.ID] = true
	}
	for _, id := range p.AllProxies {
		for _, proxy := range p.Proxies {
			if proxy.ID == id && proxy.Kind == model.KindChained {
				t.Fatalf("@all contains chained ID %d", id)
			}
		}
	}
}
