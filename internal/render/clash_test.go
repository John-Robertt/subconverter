package render

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
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
		t.Fatalf("解析 Clash YAML 失败：%v", err)
	}
	return doc
}

func goldenPipeline() *model.TargetPipeline {
	return &model.TargetPipeline{
		Proxies: []model.TargetProxy{
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
		NodeGroups: []model.TargetGroup{
			{Name: "🇭🇰 HK", Strategy: "select", Members: []string{"HK-01"}},
			{Name: "🔗 MY-PROXY", Strategy: "url-test", Members: []string{"HK-01→MY-PROXY"}},
		},
		RouteGroups: []model.TargetGroup{
			{Name: "Quick", Strategy: "select", Members: []string{"🇭🇰 HK", "🔗 MY-PROXY", "DIRECT"}},
			{Name: "Final", Strategy: "select", Members: []string{"Quick", "DIRECT"}},
		},
		Rulesets: []model.Ruleset{
			{Policy: "Quick", URLs: []string{
				"https://example.com/rules/Ad.list",
				"https://example.com/rules/Ad.list", // 重复 URL → provider 名称去重
			}},
		},
		Rules: []model.Rule{
			{Raw: "GEOIP,CN,Final", Policy: "Final"},
		},
		Fallback: "Final",
	}
}

func TestClash_GoldenNoTemplate(t *testing.T) {
	p := goldenPipeline()
	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() 出错：%v", err)
	}

	goldenPath := filepath.Join("..", "..", "testdata", "render", "clash_golden.yaml")

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, got, 0600); err != nil {
			t.Fatal(err)
		}
		t.Log("golden 文件已更新")
		return
	}

	want, err := os.ReadFile(filepath.Clean(goldenPath))
	if err != nil {
		t.Fatalf("读取 golden 文件失败（用 UPDATE_GOLDEN=1 运行可生成）：%v", err)
	}
	if string(got) != string(want) {
		t.Errorf("输出与 golden 文件不一致。\n--- GOT ---\n%s\n--- WANT ---\n%s", got, want)
	}
}

func TestClash_ChainedProxyHasDialerProxy(t *testing.T) {
	p := goldenPipeline()
	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() 出错：%v", err)
	}
	output := string(got)
	if !strings.Contains(output, "dialer-proxy: HK-01") {
		t.Error("链式节点应包含 dialer-proxy 字段")
	}
}

func TestClash_SSProxyWithSimpleObfsPlugin(t *testing.T) {
	p := &model.TargetPipeline{
		Proxies: []model.TargetProxy{{
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
		t.Fatalf("Clash() 出错：%v", err)
	}

	output := string(got)
	if !strings.Contains(output, "plugin: obfs") {
		t.Error("ss 节点应把 simple-obfs 归一化为 plugin: obfs")
	}
	if !strings.Contains(output, "plugin-opts:") {
		t.Error("ss 节点应渲染 plugin-opts 段")
	}
	if !strings.Contains(output, "mode: http") {
		t.Error("ss 节点应把 obfs 映射为插件的 mode")
	}
	if !strings.Contains(output, "host: cdn.example.com") {
		t.Error("ss 节点应把 obfs-host 映射为插件的 host")
	}
	if !strings.Contains(output, "MATCH,DIRECT") {
		t.Error("fallback 规则应仍然存在")
	}
}

func TestClash_SSProxyWithGenericPluginOptions(t *testing.T) {
	p := &model.TargetPipeline{
		Proxies: []model.TargetProxy{{
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
		t.Fatalf("Clash() 出错：%v", err)
	}

	output := string(got)
	if !strings.Contains(output, "plugin: v2ray-plugin") {
		t.Error("ss 节点在 Clash 中应保留通用插件名")
	}
	if !strings.Contains(output, "host: cdn.example.com") || !strings.Contains(output, "mode: websocket") {
		t.Error("ss 节点在 Clash 中应保留通用插件选项")
	}
}

func TestClash_SSProxyUDPAndTFO(t *testing.T) {
	p := &model.TargetPipeline{
		Proxies: []model.TargetProxy{{
			Name:   "HK-SS",
			Type:   "ss",
			Server: "hk.example.com",
			Port:   8388,
			Params: map[string]string{
				"cipher":    "2022-blake3-aes-128-gcm",
				"password":  "secret",
				"udp-relay": "true",
				"tfo":       "true",
			},
			Kind: model.KindSubscription,
		}},
		Fallback: "DIRECT",
	}

	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() 出错：%v", err)
	}

	output := string(got)
	if !strings.Contains(output, "udp: true") {
		t.Errorf("udp-relay=true 时 ss 节点应渲染 udp: true：\n%s", output)
	}
	if !strings.Contains(output, "tfo: true") {
		t.Errorf("tfo=true 时 ss 节点应渲染 tfo: true：\n%s", output)
	}
}

func TestClash_AnyTLSProxy(t *testing.T) {
	p := &model.TargetPipeline{
		Proxies: []model.TargetProxy{{
			Name:   "HK-AnyTLS",
			Type:   "anytls",
			Server: "demo.com",
			Port:   3383,
			Params: map[string]string{
				"password":                    "secret",
				"sni":                         "cache-proxy.example.com",
				"skip-cert-verify":            "true",
				"alpn":                        "h2,http/1.1",
				"client-fingerprint":          "chrome",
				"idle-session-check-interval": "30",
				"idle-session-timeout":        "30",
				"min-idle-session":            "0",
				"udp-relay":                   "false",
			},
			Kind: model.KindSubscription,
		}},
		Fallback: "DIRECT",
	}

	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() 出错：%v", err)
	}

	output := string(got)
	for _, want := range []string{
		"type: anytls",
		"password: secret",
		"udp: true",
		"sni: cache-proxy.example.com",
		"skip-cert-verify: true",
		"client-fingerprint: chrome",
		"- h2",
		"- http/1.1",
		"idle-session-check-interval: \"30\"",
		"idle-session-timeout: \"30\"",
		"min-idle-session: \"0\"",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("Clash AnyTLS 输出缺少 %q：\n%s", want, output)
		}
	}
}

func TestClash_RuleOrder(t *testing.T) {
	p := goldenPipeline()
	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() 出错：%v", err)
	}
	output := string(got)

	rulesetIdx := strings.Index(output, "RULE-SET,Ad,Quick")
	inlineIdx := strings.Index(output, "GEOIP,CN,Final")
	matchIdx := strings.Index(output, "MATCH,Final")

	if rulesetIdx < 0 || inlineIdx < 0 || matchIdx < 0 {
		t.Fatalf("输出中缺少预期的规则：\n%s", output)
	}
	if rulesetIdx >= inlineIdx || inlineIdx >= matchIdx {
		t.Error("规则顺序应为：RULE-SET < 内联规则 < MATCH")
	}
}

func TestClash_ProviderNameDedup(t *testing.T) {
	p := goldenPipeline()
	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() 出错：%v", err)
	}
	output := string(got)

	if !strings.Contains(output, "Ad:") {
		t.Error("应有 provider 名称 Ad")
	}
	if !strings.Contains(output, "Ad-2:") {
		t.Error("应有去重后的 provider 名称 Ad-2")
	}
}

func TestClash_RuleProvidersUseTextFormat(t *testing.T) {
	p := goldenPipeline()
	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() 出错：%v", err)
	}
	doc := mustParseClashProviderDoc(t, got)

	provider, ok := doc.RuleProviders["Ad"]
	if !ok {
		t.Fatal("应有 provider Ad")
	}
	if provider.Format != "text" {
		t.Errorf("provider format = %q，期望 %q", provider.Format, "text")
	}
	if provider.Path != "./rule-providers/Ad.txt" {
		t.Errorf("provider path = %q，期望 %q", provider.Path, "./rule-providers/Ad.txt")
	}
}

func TestClash_URLTestHasTolerance(t *testing.T) {
	p := goldenPipeline()
	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() 出错：%v", err)
	}
	if !strings.Contains(string(got), "tolerance: 100") {
		t.Error("url-test 分组应包含 tolerance: 100")
	}
}

func TestClash_GroupOrderRouteBeforeNode(t *testing.T) {
	p := goldenPipeline()
	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() 出错：%v", err)
	}
	output := string(got)

	quickIdx := strings.Index(output, "- name: Quick")
	hkIdx := strings.Index(output, "- name: \"\\U0001F1ED\\U0001F1F0 HK\"")
	if quickIdx < 0 || hkIdx < 0 {
		t.Fatalf("输出中缺少预期的分组：\n%s", output)
	}
	if quickIdx >= hkIdx {
		t.Error("服务组应渲染在节点组之前")
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
		t.Fatalf("Clash() 出错：%v", err)
	}
	output := string(got)

	// 模板中的设置应保留。
	if !strings.Contains(output, "mixed-port:") {
		t.Error("底版模板中的设置应保留")
	}
	if !strings.Contains(output, "allow-lan:") {
		t.Error("底版模板中的设置应保留")
	}
	// 生成的段落应存在。
	if !strings.Contains(output, "proxies:") {
		t.Error("proxies 段应存在")
	}
	if !strings.Contains(output, "proxy-groups:") {
		t.Error("proxy-groups 段应存在")
	}
	if !strings.Contains(output, "rule-providers:") {
		t.Error("rule-providers 段应存在")
	}
	if !strings.Contains(output, "rules:") {
		t.Error("rules 段应存在")
	}
}

func TestClash_EmptyBaseTemplate(t *testing.T) {
	p := goldenPipeline()
	_, err := Clash(p, []byte{})
	if err == nil {
		t.Fatal("空底版模板应返回错误")
	}
	var re *errtype.RenderError
	if !errors.As(err, &re) {
		t.Errorf("期望 *errtype.RenderError，实际为 %T", err)
	}
}

func TestClash_NullYAMLBaseTemplate(t *testing.T) {
	p := goldenPipeline()
	_, err := Clash(p, []byte("---\nnull\n"))
	if err == nil {
		t.Fatal("内容为 null 的 YAML 底版模板应返回错误")
	}
	var re *errtype.RenderError
	if !errors.As(err, &re) {
		t.Errorf("期望 *errtype.RenderError，实际为 %T", err)
	}
}

func TestClash_ProviderNameCollisionWithNatural(t *testing.T) {
	// URL A 和 B 提取出的名称都是 "Ad"，分别生成 "Ad" 和 "Ad-2"。
	// URL C 本来的名称就是 "Ad-2"，会与之冲突。
	p := &model.TargetPipeline{
		Proxies: []model.TargetProxy{
			{Name: "N1", Type: "ss", Server: "s", Port: 1, Params: map[string]string{"cipher": "aes-256-gcm", "password": "p"}, Kind: model.KindSubscription},
		},
		NodeGroups:  []model.TargetGroup{{Name: "G", Strategy: "select", Members: []string{"N1"}}},
		RouteGroups: []model.TargetGroup{{Name: "R", Strategy: "select", Members: []string{"G"}}},
		Rulesets: []model.Ruleset{
			{Policy: "R", URLs: []string{
				"https://example.com/rules/Ad.list",   // "Ad"
				"https://example.com/other/Ad.list",   // "Ad" → 本应得到 "Ad-2"，但是……
				"https://example.com/rules/Ad-2.list", // 本来就叫 "Ad-2" → 不能冲突
			}},
		},
		Fallback: "R",
	}

	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() 出错：%v", err)
	}
	output := string(got)

	// 三个 provider 名称必须互不相同。
	names := []string{"Ad:", "Ad-2:", "Ad-3:"}
	for _, n := range names {
		if !strings.Contains(output, n) {
			t.Errorf("输出中应有 provider 名称 %s", n)
		}
	}
}

// vlessPipeline 构建一个最小 Pipeline，其中只有一个按 overrides 设置的 VLESS 节点。
// overrides 中没有设置的字段默认取 Reality 示例的值。
func vlessPipeline(name string, params map[string]string) *model.TargetPipeline {
	// Reality 基准参数
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
	return &model.TargetPipeline{
		Proxies: []model.TargetProxy{{
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

// T-CLASH-VLESS-001: Reality + TCP + Vision 写出完整的 reality-opts 块。
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
		t.Fatalf("Clash() 出错：%v", err)
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
			t.Errorf("以下输出缺少 %q：\n%s", w, output)
		}
	}
}

// T-CLASH-VLESS-002: security=tls → tls:true，没有 reality-opts。
func TestClash_VlessTCPTLS(t *testing.T) {
	p := vlessPipeline("VL-TLS", map[string]string{
		"security":           "tls",
		"servername":         "sg.example.com",
		"client-fingerprint": "chrome",
		// 确保没有残留的 reality 键
		"reality-public-key": "",
		"reality-short-id":   "",
	})

	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() 出错：%v", err)
	}

	output := string(got)
	if !strings.Contains(output, "tls: true") {
		t.Error("security=tls 时应有 tls: true")
	}
	if !strings.Contains(output, "servername: sg.example.com") {
		t.Error("应有 servername")
	}
	if strings.Contains(output, "reality-opts") {
		t.Error("security=tls 时不得写出 reality-opts")
	}
}

// T-CLASH-VLESS-003: security=none → 没有 tls、servername 和 reality-opts。
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
		t.Fatalf("Clash() 出错：%v", err)
	}

	output := string(got)
	if strings.Contains(output, "tls: true") {
		t.Error("security=none 时不应写出 tls: true")
	}
	if strings.Contains(output, "servername:") {
		t.Error("security=none 时不应写出 servername")
	}
	if strings.Contains(output, "reality-opts") {
		t.Error("security=none 时不应写出 reality-opts")
	}
	if strings.Contains(output, "encryption:") {
		t.Error("空的 encryption 不应写出")
	}
	// 仍必须写出核心字段。
	for _, w := range []string{"type: vless", "network: tcp", "udp: true"} {
		if !strings.Contains(output, w) {
			t.Errorf("以下输出缺少 %q：\n%s", w, output)
		}
	}
}

// T-CLASH-VLESS-004: 非空的 encryption 原样写出。
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
		t.Fatalf("Clash() 出错：%v", err)
	}

	output := string(got)
	if !strings.Contains(output, "encryption: mlkem768x25519plus.native") {
		t.Errorf("非空的 encryption 应被渲染，输出：\n%s", output)
	}
}

// T-CLASH-VLESS-005: 逗号分隔的 alpn 写成 YAML 列表。
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
		t.Fatalf("Clash() 出错：%v", err)
	}

	// YAML 列表把每个元素渲染为以 "- " 开头的单独一行。
	// 检查两个条目都以列表项出现。
	output := string(got)
	if !strings.Contains(output, "alpn:") {
		t.Fatalf("输出中缺少 alpn 键：\n%s", output)
	}
	if !strings.Contains(output, "- h2") {
		t.Errorf("alpn 应以列表项形式包含 h2：\n%s", output)
	}
	if !strings.Contains(output, "- http/1.1") {
		t.Errorf("alpn 应以列表项形式包含 http/1.1：\n%s", output)
	}
}

// T-CLASH-VLESS-006: 未知的 URI type 在渲染前归一化为 tcp。
func TestClash_VlessUnknownTypeFallbackRendersAsTCP(t *testing.T) {
	px := model.TargetProxy{
		Name:   "VL-TCP-FALLBACK",
		Type:   "vless",
		Server: "hk.example.com",
		Port:   443,
		Params: map[string]string{
			"uuid":       "11111111-2222-3333-4444-555555555555",
			"security":   "tls",
			"servername": "hk.example.com",
			"network":    "tcp",
			"encryption": "mlkem768x25519plus.native",
		},
		Kind: model.KindVLess,
	}

	got, err := Clash(&model.TargetPipeline{
		Proxies:  []model.TargetProxy{px},
		Fallback: "DIRECT",
	}, nil)
	if err != nil {
		t.Fatalf("Clash() 出错：%v", err)
	}

	output := string(got)
	if !strings.Contains(output, "network: tcp") {
		t.Errorf("未知的 URI type 应渲染为 network: tcp，输出：\n%s", output)
	}
	for _, block := range []string{"ws-opts:", "http-opts:", "h2-opts:", "grpc-opts:", "xhttp-opts:"} {
		if strings.Contains(output, block) {
			t.Errorf("回退为 tcp 时不应写出传输层 opts 块 %q，输出：\n%s", block, output)
		}
	}
	if !strings.Contains(output, "encryption: mlkem768x25519plus.native") {
		t.Errorf("回退为 tcp 时仍应渲染 encryption，输出：\n%s", output)
	}
}

// T-CLASH-VLESS-007: 总是写出 udp: true。
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
		t.Fatalf("Clash() 出错：%v", err)
	}

	if !strings.Contains(string(got), "udp: true") {
		t.Errorf("每个 vless 节点都应有 udp: true，输出：\n%s", string(got))
	}
}

// T-CLASH-VLESS-008: 上游为 VLESS 的链式节点写出 dialer-proxy。
func TestClash_VlessChainAsDialerProxy(t *testing.T) {
	p := &model.TargetPipeline{
		Proxies: []model.TargetProxy{
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
		t.Fatalf("Clash() 出错：%v", err)
	}

	output := string(got)
	if !strings.Contains(output, "dialer-proxy: VL-HK") {
		t.Errorf("链式节点应写出 dialer-proxy: VL-HK，输出：\n%s", output)
	}
	if !strings.Contains(output, "name: VL-HK") {
		t.Error("上游 VL-HK 必须出现在 proxies 块中")
	}
}

// vlessTLSOverride 返回一组参数覆盖值，让 vlessPipeline 生成使用指定 network
// 传输层的 TLS 节点（没有 reality-opts）。调用方再往返回的 map 里补充
// 传输层专属的键。
func vlessTLSOverride(network string) map[string]string {
	return map[string]string{
		"network":            network,
		"security":           "tls",
		"servername":         "example.com",
		"reality-public-key": "",
		"reality-short-id":   "",
	}
}

// T-CLASH-VLESS-WS: 设置了 path 和 headers.Host 时，ws-opts 写出它们。
func TestClash_VlessWSOpts(t *testing.T) {
	overrides := vlessTLSOverride("ws")
	overrides["ws-path"] = "/ws-route"
	overrides["ws-host"] = "ws.example.com"
	p := vlessPipeline("VL-WS", overrides)

	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash 出错：%v", err)
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
			t.Errorf("以下输出缺少 %q：\n%s", w, out)
		}
	}
}

// T-CLASH-VLESS-HTTP: http-opts 把 path 和 headers.Host 写成 YAML 列表。
func TestClash_VlessHTTPOpts(t *testing.T) {
	overrides := vlessTLSOverride("http")
	overrides["http-path"] = "/api/v1"
	overrides["http-host"] = "api.example.com"
	p := vlessPipeline("VL-HTTP", overrides)

	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash 出错：%v", err)
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
			t.Errorf("以下输出缺少 %q：\n%s", w, out)
		}
	}
}

// T-CLASH-VLESS-H2: h2-opts 写出 host（列表）和 path（标量）。
func TestClash_VlessH2Opts(t *testing.T) {
	overrides := vlessTLSOverride("h2")
	overrides["h2-path"] = "/h2"
	overrides["h2-host"] = "h2.example.com"
	p := vlessPipeline("VL-H2", overrides)

	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash 出错：%v", err)
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
			t.Errorf("以下输出缺少 %q：\n%s", w, out)
		}
	}
}

// T-CLASH-VLESS-GRPC: grpc-opts 写出 grpc-service-name。
func TestClash_VlessGrpcOpts(t *testing.T) {
	overrides := vlessTLSOverride("grpc")
	overrides["grpc-service-name"] = "GunService"
	p := vlessPipeline("VL-GRPC", overrides)

	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash 出错：%v", err)
	}
	out := string(got)
	for _, w := range []string{
		"network: grpc",
		"grpc-opts:",
		"grpc-service-name: GunService",
	} {
		if !strings.Contains(out, w) {
			t.Errorf("以下输出缺少 %q：\n%s", w, out)
		}
	}
}

// T-CLASH-VLESS-XHTTP: xhttp-opts 写出 mode、path 和 host。
func TestClash_VlessXhttpOpts(t *testing.T) {
	overrides := vlessTLSOverride("xhttp")
	overrides["xhttp-mode"] = "packet-up"
	overrides["xhttp-path"] = "/xh"
	overrides["xhttp-host"] = "xh.example.com"
	p := vlessPipeline("VL-XH", overrides)

	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash 出错：%v", err)
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
			t.Errorf("以下输出缺少 %q：\n%s", w, out)
		}
	}
}

// TestClash_VlessAllKnownNetworksProduceConsistentOutput 是跨文件的同步守卫，
// 覆盖显式处理的非 tcp VLESS 传输层集合。
//
// 解析器的已知传输层表（pipeline.vlessAllowedNetworks）和渲染器的分派
// （render.emitClashVLessTransportOpts）各自独立维护。如果有人新增了已知
// 传输层却没有更新渲染器（或本测试），这个 network 会以 `network:` 值的形式
// 出现在 Clash YAML 中，却没有对应的 `*-opts:` 块，仓库声明的支持范围就会
// 悄悄走样。
//
// 下表列出渲染器专门处理的每一种已知非回退传输层。tcp 不应有传输层 opts 块；
// 其余每一项都必须写出 `<network>-opts`。
//
// 新增 VLESS network 时的**扩展清单**：
//  1. 加入 internal/pipeline/vlessuri.go 中的 `vlessAllowedNetworks`
//  2. 在 clash.go 的 `emitClashVLessTransportOpts` 中增加分派分支
//  3. 在这里加入该 network 及其预期的 opts 前缀
//
// 漏掉三步中的任何一步，本测试都会失败，失败信息会指回这三处同步点。
func TestClash_VlessAllKnownNetworksProduceConsistentOutput(t *testing.T) {
	cases := []struct {
		network       string
		wantOptsBlock string // 为空表示不应有 *-opts 块
		seedKey       string // 一个传输层专属的 Params 键，用来触发写出 opts
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

	// hasExactOptsLine 判断 YAML 中是否有某一行去掉首尾空白后恰好等于给定的
	// opts 块标记。之所以需要它，是因为 `http-opts:` 是 `xhttp-opts:` 的子串，
	// 直接用 strings.Contains 会让 xhttp 的输出同时匹配两者。
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
				t.Fatalf("Clash 出错：%v", err)
			}
			out := string(got)

			if !strings.Contains(out, "network: "+tc.network) {
				t.Errorf("输出中应有 network: %s：\n%s", tc.network, out)
			}

			if tc.wantOptsBlock == "" {
				// tcp：确保没有混入任何传输层 opts 块。
				for _, block := range allOptsBlocks {
					if hasExactOptsLine(out, block) {
						t.Errorf("network=%s 不得写出 %q（渲染器同步走样？）；输出：\n%s", tc.network, block, out)
					}
				}
				return
			}

			if !hasExactOptsLine(out, tc.wantOptsBlock) {
				t.Errorf("network=%s 是已知传输层，但渲染器没有写出 %q：请同时更新 clash.go 中的 emitClashVLessTransportOpts 和本测试的用例列表；输出：\n%s",
					tc.network, tc.wantOptsBlock, out)
			}
			// 确保没有混入其他 opts 块（例如 network=http 时出现 ws-opts）。
			for _, block := range allOptsBlocks {
				if block == tc.wantOptsBlock {
					continue
				}
				if hasExactOptsLine(out, block) {
					t.Errorf("network=%s 的输出混入了不相干的 %q：\n%s", tc.network, block, out)
				}
			}
		})
	}
}

// T-CLASH-VLESS-EMPTY-OPTS: network=ws 且没有 path/host 时，不写出空的
// ws-opts 块（Clash 把缺失视为默认的空值）。
func TestClash_VlessNetworkWithNoSubfieldsOmitsOptsBlock(t *testing.T) {
	p := vlessPipeline("VL-WS-BARE", vlessTLSOverride("ws"))

	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash 出错：%v", err)
	}
	out := string(got)
	if !strings.Contains(out, "network: ws") {
		t.Errorf("必须写出 network: ws，实际：\n%s", out)
	}
	if strings.Contains(out, "ws-opts:") {
		t.Errorf("空的 ws-opts 块应省略，实际：\n%s", out)
	}
}

// T-RND-CLASH-NATIVE-001: 原样复制的 Clash 声明保留任意字段，
// 名称仍以在目标客户端内分配的名字为准。
func TestClash_NativeProxyPreservesFieldsAndUsesCurrentName(t *testing.T) {
	p := &model.TargetPipeline{
		Proxies: []model.TargetProxy{{
			Name: "HK-YAML②",
			Type: "trojan",
			Kind: model.KindSubscription,
			Native: &model.NativeProxy{
				Format: model.NativeProxyFormatClash,
				Payload: `name: HK-YAML
type: trojan
server: hk.example.com
port: 443
password: secret
ws-opts:
  path: /edge
  headers:
    Host: edge.example.com
`,
			},
		}},
		Fallback: "DIRECT",
	}

	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() 出错：%v", err)
	}
	output := string(got)
	for _, want := range []string{"name: HK-YAML②", "type: trojan", "password: secret", "ws-opts:", "Host: edge.example.com"} {
		if !strings.Contains(output, want) {
			t.Errorf("原生 Clash 输出缺少 %q：\n%s", want, output)
		}
	}
	if strings.Contains(output, "name: HK-YAML\n") {
		t.Errorf("分配目标名称后仍残留旧的来源名称：\n%s", output)
	}
}

// T-RND-CLASH-NATIVE-002: Surge 原生声明到达 Clash 属于违反投影约定，
// 不应按通用方式有损渲染。
func TestClash_RejectsSurgeNativeProxy(t *testing.T) {
	p := &model.TargetPipeline{
		Proxies: []model.TargetProxy{{
			Name: "HK-CONF",
			Type: "trojan",
			Kind: model.KindSubscription,
			Native: &model.NativeProxy{
				Format:  model.NativeProxyFormatSurge,
				Payload: "trojan, hk.example.com, 443, password=secret",
			},
		}},
		Fallback: "DIRECT",
	}

	_, err := Clash(p, nil)
	if err == nil {
		t.Fatal("期望 RenderError，实际为 nil")
	}
	var renderErr *errtype.RenderError
	if !errors.As(err, &renderErr) {
		t.Fatalf("错误类型 = %T，期望 *errtype.RenderError", err)
	}
	if renderErr.Code != errtype.CodeRenderClashProxyInvalid {
		t.Fatalf("Code = %q，期望 %q", renderErr.Code, errtype.CodeRenderClashProxyInvalid)
	}
}
