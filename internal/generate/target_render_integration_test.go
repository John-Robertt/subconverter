package generate_test

import (
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/model"
	"github.com/John-Robertt/subconverter/internal/render"
	"github.com/John-Robertt/subconverter/internal/target"
)

func mustRenderClashProjected(t *testing.T, p *model.Pipeline) []byte {
	t.Helper()
	projected, err := target.ForClash(p)
	if err != nil {
		t.Fatalf("target.ForClash() 出错：%v", err)
	}
	out, err := render.Clash(projected, nil)
	if err != nil {
		t.Fatalf("Clash() 出错：%v", err)
	}
	return out
}

func mustRenderSurgeProjected(t *testing.T, p *model.Pipeline) []byte {
	t.Helper()
	projected, err := target.ForSurge(p)
	if err != nil {
		t.Fatalf("target.ForSurge() 出错：%v", err)
	}
	out, err := render.Surge(projected, "", nil)
	if err != nil {
		t.Fatalf("Surge() 出错：%v", err)
	}
	return out
}

func buildSnellFilterPipeline() *model.Pipeline {
	return &model.Pipeline{
		Proxies: []model.Proxy{
			{ID: 1, Name: "HK-01", Type: "ss", Server: "hk.example.com", Port: 8388, Kind: model.KindSubscription},
			{ID: 2, Name: "HK-Snell", Type: "snell", Server: "1.2.3.4", Port: 57891, Params: map[string]string{"psk": "x", "version": "4"}, Kind: model.KindSnell},
			{ID: 3, Name: "SG-Snell", Type: "snell", Server: "5.6.7.8", Port: 8989, Params: map[string]string{"psk": "y", "version": "4"}, Kind: model.KindSnell},
		},
		NodeGroups: []model.NodeGroup{
			{Name: "GRP_HK", Strategy: "select", Members: []model.ProxyID{1, 2}},
			{Name: "GRP_SG", Strategy: "select", Members: []model.ProxyID{3}},
		},
		RouteGroups: []model.RouteGroup{
			{Name: "SVC_Quick", Strategy: "select", Members: []model.PolicyRef{
				{Kind: model.PolicyRefNodeGroup, Name: "GRP_HK"},
				{Kind: model.PolicyRefNodeGroup, Name: "GRP_SG"},
			}},
			{Name: "SVC_SGOnly", Strategy: "select", Members: []model.PolicyRef{
				{Kind: model.PolicyRefNodeGroup, Name: "GRP_SG"},
			}},
			{Name: "SVC_FINAL", Strategy: "select", Members: []model.PolicyRef{
				{Kind: model.PolicyRefNodeGroup, Name: "GRP_HK"},
				{Kind: model.PolicyRefRouteGroup, Name: "SVC_SGOnly"},
				{Kind: model.PolicyRefReserved, Name: "DIRECT"},
			}},
		},
		Rulesets: []model.Ruleset{
			{Policy: "SVC_SGOnly", URLs: []string{"https://example.com/NF.list"}},
			{Policy: "SVC_Quick", URLs: []string{"https://example.com/Global.list"}},
		},
		Rules: []model.Rule{
			{Raw: "GEOIP,SG,SVC_SGOnly", Policy: "SVC_SGOnly"},
			{Raw: "GEOIP,CN,SVC_Quick", Policy: "SVC_Quick"},
		},
		Fallback:   "SVC_FINAL",
		AllProxies: []model.ProxyID{1, 2, 3},
	}
}

// T-TGT-RND-001：以 Snell 为上游的链式节点在 Surge 中可见，在 Clash 中被过滤
func TestRender_SnellUpstreamChainVisibility(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			{ID: 1, Name: "HK-Snell", Type: "snell", Server: "1.2.3.4", Port: 57891, Params: map[string]string{"psk": "x", "version": "4"}, Kind: model.KindSnell},
			{ID: 2, Name: "HK-01", Type: "ss", Server: "hk.example.com", Port: 8388, Params: map[string]string{"cipher": "aes-256-gcm", "password": "s"}, Kind: model.KindSubscription},
			{ID: 3, Name: "HK-Snell→MY-PROXY", Type: "socks5", Server: "10.0.0.1", Port: 1080, Params: map[string]string{"username": "u", "password": "p"}, Kind: model.KindChained, DialerID: 1, ChainNameSuffix: "MY-PROXY"},
			{ID: 4, Name: "HK-01→MY-PROXY", Type: "socks5", Server: "10.0.0.1", Port: 1080, Params: map[string]string{"username": "u", "password": "p"}, Kind: model.KindChained, DialerID: 2, ChainNameSuffix: "MY-PROXY"},
		},
		NodeGroups: []model.NodeGroup{
			{Name: "GRP_HK", Strategy: "select", Members: []model.ProxyID{2, 1}},
			{Name: "GRP_CHAIN", Strategy: "select", Members: []model.ProxyID{3, 4}},
		},
		RouteGroups: []model.RouteGroup{
			{Name: "SVC_FINAL", Strategy: "select", Members: []model.PolicyRef{
				{Kind: model.PolicyRefNodeGroup, Name: "GRP_HK"},
				{Kind: model.PolicyRefNodeGroup, Name: "GRP_CHAIN"},
				{Kind: model.PolicyRefReserved, Name: "DIRECT"},
			}},
		},
		Fallback:   "SVC_FINAL",
		AllProxies: []model.ProxyID{1, 2},
	}

	surgeOut := mustRenderSurgeProjected(t, p)
	clashOut := mustRenderClashProjected(t, p)

	if !strings.Contains(string(surgeOut), "HK-Snell→MY-PROXY = socks5, 10.0.0.1, 1080") {
		t.Errorf("Surge 缺少以 snell 为上游的链式节点行：\n%s", string(surgeOut))
	}
	if !strings.Contains(string(surgeOut), "underlying-proxy=HK-Snell") {
		t.Errorf("Surge 缺少 underlying-proxy=HK-Snell 指令：\n%s", string(surgeOut))
	}
	if strings.Contains(string(clashOut), "HK-Snell→MY-PROXY") {
		t.Errorf("Clash 输出泄露了以 Snell 为上游的链式节点：\n%s", string(clashOut))
	}
	if !strings.Contains(string(clashOut), "HK-01→MY-PROXY") {
		t.Errorf("Clash 缺少以 SS 为上游的链式节点：\n%s", string(clashOut))
	}
}

// T-TGT-RND-002：Snell 节点和分组在 Surge 中可见，在 Clash 中被级联过滤
func TestRender_SnellVisibilityDiffersBetweenFormats(t *testing.T) {
	p := buildSnellFilterPipeline()

	surgeOut := mustRenderSurgeProjected(t, p)
	clashOut := mustRenderClashProjected(t, p)

	if !strings.Contains(string(surgeOut), "HK-Snell = snell, 1.2.3.4, 57891, psk=x, version=4") {
		t.Errorf("Surge 输出缺少 HK-Snell 行：\n%s", string(surgeOut))
	}
	if !strings.Contains(string(surgeOut), "SG-Snell = snell, 5.6.7.8, 8989, psk=y, version=4") {
		t.Errorf("Surge 输出缺少 SG-Snell 行：\n%s", string(surgeOut))
	}
	if !strings.Contains(string(surgeOut), "GRP_SG = select") {
		t.Errorf("Surge 输出缺少 GRP_SG 分组：\n%s", string(surgeOut))
	}

	if strings.Contains(string(clashOut), "HK-Snell") || strings.Contains(string(clashOut), "SG-Snell") {
		t.Errorf("Clash 输出泄露了 snell 节点名：\n%s", string(clashOut))
	}
	if strings.Contains(string(clashOut), "GRP_SG") {
		t.Errorf("Clash 输出仍引用已删除的 GRP_SG：\n%s", string(clashOut))
	}
	if !strings.Contains(string(clashOut), "GRP_HK") {
		t.Errorf("Clash 输出缺少应保留的 GRP_HK：\n%s", string(clashOut))
	}
}

// T-TGT-RND-003：VLESS 节点在 Clash 中可见，在 Surge 中被级联过滤
func TestRender_VLessVisibilityDiffersBetweenFormats(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			{ID: 1, Name: "HK-01", Type: "ss", Server: "hk.example.com", Port: 8388, Params: map[string]string{"cipher": "aes-256-gcm", "password": "pw"}, Kind: model.KindSubscription},
			{ID: 2, Name: "HK-VL", Type: "vless", Server: "1.2.3.4", Port: 443, Params: map[string]string{"uuid": "11111111-2222-3333-4444-555555555555", "security": "tls", "network": "tcp", "servername": "www.example.com"}, Kind: model.KindVLess},
		},
		NodeGroups: []model.NodeGroup{
			{Name: "GRP_HK", Strategy: "select", Members: []model.ProxyID{1, 2}},
		},
		RouteGroups: []model.RouteGroup{
			{Name: "SVC_FINAL", Strategy: "select", Members: []model.PolicyRef{
				{Kind: model.PolicyRefNodeGroup, Name: "GRP_HK"},
				{Kind: model.PolicyRefReserved, Name: "DIRECT"},
			}},
		},
		Fallback:   "SVC_FINAL",
		AllProxies: []model.ProxyID{1, 2},
	}

	clashOut := mustRenderClashProjected(t, p)
	if !strings.Contains(string(clashOut), "HK-VL") {
		t.Error("VLESS 节点应出现在 Clash 输出中")
	}
	if !strings.Contains(string(clashOut), "type: vless") {
		t.Error("VLESS 类型应出现在 Clash 输出中")
	}

	surgeOut := mustRenderSurgeProjected(t, p)
	if strings.Contains(string(surgeOut), "HK-VL") {
		t.Error("VLESS 节点不应出现在 Surge 输出中")
	}
	if strings.Contains(string(surgeOut), "= vless") {
		t.Error("VLESS 类型不应出现在 Surge 输出中")
	}
	if !strings.Contains(string(surgeOut), "HK-01") {
		t.Error("SS 节点仍必须在 Surge 中渲染")
	}
}

// T-TGT-RND-004：两种目标投影都支持 AnyTLS，但每个渲染器只输出
// 本目标客户端约定的字段。
func TestRender_AnyTLSVisibleInBothFormats(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{{
			ID:     1,
			Name:   "HK-AnyTLS",
			Type:   "anytls",
			Server: "hk.example.com",
			Port:   443,
			Params: map[string]string{
				"password":                       "pw",
				"sni":                            "edge.example.com",
				"skip-cert-verify":               "true",
				"udp-relay":                      "true",
				"tfo":                            "true",
				"client-fingerprint":             "chrome",
				"alpn":                           "h2,http/1.1",
				"server-cert-fingerprint-sha256": "abc123",
			},
			Kind: model.KindSubscription,
		}},
		NodeGroups: []model.NodeGroup{
			{Name: "GRP_HK", Strategy: "select", Members: []model.ProxyID{1}},
		},
		RouteGroups: []model.RouteGroup{
			{Name: "SVC_FINAL", Strategy: "select", Members: []model.PolicyRef{
				{Kind: model.PolicyRefNodeGroup, Name: "GRP_HK"},
				{Kind: model.PolicyRefReserved, Name: "DIRECT"},
			}},
		},
		Fallback:   "SVC_FINAL",
		AllProxies: []model.ProxyID{1},
	}

	clashOut := string(mustRenderClashProjected(t, p))
	for _, want := range []string{"name: HK-AnyTLS", "type: anytls", "udp: true", "client-fingerprint: chrome"} {
		if !strings.Contains(clashOut, want) {
			t.Errorf("Clash 输出缺少 %q：\n%s", want, clashOut)
		}
	}

	surgeOut := string(mustRenderSurgeProjected(t, p))
	for _, want := range []string{"HK-AnyTLS = anytls", "password=pw", "sni=edge.example.com", "skip-cert-verify=true", "server-cert-fingerprint-sha256=abc123"} {
		if !strings.Contains(surgeOut, want) {
			t.Errorf("Surge 输出缺少 %q：\n%s", want, surgeOut)
		}
	}
	for _, unwanted := range []string{"udp-relay=true", "tfo=true", "client-fingerprint=chrome", "tls=true"} {
		if strings.Contains(surgeOut, unwanted) {
			t.Errorf("Surge 输出不应包含 %q：\n%s", unwanted, surgeOut)
		}
	}
}
