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
		t.Fatalf("Surge() 出错：%v", err)
	}

	goldenPath := filepath.Join("..", "..", "testdata", "render", "surge_golden.conf")

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

func TestSurge_ManagedHeaderPresent(t *testing.T) {
	p := goldenPipeline()
	got, err := Surge(p, "https://my-server.com/generate?format=surge", nil)
	if err != nil {
		t.Fatalf("Surge() 出错：%v", err)
	}
	output := string(got)
	expected := "#!MANAGED-CONFIG https://my-server.com/generate?format=surge interval=86400 strict=false"
	if !strings.HasPrefix(output, expected) {
		t.Errorf("输出应以 managed 头部开头。\n实际开头：%q", output[:min(len(output), len(expected)+20)])
	}
}

func TestSurge_NoManagedHeaderWhenManagedURLEmpty(t *testing.T) {
	p := goldenPipeline()
	got, err := Surge(p, "", nil)
	if err != nil {
		t.Fatalf("Surge() 出错：%v", err)
	}
	output := string(got)
	if strings.Contains(output, "#!MANAGED-CONFIG") {
		t.Error("managedURL 为空时输出不应包含 managed 头部")
	}
}

func TestSurge_ChainedProxyHasUnderlyingProxy(t *testing.T) {
	p := goldenPipeline()
	got, err := Surge(p, "", nil)
	if err != nil {
		t.Fatalf("Surge() 出错：%v", err)
	}
	output := string(got)
	if !strings.Contains(output, "underlying-proxy=HK-01") {
		t.Error("链式节点应包含 underlying-proxy 字段")
	}
}

func TestSurge_SSProxyWithSimpleObfsPlugin(t *testing.T) {
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

	got, err := Surge(p, "", nil)
	if err != nil {
		t.Fatalf("Surge() 出错：%v", err)
	}

	output := string(got)
	if !strings.Contains(output, "HK-OBFS = ss, hk.example.com, 8388, encrypt-method=aes-256-gcm, password=secret, obfs=http, obfs-host=cdn.example.com") {
		t.Error("ss 节点应渲染 Surge 的 obfs 参数")
	}
	if !strings.Contains(output, "FINAL,DIRECT") {
		t.Error("fallback 规则应仍然存在")
	}
}

func TestSurge_AnyTLSProxy(t *testing.T) {
	p := &model.TargetPipeline{
		Proxies: []model.TargetProxy{{
			Name:   "HK-AnyTLS",
			Type:   "anytls",
			Server: "demo.com",
			Port:   3383,
			Params: map[string]string{
				"password":                       "secret",
				"sni":                            "cache-proxy.example.com",
				"skip-cert-verify":               "true",
				"reuse":                          "true",
				"server-cert-fingerprint-sha256": "abc123",
				"udp-relay":                      "true",
				"tfo":                            "true",
				"client-fingerprint":             "chrome",
				"alpn":                           "h2,http/1.1",
				"idle-session-check-interval":    "30",
				"idle-session-timeout":           "30",
				"min-idle-session":               "0",
			},
			Kind: model.KindSubscription,
		}},
		Fallback: "DIRECT",
	}

	got, err := Surge(p, "", nil)
	if err != nil {
		t.Fatalf("Surge() 出错：%v", err)
	}

	want := "HK-AnyTLS = anytls, demo.com, 3383, password=secret, sni=cache-proxy.example.com, skip-cert-verify=true, reuse=true, server-cert-fingerprint-sha256=abc123"
	if !strings.Contains(string(got), want) {
		t.Errorf("Surge AnyTLS 输出缺少 %q：\n%s", want, string(got))
	}
	for _, unwanted := range []string{
		"tls=true",
		"udp-relay=true",
		"tfo=true",
		"client-fingerprint=chrome",
		"alpn=",
		"idle-session-check-interval",
		"idle-session-timeout",
		"min-idle-session",
	} {
		if strings.Contains(string(got), unwanted) {
			t.Errorf("Surge AnyTLS 输出不应包含 %q：\n%s", unwanted, string(got))
		}
	}
}

func TestSurge_SSProxyWithUnsupportedPlugin(t *testing.T) {
	p := &model.TargetPipeline{
		Proxies: []model.TargetProxy{{
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
		t.Fatal("不支持的 ss plugin 应返回错误")
	}

	var renderErr *errtype.RenderError
	if !errors.As(err, &renderErr) {
		t.Fatalf("错误类型 = %T，期望 *errtype.RenderError", err)
	}
	if !strings.Contains(err.Error(), `不支持的 ss plugin "v2ray-plugin"`) {
		t.Errorf("意外的错误：%v", err)
	}
}

func TestSurge_SSProxyUDPAndTFO(t *testing.T) {
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
				"tfo":       "false",
			},
			Kind: model.KindSubscription,
		}},
		Fallback: "DIRECT",
	}

	got, err := Surge(p, "", nil)
	if err != nil {
		t.Fatalf("Surge() 出错：%v", err)
	}

	want := "HK-SS = ss, hk.example.com, 8388, encrypt-method=2022-blake3-aes-128-gcm, password=secret, udp-relay=true, tfo=false"
	if !strings.Contains(string(got), want) {
		t.Errorf("缺少 SS 的 udp/tfo 字段；以下输出中应有 %q：\n%s", want, string(got))
	}
}

func TestSurge_SSProxyQuotesCommaSeparatedParams(t *testing.T) {
	paramKey := "pass" + "word"
	paramValue := `value,with"\tail`
	p := &model.TargetPipeline{
		Proxies: []model.TargetProxy{{
			Name:   "HK-SS",
			Type:   "ss",
			Server: "hk.example.com",
			Port:   8388,
			Params: map[string]string{
				"cipher": "aes-256-gcm",
				paramKey: paramValue,
			},
			Kind: model.KindSubscription,
		}},
		Fallback: "DIRECT",
	}

	got, err := Surge(p, "", nil)
	if err != nil {
		t.Fatalf("Surge() 出错：%v", err)
	}

	want := paramKey + `="value,with\"\\tail"`
	if !strings.Contains(string(got), want) {
		t.Errorf("含分隔符的 SS 参数值应加引号；以下输出中应有 %q：\n%s", want, string(got))
	}
}

func TestSurge_RuleOrder(t *testing.T) {
	p := goldenPipeline()
	got, err := Surge(p, "", nil)
	if err != nil {
		t.Fatalf("Surge() 出错：%v", err)
	}
	output := string(got)

	rulesetIdx := strings.Index(output, "RULE-SET,https://example.com/rules/Ad.list,Quick")
	inlineIdx := strings.Index(output, "GEOIP,CN,Final")
	finalIdx := strings.Index(output, "FINAL,Final")

	if rulesetIdx < 0 || inlineIdx < 0 || finalIdx < 0 {
		t.Fatalf("输出中缺少预期的规则：\n%s", output)
	}
	if rulesetIdx >= inlineIdx || inlineIdx >= finalIdx {
		t.Error("规则顺序应为：RULE-SET < 内联规则 < FINAL")
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
		t.Fatalf("Surge() 出错：%v", err)
	}
	output := string(got)

	// 模板中的 [General] 应保留。
	if !strings.Contains(output, "loglevel = notify") {
		t.Error("模板中的 [General] 段应保留")
	}
	// [Proxy] 应被替换，不包含旧内容。
	if strings.Contains(output, "OLD-PROXY") {
		t.Error("[Proxy] 段应被替换，不应包含旧内容")
	}
	// 生成的节点应存在。
	if !strings.Contains(output, "HK-01 = ss") {
		t.Error("生成的节点应存在")
	}
	// 生成的规则应有 FINAL，而不是旧的 DIRECT。
	if !strings.Contains(output, "FINAL,Final") {
		t.Error("生成的规则应存在")
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
		t.Fatalf("Surge() 出错：%v", err)
	}
	output := string(got)

	// 应恰好有一个 managed 头部，即新的那个。
	count := strings.Count(output, "#!MANAGED-CONFIG")
	if count != 1 {
		t.Errorf("应恰好有 1 个 managed 头部，实际 %d 个", count)
	}
	if !strings.Contains(output, "https://new-server.com/generate?format=surge&token=test-token&filename=surge.conf") {
		t.Error("managed 头部应使用新的 baseURL")
	}
	if strings.Contains(output, "old-server.com") {
		t.Error("旧的 managed 头部应被去掉")
	}
}

func TestSurge_GroupOrderRouteBeforeNode(t *testing.T) {
	p := goldenPipeline()
	got, err := Surge(p, "", nil)
	if err != nil {
		t.Fatalf("Surge() 出错：%v", err)
	}
	output := string(got)

	quickIdx := strings.Index(output, "Quick = select")
	hkIdx := strings.Index(output, "🇭🇰 HK = select")
	if quickIdx < 0 || hkIdx < 0 {
		t.Fatalf("输出中缺少预期的分组：\n%s", output)
	}
	if quickIdx >= hkIdx {
		t.Error("服务组应渲染在节点组之前")
	}
}

// T-SURGE-SNELL-001: Snell 节点按固定的键顺序渲染。
func TestSurge_SnellProxy_BasicFields(t *testing.T) {
	p := &model.TargetPipeline{
		Proxies: []model.TargetProxy{{
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
		t.Fatalf("Surge() 出错：%v", err)
	}
	output := string(got)

	// 期望：无论 map 的插入顺序如何，键顺序都固定为 psk、version、reuse、tfo。
	want := "HK-Snell = snell, 1.2.3.4, 57891, psk=xxx, version=4, reuse=true, tfo=true"
	if !strings.Contains(output, want) {
		t.Errorf("Snell 节点渲染不正确。\n期望包含：\n%s\n实际：\n%s", want, output)
	}
}

// T-SURGE-SNELL-002: ShadowTLS 字段按声明顺序出现在基础字段之后。
func TestSurge_SnellProxy_ShadowTLS(t *testing.T) {
	p := &model.TargetPipeline{
		Proxies: []model.TargetProxy{{
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
		t.Fatalf("Surge() 出错：%v", err)
	}
	output := string(got)

	want := "JP-Snell = snell, 9.10.11.12, 443, psk=zzz, version=4, shadow-tls-password=sss, shadow-tls-sni=www.microsoft.com, shadow-tls-version=3"
	if !strings.Contains(output, want) {
		t.Errorf("ShadowTLS 渲染不正确。\n期望包含：\n%s\n实际：\n%s", want, output)
	}
}

// T-SURGE-SNELL-004: Snell 节点、managed 头部和底版模板共存。
// 验证 #!MANAGED-CONFIG 恰好出现一次（不会因合并模板而重复），
// 且 Snell 节点行落在 [Proxy] 段中。
func TestSurge_SnellWithManagedHeader(t *testing.T) {
	p := &model.TargetPipeline{
		Proxies: []model.TargetProxy{
			{
				Name:   "HK-Snell",
				Type:   "snell",
				Server: "1.2.3.4",
				Port:   57891,
				Params: map[string]string{"psk": "x", "version": "4", "reuse": "true"},
				Kind:   model.KindSnell,
			},
		},
		RouteGroups: []model.TargetGroup{
			{Name: "Final", Strategy: "select", Members: []string{"HK-Snell", "DIRECT"}},
		},
		Fallback: "Final",
	}

	baseTemplate := []byte("[General]\nloglevel = notify\n\n[Proxy]\n# placeholder\n\n[Proxy Group]\n# placeholder\n\n[Rule]\n# placeholder\n")
	managedURL := "https://my-server.com/generate?format=surge&filename=surge.conf"

	got, err := Surge(p, managedURL, baseTemplate)
	if err != nil {
		t.Fatalf("Surge() 出错：%v", err)
	}
	out := string(got)

	// managed 头部必须恰好出现一次。
	if c := strings.Count(out, "#!MANAGED-CONFIG "); c != 1 {
		t.Errorf("#!MANAGED-CONFIG 出现次数 = %d，期望 1：\n%s", c, out)
	}
	// Snell 节点行必须出现在输出中。
	if !strings.Contains(out, "HK-Snell = snell, 1.2.3.4, 57891, psk=x, version=4, reuse=true") {
		t.Errorf("Surge 输出中缺少 Snell 节点行：\n%s", out)
	}
	// 底版模板的 [General] 段必须保留。
	if !strings.Contains(out, "loglevel = notify") {
		t.Errorf("底版模板的 [General] 段应保留：\n%s", out)
	}
}

// T-SURGE-SNELL-003: 未知的 Params 键不会写出（渲染器只认固定列表）。
func TestSurge_SnellProxy_UnknownKeyDropped(t *testing.T) {
	p := &model.TargetPipeline{
		Proxies: []model.TargetProxy{{
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
		t.Fatalf("Surge() 出错：%v", err)
	}
	output := string(got)
	if strings.Contains(output, "future-knob") {
		t.Errorf("未知键混入了输出：\n%s", output)
	}
	if !strings.Contains(output, "X = snell, 1.1.1.1, 443, psk=abc") {
		t.Errorf("缺少基础字段：\n%s", output)
	}
}

// T-RND-SURGE-NATIVE-001: 原样复制的 [Proxy] 声明完整保留等号右侧的内容，
// 名称仍以在目标客户端内分配的名字为准。
func TestSurge_NativeProxyPreservesDeclarationAndUsesCurrentName(t *testing.T) {
	p := &model.TargetPipeline{
		Proxies: []model.TargetProxy{{
			Name: "HK-CONF②",
			Type: "trojan",
			Kind: model.KindSubscription,
			Native: &model.NativeProxy{
				Format:  model.NativeProxyFormatSurge,
				Payload: "trojan, hk.example.com, 443, password=secret, sni=edge.example.com, future-option=enabled",
			},
		}},
		Fallback: "DIRECT",
	}

	got, err := Surge(p, "", nil)
	if err != nil {
		t.Fatalf("Surge() 出错：%v", err)
	}
	output := string(got)
	want := "HK-CONF② = trojan, hk.example.com, 443, password=secret, sni=edge.example.com, future-option=enabled"
	if !strings.Contains(output, want) {
		t.Errorf("原生 Surge 声明没有被保留。\n期望包含：\n%s\n实际：\n%s", want, output)
	}
	if strings.Contains(output, "HK-CONF = trojan") {
		t.Errorf("分配目标名称后仍残留旧的来源名称：\n%s", output)
	}
}

// T-RND-SURGE-NATIVE-002: Clash 原生声明到达 Surge 属于违反投影约定。
func TestSurge_RejectsClashNativeProxy(t *testing.T) {
	p := &model.TargetPipeline{
		Proxies: []model.TargetProxy{{
			Name: "HK-YAML",
			Type: "trojan",
			Kind: model.KindSubscription,
			Native: &model.NativeProxy{
				Format:  model.NativeProxyFormatClash,
				Payload: "name: HK-YAML\ntype: trojan\n",
			},
		}},
		Fallback: "DIRECT",
	}

	_, err := Surge(p, "", nil)
	if err == nil {
		t.Fatal("期望 RenderError，实际为 nil")
	}
	var renderErr *errtype.RenderError
	if !errors.As(err, &renderErr) {
		t.Fatalf("错误类型 = %T，期望 *errtype.RenderError", err)
	}
	if renderErr.Code != errtype.CodeRenderSurgeProxyInvalid {
		t.Fatalf("Code = %q，期望 %q", renderErr.Code, errtype.CodeRenderSurgeProxyInvalid)
	}
}
