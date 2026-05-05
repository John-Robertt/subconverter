package target

import (
	"errors"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

func buildSnellFilterPipeline() *model.Pipeline {
	return &model.Pipeline{
		Proxies: []model.Proxy{
			{Name: "HK-01", Type: "ss", Server: "hk.example.com", Port: 8388, Kind: model.KindSubscription},
			{Name: "HK-Snell", Type: "snell", Server: "1.2.3.4", Port: 57891, Params: map[string]string{"psk": "x", "version": "4"}, Kind: model.KindSnell},
			{Name: "SG-Snell", Type: "snell", Server: "5.6.7.8", Port: 8989, Params: map[string]string{"psk": "y", "version": "4"}, Kind: model.KindSnell},
		},
		NodeGroups: []model.ProxyGroup{
			{Name: "GRP_HK", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-01", "HK-Snell"}},
			{Name: "GRP_SG", Scope: model.ScopeNode, Strategy: "select", Members: []string{"SG-Snell"}},
		},
		RouteGroups: []model.ProxyGroup{
			{Name: "SVC_Quick", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_HK", "GRP_SG"}},
			{Name: "SVC_SGOnly", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_SG"}},
			{Name: "SVC_FINAL", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_HK", "SVC_SGOnly", "DIRECT"}},
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
		AllProxies: []string{"HK-01", "HK-Snell", "SG-Snell"},
	}
}

func buildVLessFilterPipeline() *model.Pipeline {
	return &model.Pipeline{
		Proxies: []model.Proxy{
			{Name: "HK-01", Type: "ss", Server: "hk.example.com", Port: 8388, Params: map[string]string{"cipher": "aes-256-gcm", "password": "pw"}, Kind: model.KindSubscription},
			{Name: "HK-VL", Type: "vless", Server: "1.2.3.4", Port: 443, Params: map[string]string{"uuid": "11111111-2222-3333-4444-555555555555", "security": "tls", "network": "tcp"}, Kind: model.KindVLess},
			{Name: "SG-VL", Type: "vless", Server: "5.6.7.8", Port: 443, Params: map[string]string{"uuid": "11111111-2222-3333-4444-555555555555", "security": "tls", "network": "tcp"}, Kind: model.KindVLess},
		},
		NodeGroups: []model.ProxyGroup{
			{Name: "GRP_HK", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-01", "HK-VL"}},
			{Name: "GRP_SG", Scope: model.ScopeNode, Strategy: "select", Members: []string{"SG-VL"}},
		},
		RouteGroups: []model.ProxyGroup{
			{Name: "SVC_Quick", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_HK", "GRP_SG"}},
			{Name: "SVC_SGOnly", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_SG"}},
			{Name: "SVC_FINAL", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_HK", "SVC_SGOnly", "DIRECT"}},
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
		AllProxies: []string{"HK-01", "HK-VL", "SG-VL"},
	}
}

func proxyNameSet(proxies []model.Proxy) map[string]bool {
	out := make(map[string]bool, len(proxies))
	for _, px := range proxies {
		out[px.Name] = true
	}
	return out
}

func groupNameSet(groups []model.ProxyGroup) map[string]bool {
	out := make(map[string]bool, len(groups))
	for _, g := range groups {
		out[g.Name] = true
	}
	return out
}

func rulesetPolicySet(rulesets []model.Ruleset) map[string]bool {
	out := make(map[string]bool, len(rulesets))
	for _, rs := range rulesets {
		out[rs.Policy] = true
	}
	return out
}

func rulePolicySet(rules []model.Rule) map[string]bool {
	out := make(map[string]bool, len(rules))
	for _, r := range rules {
		out[r.Policy] = true
	}
	return out
}

// T-TGT-001: Clash drops snell proxies and cascades to groups
func TestForClash_DropsSnellCascade(t *testing.T) {
	p := buildSnellFilterPipeline()

	projected, err := ForClash(p)
	if err != nil {
		t.Fatalf("ForClash() error: %v", err)
	}

	proxies := proxyNameSet(projected.Proxies)
	if proxies["HK-Snell"] || proxies["SG-Snell"] {
		t.Fatalf("snell proxies should be dropped, got %v", projected.Proxies)
	}
	if !proxies["HK-01"] {
		t.Fatalf("HK-01 should survive, got %v", projected.Proxies)
	}

	nodeGroups := groupNameSet(projected.NodeGroups)
	if nodeGroups["GRP_SG"] {
		t.Fatalf("GRP_SG should be dropped, got %v", projected.NodeGroups)
	}
	if !nodeGroups["GRP_HK"] {
		t.Fatalf("GRP_HK should survive, got %v", projected.NodeGroups)
	}

	routeGroups := groupNameSet(projected.RouteGroups)
	if routeGroups["SVC_SGOnly"] {
		t.Fatalf("SVC_SGOnly should be dropped, got %v", projected.RouteGroups)
	}
	if !routeGroups["SVC_Quick"] || !routeGroups["SVC_FINAL"] {
		t.Fatalf("surviving route groups missing, got %v", projected.RouteGroups)
	}

	rulesets := rulesetPolicySet(projected.Rulesets)
	if rulesets["SVC_SGOnly"] {
		t.Fatalf("ruleset for dropped policy should be removed, got %v", projected.Rulesets)
	}
	if !rulesets["SVC_Quick"] {
		t.Fatalf("ruleset for surviving policy missing, got %v", projected.Rulesets)
	}

	rules := rulePolicySet(projected.Rules)
	if rules["SVC_SGOnly"] {
		t.Fatalf("rule for dropped policy should be removed, got %v", projected.Rules)
	}
	if !rules["SVC_Quick"] {
		t.Fatalf("rule for surviving policy missing, got %v", projected.Rules)
	}

	for _, name := range projected.AllProxies {
		if name == "HK-Snell" || name == "SG-Snell" {
			t.Fatalf("filtered snell proxy leaked into AllProxies: %v", projected.AllProxies)
		}
	}
}

// T-TGT-002: Clash drops chained proxy when its dialer is dropped
func TestForClash_DropsChainedOnDroppedUpstream(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			{Name: "HK-Snell", Type: "snell", Server: "1.2.3.4", Port: 57891, Params: map[string]string{"psk": "x", "version": "4"}, Kind: model.KindSnell},
			{Name: "HK-01", Type: "ss", Server: "hk.example.com", Port: 8388, Kind: model.KindSubscription},
			{Name: "HK-Snell→MY-PROXY", Type: "socks5", Server: "1.1.1.1", Port: 1080, Kind: model.KindChained, Dialer: "HK-Snell"},
			{Name: "HK-01→MY-PROXY", Type: "socks5", Server: "1.1.1.1", Port: 1080, Kind: model.KindChained, Dialer: "HK-01"},
		},
		NodeGroups: []model.ProxyGroup{
			{Name: "GRP_HK", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-01"}},
			{Name: "GRP_CHAIN", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-Snell→MY-PROXY", "HK-01→MY-PROXY"}},
		},
		RouteGroups: []model.ProxyGroup{
			{Name: "SVC_FINAL", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_HK", "GRP_CHAIN", "DIRECT"}},
		},
		Fallback:   "SVC_FINAL",
		AllProxies: []string{"HK-Snell", "HK-01"},
	}

	projected, err := ForClash(p)
	if err != nil {
		t.Fatalf("ForClash() error: %v", err)
	}

	proxies := proxyNameSet(projected.Proxies)
	if proxies["HK-Snell→MY-PROXY"] {
		t.Fatalf("chained proxy with snell upstream should be dropped, got %v", projected.Proxies)
	}
	if !proxies["HK-01→MY-PROXY"] {
		t.Fatalf("chained proxy with ss upstream should survive, got %v", projected.Proxies)
	}
}

// T-TGT-003: Clash returns TargetError when fallback group cleared by cascade
func TestForClash_FallbackCleared(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			{Name: "HK-Snell", Type: "snell", Server: "1.2.3.4", Port: 57891, Params: map[string]string{"psk": "x", "version": "4"}, Kind: model.KindSnell},
		},
		NodeGroups: []model.ProxyGroup{
			{Name: "GRP_SG", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-Snell"}},
		},
		RouteGroups: []model.ProxyGroup{
			{Name: "SVC_FINAL", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_SG"}},
		},
		Fallback:   "SVC_FINAL",
		AllProxies: []string{"HK-Snell"},
	}

	_, err := ForClash(p)
	if err == nil {
		t.Fatal("expected TargetError when fallback group is cleared")
	}
	var te *errtype.TargetError
	if !errors.As(err, &te) {
		t.Fatalf("err type = %T, want *errtype.TargetError", err)
	}
	if te.Code != errtype.CodeTargetClashFallbackEmpty {
		t.Fatalf("Code = %q, want %q", te.Code, errtype.CodeTargetClashFallbackEmpty)
	}
	for _, keyword := range []string{"SVC_FINAL", "GRP_SG", "HK-Snell(snell)"} {
		if !strings.Contains(te.Error(), keyword) {
			t.Fatalf("error message missing %q: %s", keyword, te.Error())
		}
	}
}

// T-TGT-004: Clash fallback error path labels chained proxies in cascade trace
func TestForClash_FallbackPathLabelsChainedProxy(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			{Name: "HK-Snell", Type: "snell", Server: "1.2.3.4", Port: 57891, Params: map[string]string{"psk": "x", "version": "4"}, Kind: model.KindSnell},
			{Name: "HK-Snell→MY-PROXY", Type: "socks5", Server: "10.0.0.1", Port: 1080, Kind: model.KindChained, Dialer: "HK-Snell"},
		},
		NodeGroups: []model.ProxyGroup{
			{Name: "GRP_CHAIN", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-Snell→MY-PROXY"}},
		},
		RouteGroups: []model.ProxyGroup{
			{Name: "SVC_FINAL", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_CHAIN"}},
		},
		Fallback: "SVC_FINAL",
	}

	_, err := ForClash(p)
	if err == nil {
		t.Fatal("expected TargetError when fallback group is cleared")
	}
	var te *errtype.TargetError
	if !errors.As(err, &te) {
		t.Fatalf("err type = %T, want *errtype.TargetError", err)
	}
	if !strings.Contains(te.Error(), "HK-Snell→MY-PROXY(chained)") {
		t.Fatalf("cascade path should label chained proxy explicitly: %s", te.Error())
	}
	if !strings.Contains(te.Error(), "HK-Snell(snell)") {
		t.Fatalf("cascade path should include upstream snell root: %s", te.Error())
	}
}

// T-TGT-005: Clash shared subgraph does not misreport as cycle
func TestForClash_SharedSubgraphDoesNotReportCycle(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			{Name: "HK-Snell", Type: "snell", Server: "1.2.3.4", Port: 57891, Params: map[string]string{"psk": "x", "version": "4"}, Kind: model.KindSnell},
		},
		NodeGroups: []model.ProxyGroup{
			{Name: "GRP_SHARED", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-Snell"}},
		},
		RouteGroups: []model.ProxyGroup{
			{Name: "SVC_A", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_SHARED"}},
			{Name: "SVC_B", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_SHARED"}},
			{Name: "SVC_FINAL", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"SVC_A", "SVC_B"}},
		},
		Fallback: "SVC_FINAL",
	}

	_, err := ForClash(p)
	if err == nil {
		t.Fatal("expected TargetError when fallback group is cleared")
	}
	if strings.Contains(err.Error(), "(cycle)") {
		t.Fatalf("shared drop subgraph should not be labeled as cycle: %s", err.Error())
	}
}

// 当父服务组在 RouteGroups slice 中声明得比子服务组更早（非拓扑序）时，
// 单次遍历会漏掉父组——必须通过不动点循环在第 2 轮捕获 "SVC_PARENT 成员仅剩 SVC_CHILD，
// 而 SVC_CHILD 刚被清空"。若未来有人把 buildGroupCascade 的 for 循环改成单 pass
// （误以为 "一次遍历所有组足够"），此测试会立即失败。
// T-TGT-006: Clash cascade handles non-topological group declaration order
func TestForClash_CascadeHandlesNonTopologicalDeclaration(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			{Name: "HK-Snell", Type: "snell", Server: "1.2.3.4", Port: 57891, Params: map[string]string{"psk": "x", "version": "4"}, Kind: model.KindSnell},
		},
		NodeGroups: []model.ProxyGroup{
			{Name: "GRP_SNELL", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-Snell"}},
		},
		// 故意把 SVC_PARENT 放在 SVC_CHILD 之前声明。
		RouteGroups: []model.ProxyGroup{
			{Name: "SVC_PARENT", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"SVC_CHILD"}},
			{Name: "SVC_CHILD", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_SNELL"}},
		},
		Fallback:   "SVC_PARENT",
		AllProxies: []string{"HK-Snell"},
	}

	_, err := ForClash(p)
	if err == nil {
		t.Fatal("expected TargetError when fallback cascade clears parent")
	}
	var te *errtype.TargetError
	if !errors.As(err, &te) {
		t.Fatalf("err type = %T, want *errtype.TargetError", err)
	}
	if te.Code != errtype.CodeTargetClashFallbackEmpty {
		t.Fatalf("Code = %q, want %q", te.Code, errtype.CodeTargetClashFallbackEmpty)
	}
	// 清空路径应贯穿 SVC_PARENT ← SVC_CHILD ← GRP_SNELL ← HK-Snell(snell)，
	// 证明 2 轮不动点遍历已将父组一并清空。
	for _, keyword := range []string{"SVC_PARENT", "SVC_CHILD", "GRP_SNELL", "HK-Snell(snell)"} {
		if !strings.Contains(te.Error(), keyword) {
			t.Fatalf("error message missing %q: %s", keyword, te.Error())
		}
	}
}

// T-TGT-007: Clash drops route group when all members are snell
func TestForClash_DropsAllSnellRouteGroup(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			{Name: "HK-Snell", Type: "snell", Server: "1.2.3.4", Port: 57891, Params: map[string]string{"psk": "x", "version": "4"}, Kind: model.KindSnell},
			{Name: "SG-Snell", Type: "snell", Server: "5.6.7.8", Port: 8989, Params: map[string]string{"psk": "y", "version": "4"}, Kind: model.KindSnell},
			{Name: "HK-01", Type: "ss", Server: "hk.example.com", Port: 8388, Kind: model.KindSubscription},
		},
		NodeGroups: []model.ProxyGroup{
			{Name: "GRP_HK", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-01"}},
		},
		RouteGroups: []model.ProxyGroup{
			{Name: "SVC_ManualAll", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"HK-Snell", "SG-Snell"}},
			{Name: "SVC_FINAL", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_HK", "SVC_ManualAll", "DIRECT"}},
		},
		Fallback:   "SVC_FINAL",
		AllProxies: []string{"HK-Snell", "SG-Snell", "HK-01"},
	}

	projected, err := ForClash(p)
	if err != nil {
		t.Fatalf("ForClash() error: %v", err)
	}
	if groupNameSet(projected.RouteGroups)["SVC_ManualAll"] {
		t.Fatalf("all-snell route group should be dropped, got %v", projected.RouteGroups)
	}
}

// T-TGT-008: Clash is no-op when pipeline contains no snell
func TestForClash_NoOpWhenNoSnell(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			{Name: "HK-01", Type: "ss", Server: "hk.example.com", Port: 8388, Kind: model.KindSubscription},
		},
		NodeGroups: []model.ProxyGroup{
			{Name: "GRP_HK", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-01"}},
		},
		RouteGroups: []model.ProxyGroup{
			{Name: "SVC_FINAL", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_HK"}},
		},
		Fallback: "SVC_FINAL",
	}

	out, err := ForClash(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != p {
		t.Fatal("expected identical pointer when no filtering is needed")
	}
}

// T-TGT-009: Surge drops vless proxies and cascades to groups
func TestForSurge_DropsVLessCascade(t *testing.T) {
	p := buildVLessFilterPipeline()

	projected, err := ForSurge(p)
	if err != nil {
		t.Fatalf("ForSurge() error: %v", err)
	}

	proxies := proxyNameSet(projected.Proxies)
	if proxies["HK-VL"] || proxies["SG-VL"] {
		t.Fatalf("vless proxies should be dropped, got %v", projected.Proxies)
	}
	if !proxies["HK-01"] {
		t.Fatalf("HK-01 should survive, got %v", projected.Proxies)
	}

	nodeGroups := groupNameSet(projected.NodeGroups)
	if nodeGroups["GRP_SG"] {
		t.Fatalf("GRP_SG should be dropped, got %v", projected.NodeGroups)
	}
	if !nodeGroups["GRP_HK"] {
		t.Fatalf("GRP_HK should survive, got %v", projected.NodeGroups)
	}

	routeGroups := groupNameSet(projected.RouteGroups)
	if routeGroups["SVC_SGOnly"] {
		t.Fatalf("SVC_SGOnly should be dropped, got %v", projected.RouteGroups)
	}
	if !routeGroups["SVC_Quick"] || !routeGroups["SVC_FINAL"] {
		t.Fatalf("surviving route groups missing, got %v", projected.RouteGroups)
	}

	rulesets := rulesetPolicySet(projected.Rulesets)
	if rulesets["SVC_SGOnly"] {
		t.Fatalf("ruleset for dropped policy should be removed, got %v", projected.Rulesets)
	}
	if !rulesets["SVC_Quick"] {
		t.Fatalf("ruleset for surviving policy missing, got %v", projected.Rulesets)
	}

	rules := rulePolicySet(projected.Rules)
	if rules["SVC_SGOnly"] {
		t.Fatalf("rule for dropped policy should be removed, got %v", projected.Rules)
	}
	if !rules["SVC_Quick"] {
		t.Fatalf("rule for surviving policy missing, got %v", projected.Rules)
	}
}

// T-TGT-010: Surge drops chained proxy when its dialer is dropped
func TestForSurge_DropsChainedOnDroppedUpstream(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			{Name: "HK-01", Type: "ss", Server: "hk.example.com", Port: 8388, Params: map[string]string{"cipher": "aes-256-gcm", "password": "pw"}, Kind: model.KindSubscription},
			{Name: "HK-VL", Type: "vless", Server: "1.2.3.4", Port: 443, Params: map[string]string{"uuid": "11111111-2222-3333-4444-555555555555", "security": "tls", "network": "tcp"}, Kind: model.KindVLess},
			{Name: "HK-VL→CHAIN", Type: "socks5", Server: "127.0.0.1", Port: 1080, Kind: model.KindChained, Dialer: "HK-VL"},
		},
		NodeGroups: []model.ProxyGroup{
			{Name: "GRP_HK", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-01"}},
			{Name: "GRP_CHAIN", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-VL→CHAIN"}},
		},
		RouteGroups: []model.ProxyGroup{
			{Name: "SVC_FINAL", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_HK", "GRP_CHAIN", "DIRECT"}},
		},
		Fallback:   "SVC_FINAL",
		AllProxies: []string{"HK-01", "HK-VL"},
	}

	projected, err := ForSurge(p)
	if err != nil {
		t.Fatalf("ForSurge() error: %v", err)
	}

	proxies := proxyNameSet(projected.Proxies)
	if proxies["HK-VL→CHAIN"] {
		t.Fatalf("chained proxy with dropped VLESS upstream should be dropped, got %v", projected.Proxies)
	}
	if groupNameSet(projected.NodeGroups)["GRP_CHAIN"] {
		t.Fatalf("GRP_CHAIN should be dropped after its only member was removed, got %v", projected.NodeGroups)
	}
}

// T-TGT-011: Surge returns TargetError when fallback group cleared by cascade
func TestForSurge_FallbackCleared(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			{Name: "HK-VL", Type: "vless", Server: "1.2.3.4", Port: 443, Params: map[string]string{"uuid": "11111111-2222-3333-4444-555555555555", "security": "tls", "network": "tcp"}, Kind: model.KindVLess},
		},
		NodeGroups: []model.ProxyGroup{
			{Name: "GRP_HK", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-VL"}},
		},
		RouteGroups: []model.ProxyGroup{
			{Name: "SVC_FINAL", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_HK"}},
		},
		Fallback:   "SVC_FINAL",
		AllProxies: []string{"HK-VL"},
	}

	_, err := ForSurge(p)
	if err == nil {
		t.Fatal("expected Surge fallback-empty error")
	}
	var te *errtype.TargetError
	if !errors.As(err, &te) {
		t.Fatalf("err type = %T, want *TargetError", err)
	}
	if te.Code != errtype.CodeTargetSurgeFallbackEmpty {
		t.Fatalf("Code = %q, want %q", te.Code, errtype.CodeTargetSurgeFallbackEmpty)
	}
	if te.Format != "surge" {
		t.Fatalf("Format = %q, want surge", te.Format)
	}
	for _, want := range []string{"SVC_FINAL", "GRP_HK", "HK-VL(vless)", "被 vless 过滤级联清空"} {
		if !strings.Contains(te.Message, want) {
			t.Fatalf("message missing %q, got: %s", want, te.Message)
		}
	}
}

// T-TGT-012: Surge shared subgraph does not misreport as cycle
func TestForSurge_SharedSubgraphNoCycleMisreport(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			{Name: "VL-SRC", Type: "vless", Server: "1.2.3.4", Port: 443, Params: map[string]string{"uuid": "11111111-2222-3333-4444-555555555555", "security": "tls", "network": "tcp"}, Kind: model.KindVLess},
			{Name: "VL-SRC→A", Type: "socks5", Server: "127.0.0.1", Port: 1080, Kind: model.KindChained, Dialer: "VL-SRC"},
			{Name: "VL-SRC→B", Type: "socks5", Server: "127.0.0.1", Port: 1081, Kind: model.KindChained, Dialer: "VL-SRC"},
		},
		NodeGroups: []model.ProxyGroup{
			{Name: "GRP_CHAIN", Scope: model.ScopeNode, Strategy: "select", Members: []string{"VL-SRC→A", "VL-SRC→B"}},
		},
		RouteGroups: []model.ProxyGroup{
			{Name: "SVC_FINAL", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_CHAIN"}},
		},
		Fallback:   "SVC_FINAL",
		AllProxies: []string{"VL-SRC"},
	}

	_, err := ForSurge(p)
	if err == nil {
		t.Fatal("expected fallback-empty error")
	}
	if strings.Contains(err.Error(), "cycle") {
		t.Fatalf("shared upstream should not trigger cycle report, got: %s", err.Error())
	}
}

// T-TGT-013: Surge is no-op when pipeline contains no vless
func TestForSurge_NoOpWhenNoVLess(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			{Name: "HK-01", Type: "ss", Server: "hk.example.com", Port: 8388, Params: map[string]string{"cipher": "aes-256-gcm", "password": "pw"}, Kind: model.KindSubscription},
		},
		NodeGroups: []model.ProxyGroup{
			{Name: "GRP_HK", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-01"}},
		},
		RouteGroups: []model.ProxyGroup{
			{Name: "SVC_FINAL", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_HK", "DIRECT"}},
		},
		Fallback:   "SVC_FINAL",
		AllProxies: []string{"HK-01"},
	}

	filtered, err := ForSurge(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filtered != p {
		t.Fatal("expected pointer identity passthrough when no VLESS is present")
	}
}

// T-TGT-014: internal filter error uses CodeTargetClashProjectionInvalid
func TestInternalFilterError_UsesDedicatedProjectionCode(t *testing.T) {
	tests := []struct {
		name string
		opts cascadeOptions
		want errtype.Code
	}{
		{
			name: "clash",
			opts: cascadeOptions{formatName: "clash", internalCode: errtype.CodeTargetClashProjectionInvalid},
			want: errtype.CodeTargetClashProjectionInvalid,
		},
		{
			name: "surge",
			opts: cascadeOptions{formatName: "surge", internalCode: errtype.CodeTargetSurgeProjectionInvalid},
			want: errtype.CodeTargetSurgeProjectionInvalid,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := internalFilterError(tc.opts, "boom")
			var te *errtype.TargetError
			if !errors.As(err, &te) {
				t.Fatalf("err type = %T, want *errtype.TargetError", err)
			}
			if te.Code != tc.want {
				t.Fatalf("Code = %q, want %q", te.Code, tc.want)
			}
		})
	}
}
