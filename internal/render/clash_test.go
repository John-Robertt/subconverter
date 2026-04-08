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
