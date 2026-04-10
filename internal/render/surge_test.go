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
