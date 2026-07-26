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

// T-TGT-001: Clash drops Snell nodes and all empty dependants.
func TestForClash_DropsSnellCascade(t *testing.T) {
	projected, err := ForClash(buildSnellFilterPipeline())
	if err != nil {
		t.Fatalf("ForClash: %v", err)
	}

	proxies := targetProxyNameSet(projected.Proxies)
	if proxies["HK-Snell"] || proxies["SG-Snell"] || !proxies["HK-01"] {
		t.Fatalf("Clash proxies = %v, want HK-01 only", proxies)
	}
	nodeGroups := targetGroupNameSet(projected.NodeGroups)
	if nodeGroups["GRP_SG"] || !nodeGroups["GRP_HK"] {
		t.Fatalf("Clash node groups = %v", nodeGroups)
	}
	routeGroups := targetGroupNameSet(projected.RouteGroups)
	if routeGroups["SVC_SGOnly"] || !routeGroups["SVC_Quick"] || !routeGroups["SVC_FINAL"] {
		t.Fatalf("Clash route groups = %v", routeGroups)
	}
	if len(projected.Rulesets) != 1 || projected.Rulesets[0].Policy != "SVC_Quick" {
		t.Fatalf("Clash rulesets = %+v", projected.Rulesets)
	}
	if len(projected.Rules) != 1 || projected.Rules[0].Policy != "SVC_Quick" {
		t.Fatalf("Clash rules = %+v", projected.Rules)
	}
}

// T-TGT-002: chained nodes follow the identity of their dialer.
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
		t.Fatalf("projected proxies = %v", proxies)
	}
}

// T-TGT-003: cleared fallback reports the complete identity-aware path.
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
		t.Fatalf("error = %T %v, want TargetError", err, err)
	}
	if targetErr.Code != errtype.CodeTargetClashFallbackEmpty {
		t.Fatalf("Code = %q", targetErr.Code)
	}
	for _, want := range []string{"SVC_FINAL", "GRP_SG", "HK-Snell#1(snell)"} {
		if !strings.Contains(targetErr.Error(), want) {
			t.Fatalf("error missing %q: %s", want, targetErr.Error())
		}
	}
}

// T-TGT-004: chain removal paths retain both child and root identities.
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
			t.Fatalf("error missing %q: %s", want, err)
		}
	}
}

// T-TGT-005: shared dropped subgraphs are not cycles.
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
		t.Fatalf("shared subgraph error = %v", err)
	}
}

// T-TGT-006: cascade converges even when route groups are not topological.
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
			t.Fatalf("error missing %q: %s", want, err)
		}
	}
}

// T-TGT-007: direct proxy references are pruned by identity.
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
		t.Fatalf("all-Snell route group survived: %+v", projected.RouteGroups)
	}
}

// T-TGT-008: a compatible pipeline is still projected into renderer types.
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
		t.Fatalf("projected proxies = %+v", projected.Proxies)
	}
	if got := targetGroupByName(projected.NodeGroups, "GRP_HK"); got == nil || len(got.Members) != 1 || got.Members[0] != "HK-01" {
		t.Fatalf("projected node groups = %+v", projected.NodeGroups)
	}
}

// T-TGT-009: Surge applies the symmetric VLESS cascade.
func TestForSurge_DropsVLessCascade(t *testing.T) {
	projected, err := ForSurge(buildVLessFilterPipeline())
	if err != nil {
		t.Fatalf("ForSurge: %v", err)
	}
	proxies := targetProxyNameSet(projected.Proxies)
	if proxies["HK-VL"] || proxies["SG-VL"] || !proxies["HK-01"] {
		t.Fatalf("Surge proxies = %v", proxies)
	}
	if targetGroupNameSet(projected.NodeGroups)["GRP_SG"] {
		t.Fatalf("Surge node groups = %+v", projected.NodeGroups)
	}
	if targetGroupNameSet(projected.RouteGroups)["SVC_SGOnly"] {
		t.Fatalf("Surge route groups = %+v", projected.RouteGroups)
	}
}

// T-TGT-010: Surge drops chains whose VLESS dialer was removed.
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
		t.Fatalf("VLESS chain survived: %+v", projected.Proxies)
	}
	if targetGroupNameSet(projected.NodeGroups)["GRP_CHAIN"] {
		t.Fatalf("empty chain group survived: %+v", projected.NodeGroups)
	}
}

// T-TGT-011: Surge uses its dedicated fallback error code.
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
			t.Fatalf("error missing %q: %s", want, targetErr.Error())
		}
	}
}

// T-TGT-012: projection keeps only target-native structured subscriptions.
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
		t.Fatalf("Clash names = %v", clashNames)
	}

	surge, err := ForSurge(p)
	if err != nil {
		t.Fatalf("ForSurge: %v", err)
	}
	surgeNames := targetProxyNameSet(surge.Proxies)
	if !surgeNames["SG-CONF"] || !surgeNames["US-TXT"] || surgeNames["HK-YAML"] {
		t.Fatalf("Surge names = %v", surgeNames)
	}
}

// T-TGT-013: incompatible native roots remain visible in cascade diagnostics.
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
			t.Fatalf("error missing %q: %s", want, err)
		}
	}
}

// T-TGT-ID-001: names are allocated independently after format filtering.
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
		t.Fatalf("Clash proxies = %+v, want original SAME", clash.Proxies)
	}
	if len(surge.Proxies) != 1 || surge.Proxies[0].Name != "SAME" {
		t.Fatalf("Surge proxies = %+v, want original SAME", surge.Proxies)
	}
}

// T-TGT-ID-002: duplicates inside one target use deterministic suffixes, while
// preserving natural suffix declarations.
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
			t.Errorf("Proxies[%d].Name = %q, want %q", index, projected.Proxies[index].Name, name)
		}
	}
	group := targetGroupByName(projected.NodeGroups, "GRP_ALL")
	if group == nil || strings.Join(group.Members, ",") != strings.Join(want, ",") {
		t.Fatalf("GRP_ALL = %+v, want %v", group, want)
	}
}

// T-TGT-ID-003: group and reserved names are target-local reservations, not
// graph identities; colliding proxies are renamed during projection.
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
		t.Fatalf("projected proxies = %+v", projected.Proxies)
	}
}

// T-TGT-ID-004: a chain name and dialer reference are derived from the
// dialer's final target-local name.
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
		t.Fatalf("projected proxies = %+v", projected.Proxies)
	}
	if chain.Dialer != "UPSTREAM②" {
		t.Fatalf("chain.Dialer = %q, want UPSTREAM②", chain.Dialer)
	}
	if group := targetGroupByName(projected.NodeGroups, "GRP_CHAIN"); group == nil || len(group.Members) != 1 || group.Members[0] != chain.Name {
		t.Fatalf("GRP_CHAIN = %+v, want %q", group, chain.Name)
	}
}

// T-TGT-014: projection-invariant failures use target-specific codes.
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
				t.Fatalf("error = %T, want TargetError", err)
			}
			if targetErr.Code != test.want {
				t.Fatalf("Code = %q, want %q", targetErr.Code, test.want)
			}
		})
	}
}
