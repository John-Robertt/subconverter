package render

import (
	"os"
	"strings"
	"testing"
)

// T-RND-001: Clash renders with real base template merge
//
// TestClash_RealBaseTemplate validates that configs/base_clash.yaml
// can be parsed and merged without errors.
func TestClash_RealBaseTemplate(t *testing.T) {
	tpl, err := os.ReadFile("../../configs/base_clash.yaml")
	if err != nil {
		t.Skipf("base_clash.yaml not found: %v", err)
	}

	p := goldenPipeline()
	out, err := Clash(p, tpl)
	if err != nil {
		t.Fatalf("Clash() with real template: %v", err)
	}
	output := string(out)

	// Template settings preserved.
	if !strings.Contains(output, "mixed-port:") {
		t.Error("mixed-port should be preserved from template")
	}
	if !strings.Contains(output, "dns:") {
		t.Error("dns section should be preserved from template")
	}
	// Generated sections injected.
	if !strings.Contains(output, "proxies:") {
		t.Error("proxies should be injected")
	}
	if !strings.Contains(output, "proxy-groups:") {
		t.Error("proxy-groups should be injected")
	}
}

// T-RND-002: Surge renders with real base template merge
//
// TestSurge_RealBaseTemplate validates that configs/base_surge.conf
// can be parsed and merged without errors.
func TestSurge_RealBaseTemplate(t *testing.T) {
	tpl, err := os.ReadFile("../../configs/base_surge.conf")
	if err != nil {
		t.Skipf("base_surge.conf not found: %v", err)
	}

	p := goldenPipeline()
	out, err := Surge(p, "https://my-server.com/generate?format=surge", tpl)
	if err != nil {
		t.Fatalf("Surge() with real template: %v", err)
	}
	output := string(out)

	// Managed header present (from managedURL).
	if !strings.Contains(output, "#!MANAGED-CONFIG") {
		t.Error("managed header should be present")
	}
	// Template [General] preserved.
	if !strings.Contains(output, "loglevel = notify") {
		t.Error("[General] loglevel should be preserved from template")
	}
	if !strings.Contains(output, "skip-proxy") {
		t.Error("[General] skip-proxy should be preserved from template")
	}
	// Generated sections injected.
	if !strings.Contains(output, "[Proxy]") {
		t.Error("[Proxy] section should be present")
	}
	if !strings.Contains(output, "[Proxy Group]") {
		t.Error("[Proxy Group] section should be present")
	}
	if !strings.Contains(output, "[Rule]") {
		t.Error("[Rule] section should be present")
	}
}
