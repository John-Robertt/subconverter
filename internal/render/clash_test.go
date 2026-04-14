package render

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
	"github.com/John-Robertt/subconverter/internal/pipeline"
	"gopkg.in/yaml.v3"
)

type clashProviderDoc struct {
	RuleProviders map[string]struct {
		Format string `yaml:"format"`
		Path   string `yaml:"path"`
	} `yaml:"rule-providers"`
}

func mustParseClashProviderDoc(t *testing.T, data []byte) clashProviderDoc {
	t.Helper()
	var doc clashProviderDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse clash yaml: %v", err)
	}
	return doc
}

func goldenPipeline() *model.Pipeline {
	return &model.Pipeline{
		Proxies: []model.Proxy{
			{
				Name:   "HK-01",
				Type:   "ss",
				Server: "hk.example.com",
				Port:   8388,
				Params: map[string]string{"cipher": "aes-256-gcm", "password": "secret"},
				Kind:   model.KindSubscription,
			},
			{
				Name:   "SG-01",
				Type:   "ss",
				Server: "sg.example.com",
				Port:   8388,
				Params: map[string]string{"cipher": "aes-256-gcm", "password": "secret"},
				Kind:   model.KindSubscription,
			},
			{
				Name:   "MY-PROXY",
				Type:   "socks5",
				Server: "10.0.0.1",
				Port:   1080,
				Params: map[string]string{"username": "user", "password": "pass"},
				Kind:   model.KindCustom,
			},
			{
				Name:   "HK-01→MY-PROXY",
				Type:   "socks5",
				Server: "10.0.0.1",
				Port:   1080,
				Params: map[string]string{"username": "user", "password": "pass"},
				Kind:   model.KindChained,
				Dialer: "HK-01",
			},
		},
		NodeGroups: []model.ProxyGroup{
			{Name: "🇭🇰 HK", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-01"}},
			{Name: "🔗 MY-PROXY", Scope: model.ScopeNode, Strategy: "url-test", Members: []string{"HK-01→MY-PROXY"}},
		},
		RouteGroups: []model.ProxyGroup{
			{Name: "Quick", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"🇭🇰 HK", "🔗 MY-PROXY", "DIRECT"}},
			{Name: "Final", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"Quick", "DIRECT"}},
		},
		Rulesets: []model.Ruleset{
			{Policy: "Quick", URLs: []string{
				"https://example.com/rules/Ad.list",
				"https://example.com/rules/Ad.list", // duplicate URL → provider name dedup
			}},
		},
		Rules: []model.Rule{
			{Raw: "GEOIP,CN,Final", Policy: "Final"},
		},
		Fallback:   "Final",
		AllProxies: []string{"HK-01", "SG-01", "MY-PROXY"},
	}
}

func TestClash_GoldenNoTemplate(t *testing.T) {
	p := goldenPipeline()
	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() error: %v", err)
	}

	goldenPath := filepath.Join("..", "..", "testdata", "render", "clash_golden.yaml")

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, got, 0600); err != nil {
			t.Fatal(err)
		}
		t.Log("golden file updated")
		return
	}

	want, err := os.ReadFile(filepath.Clean(goldenPath))
	if err != nil {
		t.Fatalf("reading golden file (run with UPDATE_GOLDEN=1 to create): %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("output differs from golden file.\n--- GOT ---\n%s\n--- WANT ---\n%s", got, want)
	}
}

func TestClash_ChainedProxyHasDialerProxy(t *testing.T) {
	p := goldenPipeline()
	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() error: %v", err)
	}
	output := string(got)
	if !strings.Contains(output, "dialer-proxy: HK-01") {
		t.Error("chained proxy should contain dialer-proxy field")
	}
}

func TestClash_SSProxyWithSimpleObfsPlugin(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{{
			Name:   "HK-OBFS",
			Type:   "ss",
			Server: "hk.example.com",
			Port:   8388,
			Params: map[string]string{
				"cipher":   "aes-256-gcm",
				"password": "secret",
			},
			Plugin: &model.Plugin{Name: "simple-obfs", Opts: map[string]string{"obfs": "http", "obfs-host": "cdn.example.com"}},
			Kind:   model.KindSubscription,
		}},
		Fallback: "DIRECT",
	}

	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() error: %v", err)
	}

	output := string(got)
	if !strings.Contains(output, "plugin: obfs") {
		t.Error("ss proxy should normalize simple-obfs to plugin: obfs")
	}
	if !strings.Contains(output, "plugin-opts:") {
		t.Error("ss proxy should render plugin-opts section")
	}
	if !strings.Contains(output, "mode: http") {
		t.Error("ss proxy should map obfs to plugin mode")
	}
	if !strings.Contains(output, "host: cdn.example.com") {
		t.Error("ss proxy should map obfs-host to plugin host")
	}
	if !strings.Contains(output, "MATCH,DIRECT") {
		t.Error("fallback rule should still be present")
	}
}

func TestClash_SSProxyWithGenericPluginOptions(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{{
			Name:   "HK-V2RAY",
			Type:   "ss",
			Server: "hk.example.com",
			Port:   8388,
			Params: map[string]string{"cipher": "aes-256-gcm", "password": "secret"},
			Plugin: &model.Plugin{Name: "v2ray-plugin", Opts: map[string]string{"host": "cdn.example.com", "mode": "websocket"}},
			Kind:   model.KindSubscription,
		}},
		Fallback: "DIRECT",
	}

	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() error: %v", err)
	}

	output := string(got)
	if !strings.Contains(output, "plugin: v2ray-plugin") {
		t.Error("ss proxy should preserve generic plugin name for Clash")
	}
	if !strings.Contains(output, "host: cdn.example.com") || !strings.Contains(output, "mode: websocket") {
		t.Error("ss proxy should preserve generic plugin options for Clash")
	}
}

func TestClash_RuleOrder(t *testing.T) {
	p := goldenPipeline()
	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() error: %v", err)
	}
	output := string(got)

	rulesetIdx := strings.Index(output, "RULE-SET,Ad,Quick")
	inlineIdx := strings.Index(output, "GEOIP,CN,Final")
	matchIdx := strings.Index(output, "MATCH,Final")

	if rulesetIdx < 0 || inlineIdx < 0 || matchIdx < 0 {
		t.Fatalf("missing expected rules in output:\n%s", output)
	}
	if rulesetIdx >= inlineIdx || inlineIdx >= matchIdx {
		t.Error("rule order should be: RULE-SET < inline < MATCH")
	}
}

func TestClash_ProviderNameDedup(t *testing.T) {
	p := goldenPipeline()
	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() error: %v", err)
	}
	output := string(got)

	if !strings.Contains(output, "Ad:") {
		t.Error("expected provider name Ad")
	}
	if !strings.Contains(output, "Ad-2:") {
		t.Error("expected deduplicated provider name Ad-2")
	}
}

func TestClash_RuleProvidersUseTextFormat(t *testing.T) {
	p := goldenPipeline()
	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() error: %v", err)
	}
	doc := mustParseClashProviderDoc(t, got)

	provider, ok := doc.RuleProviders["Ad"]
	if !ok {
		t.Fatal("expected provider Ad")
	}
	if provider.Format != "text" {
		t.Errorf("provider format = %q, want %q", provider.Format, "text")
	}
	if provider.Path != "./rule-providers/Ad.txt" {
		t.Errorf("provider path = %q, want %q", provider.Path, "./rule-providers/Ad.txt")
	}
}

func TestClash_URLTestHasTolerance(t *testing.T) {
	p := goldenPipeline()
	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() error: %v", err)
	}
	if !strings.Contains(string(got), "tolerance: 100") {
		t.Error("url-test group should contain tolerance: 100")
	}
}

func TestClash_GroupOrderRouteBeforeNode(t *testing.T) {
	p := goldenPipeline()
	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() error: %v", err)
	}
	output := string(got)

	quickIdx := strings.Index(output, "- name: Quick")
	hkIdx := strings.Index(output, "- name: \"\\U0001F1ED\\U0001F1F0 HK\"")
	if quickIdx < 0 || hkIdx < 0 {
		t.Fatalf("missing expected groups in output:\n%s", output)
	}
	if quickIdx >= hkIdx {
		t.Error("route groups should be rendered before node groups")
	}
}

func TestClash_WithBaseTemplate(t *testing.T) {
	baseTemplate := []byte(`mixed-port: 7890
allow-lan: false
mode: rule
log-level: info
`)
	p := goldenPipeline()
	got, err := Clash(p, baseTemplate)
	if err != nil {
		t.Fatalf("Clash() error: %v", err)
	}
	output := string(got)

	// Template settings should be preserved.
	if !strings.Contains(output, "mixed-port:") {
		t.Error("base template settings should be preserved")
	}
	if !strings.Contains(output, "allow-lan:") {
		t.Error("base template settings should be preserved")
	}
	// Generated sections should be present.
	if !strings.Contains(output, "proxies:") {
		t.Error("proxies section should be present")
	}
	if !strings.Contains(output, "proxy-groups:") {
		t.Error("proxy-groups section should be present")
	}
	if !strings.Contains(output, "rule-providers:") {
		t.Error("rule-providers section should be present")
	}
	if !strings.Contains(output, "rules:") {
		t.Error("rules section should be present")
	}
}

func TestClash_EmptyBaseTemplate(t *testing.T) {
	p := goldenPipeline()
	_, err := Clash(p, []byte{})
	if err == nil {
		t.Fatal("expected error for empty base template")
	}
	var re *errtype.RenderError
	if !errors.As(err, &re) {
		t.Errorf("expected *errtype.RenderError, got %T", err)
	}
}

func TestClash_NullYAMLBaseTemplate(t *testing.T) {
	p := goldenPipeline()
	_, err := Clash(p, []byte("---\nnull\n"))
	if err == nil {
		t.Fatal("expected error for null YAML base template")
	}
	var re *errtype.RenderError
	if !errors.As(err, &re) {
		t.Errorf("expected *errtype.RenderError, got %T", err)
	}
}

func TestClash_ProviderNameCollisionWithNatural(t *testing.T) {
	// URL A and B both extract to "Ad", generating "Ad" and "Ad-2".
	// URL C's natural name is "Ad-2", which would collide.
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			{Name: "N1", Type: "ss", Server: "s", Port: 1, Params: map[string]string{"cipher": "aes-256-gcm", "password": "p"}, Kind: model.KindSubscription},
		},
		NodeGroups:  []model.ProxyGroup{{Name: "G", Scope: model.ScopeNode, Strategy: "select", Members: []string{"N1"}}},
		RouteGroups: []model.ProxyGroup{{Name: "R", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"G"}}},
		Rulesets: []model.Ruleset{
			{Policy: "R", URLs: []string{
				"https://example.com/rules/Ad.list",   // "Ad"
				"https://example.com/other/Ad.list",   // "Ad" → should get "Ad-2" but...
				"https://example.com/rules/Ad-2.list", // natural "Ad-2" → must not collide
			}},
		},
		Fallback:   "R",
		AllProxies: []string{"N1"},
	}

	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() error: %v", err)
	}
	output := string(got)

	// All three provider names must be distinct.
	names := []string{"Ad:", "Ad-2:", "Ad-3:"}
	for _, n := range names {
		if !strings.Contains(output, n) {
			t.Errorf("expected provider name %s in output", n)
		}
	}
}

// vlessPipeline builds a minimal Pipeline with one VLESS proxy matching the
// given overrides. Unset fields in overrides default to the Reality sample.
func vlessPipeline(name string, params map[string]string) *model.Pipeline {
	// baseline reality params
	base := map[string]string{
		"uuid":     "b33a72bf-75ab-4be2-b182-223a727b37a5",
		"network":  "tcp",
		"security": "reality",
	}
	for k, v := range params {
		if v == "" {
			delete(base, k)
		} else {
			base[k] = v
		}
	}
	return &model.Pipeline{
		Proxies: []model.Proxy{{
			Name:   name,
			Type:   "vless",
			Server: "11.11.11.11",
			Port:   443,
			Params: base,
			Kind:   model.KindVLess,
		}},
		Fallback: "DIRECT",
	}
}

// T-CLASH-VLESS-001: Reality + TCP + Vision emits full reality-opts block.
func TestClash_VlessTCPReality(t *testing.T) {
	p := vlessPipeline("VL-R", map[string]string{
		"flow":               "xtls-rprx-vision",
		"servername":         "www.cloudflare.com",
		"client-fingerprint": "chrome",
		"reality-public-key": "KEY",
		"reality-short-id":   "SID",
	})

	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() error: %v", err)
	}

	output := string(got)
	wantSubstrings := []string{
		"type: vless",
		"uuid: b33a72bf-75ab-4be2-b182-223a727b37a5",
		"network: tcp",
		"udp: true",
		"flow: xtls-rprx-vision",
		"tls: true",
		"servername: www.cloudflare.com",
		"client-fingerprint: chrome",
		"reality-opts:",
		"public-key: KEY",
		"short-id: SID",
	}
	for _, w := range wantSubstrings {
		if !strings.Contains(output, w) {
			t.Errorf("missing %q in:\n%s", w, output)
		}
	}
}

// T-CLASH-VLESS-002: security=tls → tls:true, no reality-opts.
func TestClash_VlessTCPTLS(t *testing.T) {
	p := vlessPipeline("VL-TLS", map[string]string{
		"security":           "tls",
		"servername":         "sg.example.com",
		"client-fingerprint": "chrome",
		// ensure no residual reality keys
		"reality-public-key": "",
		"reality-short-id":   "",
	})

	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() error: %v", err)
	}

	output := string(got)
	if !strings.Contains(output, "tls: true") {
		t.Error("tls: true should be present for security=tls")
	}
	if !strings.Contains(output, "servername: sg.example.com") {
		t.Error("servername should be present")
	}
	if strings.Contains(output, "reality-opts") {
		t.Error("reality-opts must NOT be emitted for security=tls")
	}
}

// T-CLASH-VLESS-003: security=none → no tls, no servername, no reality-opts.
func TestClash_VlessTCPNone(t *testing.T) {
	p := vlessPipeline("VL-Plain", map[string]string{
		"security":           "none",
		"servername":         "",
		"client-fingerprint": "",
		"reality-public-key": "",
		"reality-short-id":   "",
	})

	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() error: %v", err)
	}

	output := string(got)
	if strings.Contains(output, "tls: true") {
		t.Error("security=none should NOT emit tls: true")
	}
	if strings.Contains(output, "servername:") {
		t.Error("security=none should NOT emit servername")
	}
	if strings.Contains(output, "reality-opts") {
		t.Error("security=none should NOT emit reality-opts")
	}
	if strings.Contains(output, "encryption:") {
		t.Error("empty encryption should NOT be emitted")
	}
	// Still must emit core fields.
	for _, w := range []string{"type: vless", "network: tcp", "udp: true"} {
		if !strings.Contains(output, w) {
			t.Errorf("missing %q in:\n%s", w, output)
		}
	}
}

// T-CLASH-VLESS-004: Non-empty encryption is emitted verbatim.
func TestClash_VlessEncryptionPassthrough(t *testing.T) {
	p := vlessPipeline("VL-ENC", map[string]string{
		"security":           "tls",
		"servername":         "enc.example.com",
		"encryption":         "mlkem768x25519plus.native",
		"reality-public-key": "",
		"reality-short-id":   "",
	})

	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() error: %v", err)
	}

	output := string(got)
	if !strings.Contains(output, "encryption: mlkem768x25519plus.native") {
		t.Errorf("non-empty encryption should be rendered, output:\n%s", output)
	}
}

// T-CLASH-VLESS-005: Comma-separated alpn becomes YAML list.
func TestClash_VlessAlpnListEmission(t *testing.T) {
	p := vlessPipeline("VL-ALPN", map[string]string{
		"security":           "tls",
		"servername":         "a.b",
		"alpn":               "h2,http/1.1",
		"reality-public-key": "",
		"reality-short-id":   "",
	})

	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() error: %v", err)
	}

	// yaml lists render each element on its own line starting with "- ".
	// Check for both entries as list items.
	output := string(got)
	if !strings.Contains(output, "alpn:") {
		t.Fatalf("missing alpn key in output:\n%s", output)
	}
	if !strings.Contains(output, "- h2") {
		t.Errorf("alpn should contain h2 as list item:\n%s", output)
	}
	if !strings.Contains(output, "- http/1.1") {
		t.Errorf("alpn should contain http/1.1 as list item:\n%s", output)
	}
}

// T-CLASH-VLESS-006: Unknown URI type is normalized to tcp before render.
func TestClash_VlessUnknownTypeFallbackRendersAsTCP(t *testing.T) {
	px, err := pipeline.ParseVLessURI("vless://11111111-2222-3333-4444-555555555555@hk.example.com:443?security=tls&sni=hk.example.com&type=quic&encryption=mlkem768x25519plus.native#VL-TCP-FALLBACK")
	if err != nil {
		t.Fatalf("ParseVLessURI: %v", err)
	}

	got, err := Clash(&model.Pipeline{
		Proxies:  []model.Proxy{px},
		Fallback: "DIRECT",
	}, nil)
	if err != nil {
		t.Fatalf("Clash() error: %v", err)
	}

	output := string(got)
	if !strings.Contains(output, "network: tcp") {
		t.Errorf("unknown URI type should render as network: tcp, output:\n%s", output)
	}
	for _, block := range []string{"ws-opts:", "http-opts:", "h2-opts:", "grpc-opts:", "xhttp-opts:"} {
		if strings.Contains(output, block) {
			t.Errorf("tcp fallback should not emit transport opts block %q, output:\n%s", block, output)
		}
	}
	if !strings.Contains(output, "encryption: mlkem768x25519plus.native") {
		t.Errorf("tcp fallback case should still render encryption, output:\n%s", output)
	}
}

// T-CLASH-VLESS-007: udp: true is always emitted.
func TestClash_VlessUDPDefaultTrue(t *testing.T) {
	p := vlessPipeline("VL-UDP", map[string]string{
		"security":           "none",
		"servername":         "",
		"client-fingerprint": "",
		"reality-public-key": "",
		"reality-short-id":   "",
	})

	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() error: %v", err)
	}

	if !strings.Contains(string(got), "udp: true") {
		t.Errorf("udp: true should be present for every vless proxy, output:\n%s", string(got))
	}
}

// T-CLASH-VLESS-008: Chained proxy with VLESS upstream emits dialer-proxy.
func TestClash_VlessChainAsDialerProxy(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			{
				Name:   "VL-HK",
				Type:   "vless",
				Server: "hk.example.com",
				Port:   443,
				Params: map[string]string{
					"uuid":     "11111111-2222-3333-4444-555555555555",
					"network":  "tcp",
					"security": "tls",
				},
				Kind: model.KindVLess,
			},
			{
				Name:   "VL-HK→CHAIN",
				Type:   "socks5",
				Server: "127.0.0.1",
				Port:   1080,
				Kind:   model.KindChained,
				Dialer: "VL-HK",
			},
		},
		Fallback: "DIRECT",
	}

	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() error: %v", err)
	}

	output := string(got)
	if !strings.Contains(output, "dialer-proxy: VL-HK") {
		t.Errorf("chain proxy should emit dialer-proxy: VL-HK, output:\n%s", output)
	}
	if !strings.Contains(output, "name: VL-HK") {
		t.Error("upstream VL-HK must appear in proxies block")
	}
}

// vlessTLSOverride returns the params overrides needed to make vlessPipeline
// produce a TLS proxy (no reality-opts) with a specific network transport.
// Call sites spread additional transport-specific keys into the returned map.
func vlessTLSOverride(network string) map[string]string {
	return map[string]string{
		"network":            network,
		"security":           "tls",
		"servername":         "example.com",
		"reality-public-key": "",
		"reality-short-id":   "",
	}
}

// T-CLASH-VLESS-WS: ws-opts emits path and headers.Host when set.
func TestClash_VlessWSOpts(t *testing.T) {
	overrides := vlessTLSOverride("ws")
	overrides["ws-path"] = "/ws-route"
	overrides["ws-host"] = "ws.example.com"
	p := vlessPipeline("VL-WS", overrides)

	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash: %v", err)
	}
	out := string(got)
	for _, w := range []string{
		"network: ws",
		"ws-opts:",
		"path: /ws-route",
		"headers:",
		"Host: ws.example.com",
	} {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in:\n%s", w, out)
		}
	}
}

// T-CLASH-VLESS-HTTP: http-opts emits path and headers.Host as YAML lists.
func TestClash_VlessHTTPOpts(t *testing.T) {
	overrides := vlessTLSOverride("http")
	overrides["http-path"] = "/api/v1"
	overrides["http-host"] = "api.example.com"
	p := vlessPipeline("VL-HTTP", overrides)

	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash: %v", err)
	}
	out := string(got)
	for _, w := range []string{
		"network: http",
		"http-opts:",
		"path:",
		"- /api/v1",
		"Host:",
		"- api.example.com",
	} {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in:\n%s", w, out)
		}
	}
}

// T-CLASH-VLESS-H2: h2-opts emits host (list) and path (scalar).
func TestClash_VlessH2Opts(t *testing.T) {
	overrides := vlessTLSOverride("h2")
	overrides["h2-path"] = "/h2"
	overrides["h2-host"] = "h2.example.com"
	p := vlessPipeline("VL-H2", overrides)

	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash: %v", err)
	}
	out := string(got)
	for _, w := range []string{
		"network: h2",
		"h2-opts:",
		"host:",
		"- h2.example.com",
		"path: /h2",
	} {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in:\n%s", w, out)
		}
	}
}

// T-CLASH-VLESS-GRPC: grpc-opts emits grpc-service-name.
func TestClash_VlessGrpcOpts(t *testing.T) {
	overrides := vlessTLSOverride("grpc")
	overrides["grpc-service-name"] = "GunService"
	p := vlessPipeline("VL-GRPC", overrides)

	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash: %v", err)
	}
	out := string(got)
	for _, w := range []string{
		"network: grpc",
		"grpc-opts:",
		"grpc-service-name: GunService",
	} {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in:\n%s", w, out)
		}
	}
}

// T-CLASH-VLESS-XHTTP: xhttp-opts emits mode, path and host.
func TestClash_VlessXhttpOpts(t *testing.T) {
	overrides := vlessTLSOverride("xhttp")
	overrides["xhttp-mode"] = "packet-up"
	overrides["xhttp-path"] = "/xh"
	overrides["xhttp-host"] = "xh.example.com"
	p := vlessPipeline("VL-XH", overrides)

	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash: %v", err)
	}
	out := string(got)
	for _, w := range []string{
		"network: xhttp",
		"xhttp-opts:",
		"mode: packet-up",
		"path: /xh",
		"host: xh.example.com",
	} {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in:\n%s", w, out)
		}
	}
}

// TestClash_VlessAllKnownNetworksProduceConsistentOutput is the cross-file
// sync guard for the explicitly handled non-tcp VLESS transport set.
//
// The parser's known-transport table (pipeline.vlessAllowedNetworks) and the
// renderer dispatch (render.emitClashVLessTransportOpts) are maintained
// independently. If someone adds a new known transport without updating the
// renderer (or this test), that network would leak into Clash YAML as a
// `network:` value with no matching `*-opts:` block — a silent drift in the
// repo's declared support set.
//
// This table enumerates every known non-fallback transport the renderer
// handles specially. tcp expects no transport opts block; every other entry
// must emit `<network>-opts`.
//
// **Extension checklist** when adding a new VLESS network:
//  1. Add to `vlessAllowedNetworks` in internal/pipeline/vlessuri.go
//  2. Add a dispatch case to `emitClashVLessTransportOpts` in clash.go
//  3. Add the network here with its expected opts prefix
//
// If any of these three steps is skipped, this test fails with a message
// pointing back at the same three sync sites.
func TestClash_VlessAllKnownNetworksProduceConsistentOutput(t *testing.T) {
	cases := []struct {
		network       string
		wantOptsBlock string // empty means no *-opts block expected
		seedKey       string // a transport-specific Params key to trigger opts emission
		seedValue     string
	}{
		{network: "tcp", wantOptsBlock: ""},
		{network: "ws", wantOptsBlock: "ws-opts:", seedKey: "ws-path", seedValue: "/x"},
		{network: "http", wantOptsBlock: "http-opts:", seedKey: "http-path", seedValue: "/x"},
		{network: "h2", wantOptsBlock: "h2-opts:", seedKey: "h2-path", seedValue: "/x"},
		{network: "grpc", wantOptsBlock: "grpc-opts:", seedKey: "grpc-service-name", seedValue: "svc"},
		{network: "xhttp", wantOptsBlock: "xhttp-opts:", seedKey: "xhttp-path", seedValue: "/x"},
	}

	allOptsBlocks := []string{"ws-opts:", "http-opts:", "h2-opts:", "grpc-opts:", "xhttp-opts:"}

	// hasExactOptsLine reports whether any trimmed line in the YAML equals
	// the given opts-block marker. Needed because `http-opts:` is a
	// substring of `xhttp-opts:` — naive strings.Contains would report
	// both matching for xhttp output.
	hasExactOptsLine := func(out, block string) bool {
		for _, line := range strings.Split(out, "\n") {
			if strings.TrimSpace(line) == block {
				return true
			}
		}
		return false
	}

	for _, tc := range cases {
		t.Run(tc.network, func(t *testing.T) {
			overrides := vlessTLSOverride(tc.network)
			if tc.seedKey != "" {
				overrides[tc.seedKey] = tc.seedValue
			}
			p := vlessPipeline("VL-"+tc.network, overrides)

			got, err := Clash(p, nil)
			if err != nil {
				t.Fatalf("Clash: %v", err)
			}
			out := string(got)

			if !strings.Contains(out, "network: "+tc.network) {
				t.Errorf("expected network: %s in output:\n%s", tc.network, out)
			}

			if tc.wantOptsBlock == "" {
				// tcp: ensure NO transport opts block leaked.
				for _, block := range allOptsBlocks {
					if hasExactOptsLine(out, block) {
						t.Errorf("network=%s must not emit %q (renderer sync drift?); output:\n%s", tc.network, block, out)
					}
				}
				return
			}

			if !hasExactOptsLine(out, tc.wantOptsBlock) {
				t.Errorf("network=%s is a known transport but renderer did NOT emit %q — update emitClashVLessTransportOpts in clash.go AND the case list in this test; output:\n%s",
					tc.network, tc.wantOptsBlock, out)
			}
			// Ensure no OTHER opts block leaked (e.g. ws-opts for network=http).
			for _, block := range allOptsBlocks {
				if block == tc.wantOptsBlock {
					continue
				}
				if hasExactOptsLine(out, block) {
					t.Errorf("network=%s leaked foreign %q into output:\n%s", tc.network, block, out)
				}
			}
		})
	}
}

// T-CLASH-VLESS-EMPTY-OPTS: network=ws with no path/host does NOT emit
// an empty ws-opts block (Clash treats missing as default-empty).
func TestClash_VlessNetworkWithNoSubfieldsOmitsOptsBlock(t *testing.T) {
	p := vlessPipeline("VL-WS-BARE", vlessTLSOverride("ws"))

	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash: %v", err)
	}
	out := string(got)
	if !strings.Contains(out, "network: ws") {
		t.Errorf("must emit network: ws, got:\n%s", out)
	}
	if strings.Contains(out, "ws-opts:") {
		t.Errorf("empty ws-opts block should be omitted, got:\n%s", out)
	}
}
