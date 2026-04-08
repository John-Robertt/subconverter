package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSurge_GoldenNoTemplate(t *testing.T) {
	p := goldenPipeline()
	got, err := Surge(p, "https://my-server.com", nil)
	if err != nil {
		t.Fatalf("Surge() error: %v", err)
	}

	goldenPath := filepath.Join("..", "..", "testdata", "render", "surge_golden.conf")

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, got, 0644); err != nil {
			t.Fatal(err)
		}
		t.Log("golden file updated")
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden file (run with UPDATE_GOLDEN=1 to create): %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("output differs from golden file.\n--- GOT ---\n%s\n--- WANT ---\n%s", got, want)
	}
}

func TestSurge_ManagedHeaderPresent(t *testing.T) {
	p := goldenPipeline()
	got, err := Surge(p, "https://my-server.com", nil)
	if err != nil {
		t.Fatalf("Surge() error: %v", err)
	}
	output := string(got)
	expected := "#!MANAGED-CONFIG https://my-server.com/generate?format=surge interval=86400 strict=false"
	if !strings.HasPrefix(output, expected) {
		t.Errorf("output should start with managed header.\nGot prefix: %q", output[:min(len(output), len(expected)+20)])
	}
}

func TestSurge_NoManagedHeaderWhenBaseURLEmpty(t *testing.T) {
	p := goldenPipeline()
	got, err := Surge(p, "", nil)
	if err != nil {
		t.Fatalf("Surge() error: %v", err)
	}
	output := string(got)
	if strings.Contains(output, "#!MANAGED-CONFIG") {
		t.Error("output should not contain managed header when baseURL is empty")
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
	if !(rulesetIdx < inlineIdx && inlineIdx < finalIdx) {
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
	got, err := Surge(p, "https://new-server.com", baseTemplate)
	if err != nil {
		t.Fatalf("Surge() error: %v", err)
	}
	output := string(got)

	// Should have exactly one managed header — the new one.
	count := strings.Count(output, "#!MANAGED-CONFIG")
	if count != 1 {
		t.Errorf("expected exactly 1 managed header, got %d", count)
	}
	if !strings.Contains(output, "https://new-server.com/generate?format=surge") {
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
	if !(quickIdx < hkIdx) {
		t.Error("route groups should be rendered before node groups")
	}
}
