package render

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

func TestSurge_GoldenNoTemplate(t *testing.T) {
	p := goldenPipeline()
	got, err := Surge(p, "https://my-server.com/generate?format=surge", nil)
	if err != nil {
		t.Fatalf("Surge() error: %v", err)
	}

	goldenPath := filepath.Join("..", "..", "testdata", "render", "surge_golden.conf")

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

func TestSurge_ManagedHeaderPresent(t *testing.T) {
	p := goldenPipeline()
	got, err := Surge(p, "https://my-server.com/generate?format=surge", nil)
	if err != nil {
		t.Fatalf("Surge() error: %v", err)
	}
	output := string(got)
	expected := "#!MANAGED-CONFIG https://my-server.com/generate?format=surge interval=86400 strict=false"
	if !strings.HasPrefix(output, expected) {
		t.Errorf("output should start with managed header.\nGot prefix: %q", output[:min(len(output), len(expected)+20)])
	}
}

func TestSurge_NoManagedHeaderWhenManagedURLEmpty(t *testing.T) {
	p := goldenPipeline()
	got, err := Surge(p, "", nil)
	if err != nil {
		t.Fatalf("Surge() error: %v", err)
	}
	output := string(got)
	if strings.Contains(output, "#!MANAGED-CONFIG") {
		t.Error("output should not contain managed header when managedURL is empty")
	}
}

func TestSurge_ChainedProxyHasUnderlyingProxy(t *testing.T) {
	p := goldenPipeline()
	got, err := Surge(p, "", nil)
	if err != nil {
		t.Fatalf("Surge() error: %v", err)
	}
	output := string(got)
	if !strings.Contains(output, "underlying-proxy=HK-01") {
		t.Error("chained proxy should contain underlying-proxy field")
	}
}

func TestSurge_SSProxyWithSimpleObfsPlugin(t *testing.T) {
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

	got, err := Surge(p, "", nil)
	if err != nil {
		t.Fatalf("Surge() error: %v", err)
	}

	output := string(got)
	if !strings.Contains(output, "HK-OBFS = ss, hk.example.com, 8388, encrypt-method=aes-256-gcm, password=secret, obfs=http, obfs-host=cdn.example.com") {
		t.Error("ss proxy should render Surge obfs parameters")
	}
	if !strings.Contains(output, "FINAL,DIRECT") {
		t.Error("fallback rule should still be present")
	}
}

func TestSurge_SSProxyWithUnsupportedPlugin(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{{
			Name:   "HK-V2RAY",
			Type:   "ss",
			Server: "hk.example.com",
			Port:   8388,
			Params: map[string]string{"cipher": "aes-256-gcm", "password": "secret"},
			Plugin: &model.Plugin{Name: "v2ray-plugin", Opts: map[string]string{"mode": "websocket"}},
			Kind:   model.KindSubscription,
		}},
		Fallback: "DIRECT",
	}

	_, err := Surge(p, "", nil)
	if err == nil {
		t.Fatal("expected error for unsupported ss plugin")
	}

	var renderErr *errtype.RenderError
	if !errors.As(err, &renderErr) {
		t.Fatalf("error type = %T, want *errtype.RenderError", err)
	}
	if !strings.Contains(err.Error(), `不支持的 ss plugin "v2ray-plugin"`) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSurge_RuleOrder(t *testing.T) {
	p := goldenPipeline()
	got, err := Surge(p, "", nil)
	if err != nil {
		t.Fatalf("Surge() error: %v", err)
	}
	output := string(got)

	rulesetIdx := strings.Index(output, "RULE-SET,https://example.com/rules/Ad.list,Quick")
	inlineIdx := strings.Index(output, "GEOIP,CN,Final")
	finalIdx := strings.Index(output, "FINAL,Final")

	if rulesetIdx < 0 || inlineIdx < 0 || finalIdx < 0 {
		t.Fatalf("missing expected rules in output:\n%s", output)
	}
	if rulesetIdx >= inlineIdx || inlineIdx >= finalIdx {
		t.Error("rule order should be: RULE-SET < inline < FINAL")
	}
}

func TestSurge_WithBaseTemplate(t *testing.T) {
	baseTemplate := []byte(`[General]
loglevel = notify
skip-proxy = 127.0.0.1

[Proxy]
OLD-PROXY = ss, old.example.com, 443, encrypt-method=aes-128-gcm, password=old

[Proxy Group]
OLD-GROUP = select, OLD-PROXY

[Rule]
FINAL,DIRECT
`)
	p := goldenPipeline()
	got, err := Surge(p, "", baseTemplate)
	if err != nil {
		t.Fatalf("Surge() error: %v", err)
	}
	output := string(got)

	// Template [General] should be preserved.
	if !strings.Contains(output, "loglevel = notify") {
		t.Error("[General] section should be preserved from template")
	}
	// [Proxy] should be replaced, not contain old content.
	if strings.Contains(output, "OLD-PROXY") {
		t.Error("[Proxy] section should be replaced, not contain old content")
	}
	// Generated proxies should be present.
	if !strings.Contains(output, "HK-01 = ss") {
		t.Error("generated proxies should be present")
	}
	// Generated rules should have FINAL, not old DIRECT.
	if !strings.Contains(output, "FINAL,Final") {
		t.Error("generated rules should be present")
	}
}

func TestSurge_BaseTemplateWithManagedHeader(t *testing.T) {
	baseTemplate := []byte(`#!MANAGED-CONFIG https://old-server.com/generate?format=surge interval=3600 strict=true

[General]
loglevel = notify

[Proxy]
OLD = ss, old.example.com, 443, encrypt-method=aes-128-gcm, password=old

[Proxy Group]
OLD-GROUP = select, OLD

[Rule]
FINAL,DIRECT
`)
	p := goldenPipeline()
	got, err := Surge(p, "https://new-server.com/generate?format=surge&token=test-token&filename=surge.conf", baseTemplate)
	if err != nil {
		t.Fatalf("Surge() error: %v", err)
	}
	output := string(got)

	// Should have exactly one managed header — the new one.
	count := strings.Count(output, "#!MANAGED-CONFIG")
	if count != 1 {
		t.Errorf("expected exactly 1 managed header, got %d", count)
	}
	if !strings.Contains(output, "https://new-server.com/generate?format=surge&token=test-token&filename=surge.conf") {
		t.Error("managed header should use new baseURL")
	}
	if strings.Contains(output, "old-server.com") {
		t.Error("old managed header should be stripped")
	}
}

func TestSurge_GroupOrderRouteBeforeNode(t *testing.T) {
	p := goldenPipeline()
	got, err := Surge(p, "", nil)
	if err != nil {
		t.Fatalf("Surge() error: %v", err)
	}
	output := string(got)

	quickIdx := strings.Index(output, "Quick = select")
	hkIdx := strings.Index(output, "🇭🇰 HK = select")
	if quickIdx < 0 || hkIdx < 0 {
		t.Fatalf("missing expected groups in output:\n%s", output)
	}
	if quickIdx >= hkIdx {
		t.Error("route groups should be rendered before node groups")
	}
}

// T-SURGE-SNELL-001: Snell proxy renders with the fixed key order.
func TestSurge_SnellProxy_BasicFields(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{{
			Name:   "HK-Snell",
			Type:   "snell",
			Server: "1.2.3.4",
			Port:   57891,
			Params: map[string]string{
				"psk":     "xxx",
				"version": "4",
				"reuse":   "true",
				"tfo":     "true",
			},
			Kind: model.KindSnell,
		}},
		Fallback: "DIRECT",
	}

	got, err := Surge(p, "", nil)
	if err != nil {
		t.Fatalf("Surge() error: %v", err)
	}
	output := string(got)

	// Expected: fixed key order (psk, version, reuse, tfo) regardless of map insertion order.
	want := "HK-Snell = snell, 1.2.3.4, 57891, psk=xxx, version=4, reuse=true, tfo=true"
	if !strings.Contains(output, want) {
		t.Errorf("Snell proxy not rendered correctly.\nwant substring:\n%s\ngot:\n%s", want, output)
	}
}

// T-SURGE-SNELL-002: ShadowTLS fields appear after base fields in declared order.
func TestSurge_SnellProxy_ShadowTLS(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{{
			Name:   "JP-Snell",
			Type:   "snell",
			Server: "9.10.11.12",
			Port:   443,
			Params: map[string]string{
				"psk":                 "zzz",
				"version":             "4",
				"shadow-tls-password": "sss",
				"shadow-tls-sni":      "www.microsoft.com",
				"shadow-tls-version":  "3",
			},
			Kind: model.KindSnell,
		}},
		Fallback: "DIRECT",
	}

	got, err := Surge(p, "", nil)
	if err != nil {
		t.Fatalf("Surge() error: %v", err)
	}
	output := string(got)

	want := "JP-Snell = snell, 9.10.11.12, 443, psk=zzz, version=4, shadow-tls-password=sss, shadow-tls-sni=www.microsoft.com, shadow-tls-version=3"
	if !strings.Contains(output, want) {
		t.Errorf("ShadowTLS not rendered correctly.\nwant substring:\n%s\ngot:\n%s", want, output)
	}
}

// T-SURGE-SNELL-004: Snell node + managed header + base template共存。
// Verifies that #!MANAGED-CONFIG appears exactly once (not duplicated by the
// template merge) and Snell proxy lines land in the [Proxy] section.
func TestSurge_SnellWithManagedHeader(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			{
				Name:   "HK-Snell",
				Type:   "snell",
				Server: "1.2.3.4",
				Port:   57891,
				Params: map[string]string{"psk": "x", "version": "4", "reuse": "true"},
				Kind:   model.KindSnell,
			},
		},
		RouteGroups: []model.ProxyGroup{
			{Name: "Final", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"HK-Snell", "DIRECT"}},
		},
		Fallback: "Final",
	}

	baseTemplate := []byte("[General]\nloglevel = notify\n\n[Proxy]\n# placeholder\n\n[Proxy Group]\n# placeholder\n\n[Rule]\n# placeholder\n")
	managedURL := "https://my-server.com/generate?format=surge&filename=surge.conf"

	got, err := Surge(p, managedURL, baseTemplate)
	if err != nil {
		t.Fatalf("Surge() error: %v", err)
	}
	out := string(got)

	// Managed header must appear exactly once.
	if c := strings.Count(out, "#!MANAGED-CONFIG "); c != 1 {
		t.Errorf("#!MANAGED-CONFIG count = %d, want 1:\n%s", c, out)
	}
	// Snell proxy line must land in output.
	if !strings.Contains(out, "HK-Snell = snell, 1.2.3.4, 57891, psk=x, version=4, reuse=true") {
		t.Errorf("Snell proxy line missing from Surge output:\n%s", out)
	}
	// Base template's [General] section must survive.
	if !strings.Contains(out, "loglevel = notify") {
		t.Errorf("base template [General] section should be preserved:\n%s", out)
	}
}

// T-SURGE-SNELL-003: Unknown Params keys are not emitted (fixed-list renderer).
func TestSurge_SnellProxy_UnknownKeyDropped(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{{
			Name:   "X",
			Type:   "snell",
			Server: "1.1.1.1",
			Port:   443,
			Params: map[string]string{
				"psk":         "abc",
				"future-knob": "42",
			},
			Kind: model.KindSnell,
		}},
		Fallback: "DIRECT",
	}

	got, err := Surge(p, "", nil)
	if err != nil {
		t.Fatalf("Surge() error: %v", err)
	}
	output := string(got)
	if strings.Contains(output, "future-knob") {
		t.Errorf("unknown key leaked into output:\n%s", output)
	}
	if !strings.Contains(output, "X = snell, 1.1.1.1, 443, psk=abc") {
		t.Errorf("base fields missing:\n%s", output)
	}
}
