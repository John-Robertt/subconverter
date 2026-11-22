package target

import (
	"errors"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

func ssProxy(id model.ProxyID, name string) model.Proxy {
	return model.Proxy{
		ID:     id,
		Name:   name,
		Type:   "ss",
		Server: "proxy.example.com",
		Port:   8388,
		Params: map[string]string{"cipher": "aes-256-gcm", "password": "pw"},
		Kind:   model.KindSubscription,
	}
}

func snellProxy(id model.ProxyID, name string) model.Proxy {
	return model.Proxy{
		ID:     id,
		Name:   name,
		Type:   "snell",
		Server: "proxy.example.com",
		Port:   57891,
		Params: map[string]string{"psk": "pw", "version": "4"},
		Kind:   model.KindSnell,
	}
}

func vlessProxy(id model.ProxyID, name string) model.Proxy {
	return model.Proxy{
		ID:     id,
		Name:   name,
		Type:   "vless",
		Server: "proxy.example.com",
		Port:   443,
		Params: map[string]string{
			"uuid":     "11111111-2222-3333-4444-555555555555",
			"security": "tls",
			"network":  "tcp",
		},
		Kind: model.KindVLess,
	}
}

func nativeProxy(id model.ProxyID, name string, format model.NativeProxyFormat) model.Proxy {
	payload := "name: " + name + "\ntype: trojan\n"
	if format == model.NativeProxyFormatSurge {
		payload = "trojan, proxy.example.com, 443, password=pw"
	}
	return model.Proxy{
		ID:   id,
		Name: name,
		Type: "trojan",
		Kind: model.KindSubscription,
		Native: &model.NativeProxy{
			Format:  format,
			Payload: payload,
		},
	}
}

func chainedProxy(id model.ProxyID, name string, dialerID model.ProxyID, suffix string) model.Proxy {
	return model.Proxy{
		ID:              id,
		Name:            name,
		Type:            "socks5",
		Server:          "127.0.0.1",
		Port:            1080,
		Kind:            model.KindChained,
		DialerID:        dialerID,
		ChainNameSuffix: suffix,
	}
}

func nodeGroup(name string, members ...model.ProxyID) model.NodeGroup {
	return model.NodeGroup{Name: name, Strategy: "select", Members: members}
}

func proxyPolicy(id model.ProxyID) model.PolicyRef {
	return model.PolicyRef{Kind: model.PolicyRefProxy, ProxyID: id}
}

func nodePolicy(name string) model.PolicyRef {
	return model.PolicyRef{Kind: model.PolicyRefNodeGroup, Name: name}
}

func routePolicy(name string) model.PolicyRef {
	return model.PolicyRef{Kind: model.PolicyRefRouteGroup, Name: name}
}

func reservedPolicy(name string) model.PolicyRef {
	return model.PolicyRef{Kind: model.PolicyRefReserved, Name: name}
}

func routeGroup(name string, members ...model.PolicyRef) model.RouteGroup {
	return model.RouteGroup{Name: name, Strategy: "select", Members: members}
}

func targetProxyNameSet(proxies []model.TargetProxy) map[string]bool {
	result := make(map[string]bool, len(proxies))
	for _, proxy := range proxies {
		result[proxy.Name] = true
	}
	return result
}

func targetGroupNameSet(groups []model.TargetGroup) map[string]bool {
	result := make(map[string]bool, len(groups))
	for _, group := range groups {
		result[group.Name] = true
	}
	return result
}

func targetGroupByName(groups []model.TargetGroup, name string) *model.TargetGroup {
	for index := range groups {
		if groups[index].Name == name {
			return &groups[index]
		}
	}
	return nil
}

func targetProxyByName(proxies []model.TargetProxy, name string) *model.TargetProxy {
	for index := range proxies {
		if proxies[index].Name == name {
			return &proxies[index]
		}
	}
	return nil
}

func buildSnellFilterPipeline() *model.Pipeline {
	return &model.Pipeline{
		Proxies: []model.Proxy{
			ssProxy(1, "HK-01"),
			snellProxy(2, "HK-Snell"),
			snellProxy(3, "SG-Snell"),
		},
		NodeGroups: []model.NodeGroup{
			nodeGroup("GRP_HK", 1, 2),
			nodeGroup("GRP_SG", 3),
		},
		RouteGroups: []model.RouteGroup{
			routeGroup("SVC_Quick", nodePolicy("GRP_HK"), nodePolicy("GRP_SG")),
			routeGroup("SVC_SGOnly", nodePolicy("GRP_SG")),
			routeGroup("SVC_FINAL", nodePolicy("GRP_HK"), routePolicy("SVC_SGOnly"), reservedPolicy("DIRECT")),
		},
		Rulesets: []model.Ruleset{
			{Policy: "SVC_SGOnly", URLs: []string{"https://example.com/sg.list"}},
			{Policy: "SVC_Quick", URLs: []string{"https://example.com/global.list"}},
		},
		Rules: []model.Rule{
			{Raw: "GEOIP,SG,SVC_SGOnly", Policy: "SVC_SGOnly"},
			{Raw: "GEOIP,CN,SVC_Quick", Policy: "SVC_Quick"},
		},
		Fallback:   "SVC_FINAL",
		AllProxies: []model.ProxyID{1, 2, 3},
	}
}

func buildVLessFilterPipeline() *model.Pipeline {
	return &model.Pipeline{
		Proxies: []model.Proxy{
			ssProxy(1, "HK-01"),
			vlessProxy(2, "HK-VL"),
			vlessProxy(3, "SG-VL"),
		},
		NodeGroups: []model.NodeGroup{
			nodeGroup("GRP_HK", 1, 2),
			nodeGroup("GRP_SG", 3),
		},
		RouteGroups: []model.RouteGroup{
			routeGroup("SVC_Quick", nodePolicy("GRP_HK"), nodePolicy("GRP_SG")),
			routeGroup("SVC_SGOnly", nodePolicy("GRP_SG")),
			routeGroup("SVC_FINAL", nodePolicy("GRP_HK"), routePolicy("SVC_SGOnly"), reservedPolicy("DIRECT")),
		},
		Rulesets: []model.Ruleset{
			{Policy: "SVC_SGOnly", URLs: []string{"https://example.com/sg.list"}},
			{Policy: "SVC_Quick", URLs: []string{"https://example.com/global.list"}},
		},
		Rules: []model.Rule{
			{Raw: "GEOIP,SG,SVC_SGOnly", Policy: "SVC_SGOnly"},
			{Raw: "GEOIP,CN,SVC_Quick", Policy: "SVC_Quick"},
		},
		Fallback:   "SVC_FINAL",
		AllProxies: []model.ProxyID{1, 2, 3},
	}
}

// T-TGT-001：Clash 剔除 Snell 节点，以及所有因此变空的依赖项。
func TestForClash_DropsSnellCascade(t *testing.T) {
	projected, err := ForClash(buildSnellFilterPipeline())
	if err != nil {
		t.Fatalf("ForClash: %v", err)
	}

	proxies := targetProxyNameSet(projected.Proxies)
	if proxies["HK-Snell"] || proxies["SG-Snell"] || !proxies["HK-01"] {
		t.Fatalf("Clash 节点 = %v，期望只有 HK-01", proxies)
	}
	nodeGroups := targetGroupNameSet(projected.NodeGroups)
	if nodeGroups["GRP_SG"] || !nodeGroups["GRP_HK"] {
		t.Fatalf("Clash 节点组 = %v", nodeGroups)
	}
	routeGroups := targetGroupNameSet(projected.RouteGroups)
	if routeGroups["SVC_SGOnly"] || !routeGroups["SVC_Quick"] || !routeGroups["SVC_FINAL"] {
		t.Fatalf("Clash 服务组 = %v", routeGroups)
	}
	if len(projected.Rulesets) != 1 || projected.Rulesets[0].Policy != "SVC_Quick" {
		t.Fatalf("Clash 规则集 = %+v", projected.Rulesets)
	}
	if len(projected.Rules) != 1 || projected.Rules[0].Policy != "SVC_Quick" {
		t.Fatalf("Clash 规则 = %+v", projected.Rules)
	}
}

// T-TGT-002：链式节点的去留跟随其上游（dialer）节点。
func TestForClash_DropsChainedOnDroppedUpstream(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			snellProxy(1, "HK-Snell"),
			ssProxy(2, "HK-01"),
			chainedProxy(3, "HK-Snell→MY-PROXY", 1, "MY-PROXY"),
			chainedProxy(4, "HK-01→MY-PROXY", 2, "MY-PROXY"),
		},
		NodeGroups: []model.NodeGroup{
			nodeGroup("GRP_HK", 2),
			nodeGroup("GRP_CHAIN", 3, 4),
		},
		RouteGroups: []model.RouteGroup{
			routeGroup("SVC_FINAL", nodePolicy("GRP_HK"), nodePolicy("GRP_CHAIN"), reservedPolicy("DIRECT")),
		},
		Fallback:   "SVC_FINAL",
		AllProxies: []model.ProxyID{1, 2},
	}

	projected, err := ForClash(p)
	if err != nil {
		t.Fatalf("ForClash: %v", err)
	}
	proxies := targetProxyNameSet(projected.Proxies)
	if proxies["HK-Snell→MY-PROXY"] || !proxies["HK-01→MY-PROXY"] {
		t.Fatalf("投影后的节点 = %v", proxies)
	}
}

// T-TGT-003：fallback 被清空时，报告带节点编号的完整清空路径。
func TestForClash_FallbackCleared(t *testing.T) {
	p := &model.Pipeline{
		Proxies:    []model.Proxy{snellProxy(1, "HK-Snell")},
		NodeGroups: []model.NodeGroup{nodeGroup("GRP_SG", 1)},
		RouteGroups: []model.RouteGroup{
			routeGroup("SVC_FINAL", nodePolicy("GRP_SG")),
		},
		Fallback:   "SVC_FINAL",
		AllProxies: []model.ProxyID{1},
	}

	_, err := ForClash(p)
	var targetErr *errtype.TargetError
	if !errors.As(err, &targetErr) {
		t.Fatalf("error = %T %v，期望 TargetError", err, err)
	}
	if targetErr.Code != errtype.CodeTargetClashFallbackEmpty {
		t.Fatalf("Code = %q", targetErr.Code)
	}
	for _, want := range []string{"SVC_FINAL", "GRP_SG", "HK-Snell#1(snell)"} {
		if !strings.Contains(targetErr.Error(), want) {
			t.Fatalf("错误信息缺少 %q：%s", want, targetErr.Error())
		}
	}
}

// T-TGT-004：链式剔除路径同时保留下游节点和根因节点的标识。
func TestForClash_FallbackPathLabelsChainedProxy(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			snellProxy(1, "HK-Snell"),
			chainedProxy(2, "HK-Snell→CHAIN", 1, "CHAIN"),
		},
		NodeGroups: []model.NodeGroup{nodeGroup("GRP_CHAIN", 2)},
		RouteGroups: []model.RouteGroup{
			routeGroup("SVC_FINAL", nodePolicy("GRP_CHAIN")),
		},
		Fallback: "SVC_FINAL",
	}

	_, err := ForClash(p)
	for _, want := range []string{"HK-Snell→CHAIN#2(chained)", "HK-Snell#1(snell)"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("错误信息缺少 %q：%s", want, err)
		}
	}
}

// T-TGT-005：被共享的已剔除子图不算循环。
func TestForClash_SharedSubgraphDoesNotReportCycle(t *testing.T) {
	p := &model.Pipeline{
		Proxies:    []model.Proxy{snellProxy(1, "HK-Snell")},
		NodeGroups: []model.NodeGroup{nodeGroup("GRP_SHARED", 1)},
		RouteGroups: []model.RouteGroup{
			routeGroup("SVC_A", nodePolicy("GRP_SHARED")),
			routeGroup("SVC_B", nodePolicy("GRP_SHARED")),
			routeGroup("SVC_FINAL", routePolicy("SVC_A"), routePolicy("SVC_B")),
		},
		Fallback: "SVC_FINAL",
	}

	_, err := ForClash(p)
	if err == nil || strings.Contains(err.Error(), "(cycle)") {
		t.Fatalf("共享子图的错误 = %v", err)
	}
}

// T-TGT-006：即使服务组不是按拓扑顺序声明，级联剔除也能收敛。
func TestForClash_CascadeHandlesNonTopologicalDeclaration(t *testing.T) {
	p := &model.Pipeline{
		Proxies:    []model.Proxy{snellProxy(1, "HK-Snell")},
		NodeGroups: []model.NodeGroup{nodeGroup("GRP_SNELL", 1)},
		RouteGroups: []model.RouteGroup{
			routeGroup("SVC_PARENT", routePolicy("SVC_CHILD")),
			routeGroup("SVC_CHILD", nodePolicy("GRP_SNELL")),
		},
		Fallback: "SVC_PARENT",
	}

	_, err := ForClash(p)
	for _, want := range []string{"SVC_PARENT", "SVC_CHILD", "GRP_SNELL", "HK-Snell#1(snell)"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("错误信息缺少 %q：%s", want, err)
		}
	}
}

// T-TGT-007：对节点的直接引用按节点编号剔除。
func TestForClash_DropsAllSnellRouteGroup(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			snellProxy(1, "HK-Snell"),
			snellProxy(2, "SG-Snell"),
			ssProxy(3, "HK-01"),
		},
		NodeGroups: []model.NodeGroup{nodeGroup("GRP_HK", 3)},
		RouteGroups: []model.RouteGroup{
			routeGroup("SVC_ManualAll", proxyPolicy(1), proxyPolicy(2)),
			routeGroup("SVC_FINAL", nodePolicy("GRP_HK"), routePolicy("SVC_ManualAll"), reservedPolicy("DIRECT")),
		},
		Fallback:   "SVC_FINAL",
		AllProxies: []model.ProxyID{1, 2, 3},
	}

	projected, err := ForClash(p)
	if err != nil {
		t.Fatalf("ForClash: %v", err)
	}
	if targetGroupNameSet(projected.RouteGroups)["SVC_ManualAll"] {
		t.Fatalf("全部由 Snell 组成的服务组没有被剔除：%+v", projected.RouteGroups)
	}
}

// T-TGT-008：完全兼容的 pipeline 仍会被投影成渲染器使用的类型。
func TestForClash_CompatiblePipelineProjectsNames(t *testing.T) {
	p := &model.Pipeline{
		Proxies:    []model.Proxy{ssProxy(1, "HK-01")},
		NodeGroups: []model.NodeGroup{nodeGroup("GRP_HK", 1)},
		RouteGroups: []model.RouteGroup{
			routeGroup("SVC_FINAL", nodePolicy("GRP_HK")),
		},
		Fallback: "SVC_FINAL",
	}

	projected, err := ForClash(p)
	if err != nil {
		t.Fatalf("ForClash: %v", err)
	}
	if len(projected.Proxies) != 1 || projected.Proxies[0].Name != "HK-01" {
		t.Fatalf("投影后的节点 = %+v", projected.Proxies)
	}
	if got := targetGroupByName(projected.NodeGroups, "GRP_HK"); got == nil || len(got.Members) != 1 || got.Members[0] != "HK-01" {
		t.Fatalf("投影后的节点组 = %+v", projected.NodeGroups)
	}
}

// T-TGT-009：Surge 对 VLESS 执行对称的级联剔除。
func TestForSurge_DropsVLessCascade(t *testing.T) {
	projected, err := ForSurge(buildVLessFilterPipeline())
	if err != nil {
		t.Fatalf("ForSurge: %v", err)
	}
	proxies := targetProxyNameSet(projected.Proxies)
	if proxies["HK-VL"] || proxies["SG-VL"] || !proxies["HK-01"] {
		t.Fatalf("Surge 节点 = %v", proxies)
	}
	if targetGroupNameSet(projected.NodeGroups)["GRP_SG"] {
		t.Fatalf("Surge 节点组 = %+v", projected.NodeGroups)
	}
	if targetGroupNameSet(projected.RouteGroups)["SVC_SGOnly"] {
		t.Fatalf("Surge 服务组 = %+v", projected.RouteGroups)
	}
}

// T-TGT-010：Surge 剔除上游 VLESS 节点已被移除的链式节点。
func TestForSurge_DropsChainedOnDroppedUpstream(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			ssProxy(1, "HK-01"),
			vlessProxy(2, "HK-VL"),
			chainedProxy(3, "HK-VL→CHAIN", 2, "CHAIN"),
		},
		NodeGroups: []model.NodeGroup{
			nodeGroup("GRP_HK", 1),
			nodeGroup("GRP_CHAIN", 3),
		},
		RouteGroups: []model.RouteGroup{
			routeGroup("SVC_FINAL", nodePolicy("GRP_HK"), nodePolicy("GRP_CHAIN"), reservedPolicy("DIRECT")),
		},
		Fallback: "SVC_FINAL",
	}

	projected, err := ForSurge(p)
	if err != nil {
		t.Fatalf("ForSurge: %v", err)
	}
	if targetProxyNameSet(projected.Proxies)["HK-VL→CHAIN"] {
		t.Fatalf("VLESS 链式节点没有被剔除：%+v", projected.Proxies)
	}
	if targetGroupNameSet(projected.NodeGroups)["GRP_CHAIN"] {
		t.Fatalf("空的链式节点组没有被剔除：%+v", projected.NodeGroups)
	}
}

// T-TGT-011：Surge 使用自己专用的 fallback 错误码。
func TestForSurge_FallbackCleared(t *testing.T) {
	p := &model.Pipeline{
		Proxies:    []model.Proxy{vlessProxy(1, "HK-VL")},
		NodeGroups: []model.NodeGroup{nodeGroup("GRP_HK", 1)},
		RouteGroups: []model.RouteGroup{
			routeGroup("SVC_FINAL", nodePolicy("GRP_HK")),
		},
		Fallback: "SVC_FINAL",
	}

	_, err := ForSurge(p)
	var targetErr *errtype.TargetError
	if !errors.As(err, &targetErr) {
		t.Fatalf("error = %T %v", err, err)
	}
	if targetErr.Code != errtype.CodeTargetSurgeFallbackEmpty || targetErr.Format != "surge" {
		t.Fatalf("TargetError = %+v", targetErr)
	}
	for _, want := range []string{"SVC_FINAL", "GRP_HK", "HK-VL#1(vless)"} {
		if !strings.Contains(targetErr.Error(), want) {
			t.Fatalf("错误信息缺少 %q：%s", want, targetErr.Error())
		}
	}
}

// T-TGT-012：投影只保留目标格式自己的结构化订阅节点。
func TestTargetProjection_FiltersNativeSubscriptionFormats(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			nativeProxy(1, "HK-YAML", model.NativeProxyFormatClash),
			nativeProxy(2, "SG-CONF", model.NativeProxyFormatSurge),
			ssProxy(3, "US-TXT"),
		},
		NodeGroups: []model.NodeGroup{nodeGroup("GRP_ALL", 1, 2, 3)},
		RouteGroups: []model.RouteGroup{
			routeGroup("SVC_FINAL", nodePolicy("GRP_ALL"), reservedPolicy("DIRECT")),
		},
		Fallback:   "SVC_FINAL",
		AllProxies: []model.ProxyID{1, 2, 3},
	}

	clash, err := ForClash(p)
	if err != nil {
		t.Fatalf("ForClash: %v", err)
	}
	clashNames := targetProxyNameSet(clash.Proxies)
	if !clashNames["HK-YAML"] || !clashNames["US-TXT"] || clashNames["SG-CONF"] {
		t.Fatalf("Clash 名字 = %v", clashNames)
	}

	surge, err := ForSurge(p)
	if err != nil {
		t.Fatalf("ForSurge: %v", err)
	}
	surgeNames := targetProxyNameSet(surge.Proxies)
	if !surgeNames["SG-CONF"] || !surgeNames["US-TXT"] || surgeNames["HK-YAML"] {
		t.Fatalf("Surge 名字 = %v", surgeNames)
	}
}

// T-TGT-013：不兼容的原生根因节点在级联诊断信息中仍然可见。
func TestForClash_SurgeNativeCascadeIncludesRootCause(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			nativeProxy(1, "HK-CONF", model.NativeProxyFormatSurge),
			chainedProxy(2, "HK-CONF→CHAIN", 1, "CHAIN"),
		},
		NodeGroups: []model.NodeGroup{nodeGroup("GRP_CHAIN", 2)},
		RouteGroups: []model.RouteGroup{
			routeGroup("SVC_FINAL", nodePolicy("GRP_CHAIN")),
		},
		Fallback: "SVC_FINAL",
	}

	_, err := ForClash(p)
	for _, want := range []string{"HK-CONF→CHAIN#2(chained)", "HK-CONF#1(Surge CONF 专属)", "SVC_FINAL"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("错误信息缺少 %q：%s", want, err)
		}
	}
}

// T-TGT-ID-001：名字在按格式过滤之后，各格式独立分配。
func TestTargetProjection_DuplicateNativeNamesRemainOriginalPerFormat(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			nativeProxy(1, "SAME", model.NativeProxyFormatClash),
			nativeProxy(2, "SAME", model.NativeProxyFormatSurge),
		},
		NodeGroups: []model.NodeGroup{nodeGroup("GRP_ALL", 1, 2)},
		RouteGroups: []model.RouteGroup{
			routeGroup("SVC_FINAL", nodePolicy("GRP_ALL")),
		},
		Fallback: "SVC_FINAL",
	}

	clash, err := ForClash(p)
	if err != nil {
		t.Fatalf("ForClash: %v", err)
	}
	surge, err := ForSurge(p)
	if err != nil {
		t.Fatalf("ForSurge: %v", err)
	}
	if len(clash.Proxies) != 1 || clash.Proxies[0].Name != "SAME" {
		t.Fatalf("Clash 节点 = %+v，期望保持原名 SAME", clash.Proxies)
	}
	if len(surge.Proxies) != 1 || surge.Proxies[0].Name != "SAME" {
		t.Fatalf("Surge 节点 = %+v，期望保持原名 SAME", surge.Proxies)
	}
}

// T-TGT-ID-002：同一目标格式内的重名使用确定的后缀，原本就带这类后缀的
// 名字保持不变。
func TestTargetProjection_DeduplicatesWithinOneFormat(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			ssProxy(1, "NODE-A"),
			ssProxy(2, "NODE-A②"),
			ssProxy(3, "NODE-A"),
		},
		NodeGroups: []model.NodeGroup{nodeGroup("GRP_ALL", 1, 2, 3)},
		RouteGroups: []model.RouteGroup{
			routeGroup("SVC_FINAL", nodePolicy("GRP_ALL")),
		},
		Fallback: "SVC_FINAL",
	}

	projected, err := ForClash(p)
	if err != nil {
		t.Fatalf("ForClash: %v", err)
	}
	want := []string{"NODE-A", "NODE-A②", "NODE-A③"}
	for index, name := range want {
		if projected.Proxies[index].Name != name {
			t.Errorf("Proxies[%d].Name = %q，期望 %q", index, projected.Proxies[index].Name, name)
		}
	}
	group := targetGroupByName(projected.NodeGroups, "GRP_ALL")
	if group == nil || strings.Join(group.Members, ",") != strings.Join(want, ",") {
		t.Fatalf("GRP_ALL = %+v，期望 %v", group, want)
	}
}

// T-TGT-ID-003：组名和保留名只在目标格式内占用名字，不是关系图中的标识；
// 与它们冲突的节点在投影时改名。
func TestTargetProjection_AvoidsGroupAndReservedNames(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			ssProxy(1, "GRP_ALL"),
			ssProxy(2, "DIRECT"),
		},
		NodeGroups: []model.NodeGroup{nodeGroup("GRP_ALL", 1, 2)},
		RouteGroups: []model.RouteGroup{
			routeGroup("SVC_FINAL", nodePolicy("GRP_ALL"), reservedPolicy("DIRECT")),
		},
		Fallback: "SVC_FINAL",
	}

	projected, err := ForClash(p)
	if err != nil {
		t.Fatalf("ForClash: %v", err)
	}
	if projected.Proxies[0].Name != "GRP_ALL②" || projected.Proxies[1].Name != "DIRECT②" {
		t.Fatalf("投影后的节点 = %+v", projected.Proxies)
	}
}

// T-TGT-ID-004：链式节点的名字和对上游的引用，都取自上游节点在目标格式内
// 最终分配到的名字。
func TestTargetProjection_ChainedNamesUseFinalDialerName(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			ssProxy(1, "UPSTREAM"),
			ssProxy(2, "UPSTREAM"),
			chainedProxy(3, "UPSTREAM→CHAIN", 2, "CHAIN"),
		},
		NodeGroups: []model.NodeGroup{nodeGroup("GRP_CHAIN", 3)},
		RouteGroups: []model.RouteGroup{
			routeGroup("SVC_FINAL", nodePolicy("GRP_CHAIN")),
		},
		Fallback: "SVC_FINAL",
	}

	projected, err := ForClash(p)
	if err != nil {
		t.Fatalf("ForClash: %v", err)
	}
	chain := targetProxyByName(projected.Proxies, "UPSTREAM②→CHAIN")
	if chain == nil {
		t.Fatalf("投影后的节点 = %+v", projected.Proxies)
	}
	if chain.Dialer != "UPSTREAM②" {
		t.Fatalf("chain.Dialer = %q，期望 UPSTREAM②", chain.Dialer)
	}
	if group := targetGroupByName(projected.NodeGroups, "GRP_CHAIN"); group == nil || len(group.Members) != 1 || group.Members[0] != chain.Name {
		t.Fatalf("GRP_CHAIN = %+v，期望 %q", group, chain.Name)
	}
}

// T-TGT-014：投影不变量失败时使用各目标格式专用的错误码。
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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := internalFilterError(test.opts, "boom")
			var targetErr *errtype.TargetError
			if !errors.As(err, &targetErr) {
				t.Fatalf("error = %T，期望 TargetError", err)
			}
			if targetErr.Code != test.want {
				t.Fatalf("Code = %q，期望 %q", targetErr.Code, test.want)
			}
		})
	}
}
