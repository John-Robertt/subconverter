package render

import (
	"os"
	"strings"
	"testing"
)

// T-RND-001: Clash 渲染时合并真实的底版模板
//
// TestClash_RealBaseTemplate 验证 configs/base_clash.yaml
// 能被正常解析并合并。
func TestClash_RealBaseTemplate(t *testing.T) {
	tpl, err := os.ReadFile("../../configs/base_clash.yaml")
	if err != nil {
		t.Skipf("找不到 base_clash.yaml：%v", err)
	}

	p := goldenPipeline()
	out, err := Clash(p, tpl)
	if err != nil {
		t.Fatalf("使用真实模板调用 Clash() 出错：%v", err)
	}
	output := string(out)

	// 模板中的设置被保留。
	if !strings.Contains(output, "mixed-port:") {
		t.Error("模板中的 mixed-port 应保留")
	}
	if !strings.Contains(output, "dns:") {
		t.Error("模板中的 dns 段应保留")
	}
	// 生成的段落已注入。
	if !strings.Contains(output, "proxies:") {
		t.Error("proxies 应被注入")
	}
	if !strings.Contains(output, "proxy-groups:") {
		t.Error("proxy-groups 应被注入")
	}
}

// T-RND-002: Surge 渲染时合并真实的底版模板
//
// TestSurge_RealBaseTemplate 验证 configs/base_surge.conf
// 能被正常解析并合并。
func TestSurge_RealBaseTemplate(t *testing.T) {
	tpl, err := os.ReadFile("../../configs/base_surge.conf")
	if err != nil {
		t.Skipf("找不到 base_surge.conf：%v", err)
	}

	p := goldenPipeline()
	out, err := Surge(p, "https://my-server.com/generate?format=surge", tpl)
	if err != nil {
		t.Fatalf("使用真实模板调用 Surge() 出错：%v", err)
	}
	output := string(out)

	// managed 头部存在（来自 managedURL）。
	if !strings.Contains(output, "#!MANAGED-CONFIG") {
		t.Error("应有 managed 头部")
	}
	// 模板中的 [General] 被保留。
	if !strings.Contains(output, "loglevel = notify") {
		t.Error("模板 [General] 中的 loglevel 应保留")
	}
	if !strings.Contains(output, "skip-proxy") {
		t.Error("模板 [General] 中的 skip-proxy 应保留")
	}
	// 生成的段落已注入。
	if !strings.Contains(output, "[Proxy]") {
		t.Error("应有 [Proxy] 段")
	}
	if !strings.Contains(output, "[Proxy Group]") {
		t.Error("应有 [Proxy Group] 段")
	}
	if !strings.Contains(output, "[Rule]") {
		t.Error("应有 [Rule] 段")
	}
}
