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
		t.Fatalf("target.ForClash() error: %v", err)
	}
	out, err := render.Clash(projected, nil)
	if err != nil {
		t.Fatalf("Clash() error: %v", err)
	}
	return out
}

func mustRenderSurgeProjected(t *testing.T, p *model.Pipeline) []byte {
	t.Helper()
	projected, err := target.ForSurge(p)
	if err != nil {
		t.Fatalf("target.ForSurge() error: %v", err)
	}
	out, err := render.Surge(projected, "", nil)
	if err != nil {
		t.Fatalf("Surge() error: %v", err)
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

// T-TGT-RND-001: Snell-upstream chained node visible in Surge but filtered from Clash
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
		t.Errorf("Surge missing snell-chained proxy line:\n%s", string(surgeOut))
	}
	if !strings.Contains(string(surgeOut), "underlying-proxy=HK-Snell") {
		t.Errorf("Surge missing underlying-proxy=HK-Snell directive:\n%s", string(surgeOut))
	}
	if strings.Contains(string(clashOut), "HK-Snell→MY-PROXY") {
		t.Errorf("Clash output leaked Snell-upstream chained node:\n%s", string(clashOut))
	}
	if !strings.Contains(string(clashOut), "HK-01→MY-PROXY") {
		t.Errorf("Clash missing SS-upstream chained node:\n%s", string(clashOut))
	}
}

// T-TGT-RND-002: Snell nodes and groups visible in Surge, cascade-filtered from Clash
func TestRender_SnellVisibilityDiffersBetweenFormats(t *testing.T) {
	p := buildSnellFilterPipeline()

	surgeOut := mustRenderSurgeProjected(t, p)
	clashOut := mustRenderClashProjected(t, p)

	if !strings.Contains(string(surgeOut), "HK-Snell = snell, 1.2.3.4, 57891, psk=x, version=4") {
		t.Errorf("Surge output missing HK-Snell line:\n%s", string(surgeOut))
	}
	if !strings.Contains(string(surgeOut), "SG-Snell = snell, 5.6.7.8, 8989, psk=y, version=4") {
		t.Errorf("Surge output missing SG-Snell line:\n%s", string(surgeOut))
	}
	if !strings.Contains(string(surgeOut), "GRP_SG = select") {
		t.Errorf("Surge output missing GRP_SG group:\n%s", string(surgeOut))
	}

	if strings.Contains(string(clashOut), "HK-Snell") || strings.Contains(string(clashOut), "SG-Snell") {
		t.Errorf("Clash output leaked snell proxy name:\n%s", string(clashOut))
	}
	if strings.Contains(string(clashOut), "GRP_SG") {
		t.Errorf("Clash output still references dropped GRP_SG:\n%s", string(clashOut))
	}
	if !strings.Contains(string(clashOut), "GRP_HK") {
		t.Errorf("Clash output missing surviving GRP_HK:\n%s", string(clashOut))
	}
}

// T-TGT-RND-003: VLESS nodes visible in Clash, cascade-filtered from Surge
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
		t.Error("VLESS node should appear in Clash output")
	}
	if !strings.Contains(string(clashOut), "type: vless") {
		t.Error("VLESS type should appear in Clash output")
	}

	surgeOut := mustRenderSurgeProjected(t, p)
	if strings.Contains(string(surgeOut), "HK-VL") {
		t.Error("VLESS node should NOT appear in Surge output")
	}
	if strings.Contains(string(surgeOut), "= vless") {
		t.Error("VLESS type should NOT appear in Surge output")
	}
	if !strings.Contains(string(surgeOut), "HK-01") {
		t.Error("SS node must still render in Surge")
	}
}

// T-TGT-RND-004: AnyTLS is supported by both target projections, but each
// renderer emits only its target-client field contract.
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
			t.Errorf("Clash output missing %q:\n%s", want, clashOut)
		}
	}

	surgeOut := string(mustRenderSurgeProjected(t, p))
	for _, want := range []string{"HK-AnyTLS = anytls", "password=pw", "sni=edge.example.com", "skip-cert-verify=true", "server-cert-fingerprint-sha256=abc123"} {
		if !strings.Contains(surgeOut, want) {
			t.Errorf("Surge output missing %q:\n%s", want, surgeOut)
		}
	}
	for _, unwanted := range []string{"udp-relay=true", "tfo=true", "client-fingerprint=chrome", "tls=true"} {
		if strings.Contains(surgeOut, unwanted) {
			t.Errorf("Surge output should not contain %q:\n%s", unwanted, surgeOut)
		}
	}
}
