package pipeline

import (
	"errors"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
	"gopkg.in/yaml.v3"
)

// --- 测试辅助函数 ---

func makeSubProxy(name string) model.Proxy {
	return model.Proxy{
		ID:     1,
		Name:   name,
		Type:   "ss",
		Server: "example.com",
		Port:   8388,
		Params: map[string]string{"cipher": "aes-256-cfb", "password": "test"},
		Kind:   model.KindSubscription,
	}
}

func makeCustomProxyModel(name string) model.Proxy {
	return model.Proxy{
		Name:   name,
		Type:   "socks5",
		Server: "1.2.3.4",
		Port:   1080,
		Kind:   model.KindCustom,
	}
}

func mustGroupsMap(t *testing.T, yamlStr string) config.OrderedMap[config.Group] {
	t.Helper()
	var m config.OrderedMap[config.Group]
	if err := yaml.Unmarshal([]byte(yamlStr), &m); err != nil {
		t.Fatalf("mustGroupsMap 出错：%v", err)
	}
	return m
}

func mustRoutingMap(t *testing.T, yamlStr string) config.OrderedMap[[]string] {
	t.Helper()
	var m config.OrderedMap[[]string]
	if err := yaml.Unmarshal([]byte(yamlStr), &m); err != nil {
		t.Fatalf("mustRoutingMap 出错：%v", err)
	}
	return m
}

// --- Group 测试 ---

// T-GRP-001：区域组正则匹配
func TestGroup_RegionGroupMatching(t *testing.T) {
	proxies := []model.Proxy{
		makeSubProxy("HK-01"),
		makeSubProxy("HK-02"),
		makeSubProxy("SG-01"),
		makeSubProxy("JP-01"),
		makeSubProxy("US-01"),
		makeCustomProxyModel("MY-PROXY"),
	}

	cfg := &config.Config{
		Groups: mustGroupsMap(t, `
"🇭🇰 Hong Kong": { match: "(HK)", strategy: select }
"🇸🇬 Singapore": { match: "(SG)", strategy: url-test }
`),
	}

	result, err := groupFromConfig(cfg, proxies)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	if len(result.NodeGroups) != 2 {
		t.Fatalf("得到 %d 个节点组，期望 2 个", len(result.NodeGroups))
	}

	// HK 组
	hk := result.NodeGroups[0]
	if hk.Name != "🇭🇰 Hong Kong" {
		t.Errorf("group[0].Name = %q，期望 %q", hk.Name, "🇭🇰 Hong Kong")
	}
	if hk.Strategy != "select" {
		t.Errorf("group[0].Strategy = %q，期望 %q", hk.Strategy, "select")
	}
	wantHK := []string{"HK-01", "HK-02"}
	if len(hk.Members) != len(wantHK) {
		t.Fatalf("HK 成员 = %v，期望 %v", hk.Members, wantHK)
	}
	for i, want := range wantHK {
		if got := proxyNameByID(result.Proxies, hk.Members[i]); got != want {
			t.Errorf("HK.Members[%d] 解析为 %q，期望 %q", i, got, want)
		}
	}

	// SG 组
	sg := result.NodeGroups[1]
	if sg.Name != "🇸🇬 Singapore" {
		t.Errorf("group[1].Name = %q，期望 %q", sg.Name, "🇸🇬 Singapore")
	}
	if sg.Strategy != "url-test" {
		t.Errorf("group[1].Strategy = %q，期望 %q", sg.Strategy, "url-test")
	}
	if len(sg.Members) != 1 || proxyNameByID(result.Proxies, sg.Members[0]) != "SG-01" {
		t.Errorf("SG.Members = %v，期望 [SG-01]", sg.Members)
	}

	// 自定义节点 "MY-PROXY" 不应出现在任何区域组中。
	for _, g := range result.NodeGroups {
		for _, m := range g.Members {
			if proxyNameByID(result.Proxies, m) == "MY-PROXY" {
				t.Errorf("自定义节点 MY-PROXY 不应被区域组 %q 匹配", g.Name)
			}
		}
	}
}

// T-GRP-002：relay_through type=group
func TestGroup_ChainedTypeGroup(t *testing.T) {
	proxies := []model.Proxy{
		makeSubProxy("HK-01"),
		makeSubProxy("HK-02"),
	}

	cfg := &config.Config{
		Groups: mustGroupsMap(t, `
"🇭🇰 Hong Kong": { match: "(HK)", strategy: select }
`),
		Sources: config.Sources{
			CustomProxies: []config.CustomProxy{
				customProxy("HK-ISP", "socks5://user1:pass1@154.197.1.1:45002", &config.RelayThrough{
					Type:     "group",
					Name:     "🇭🇰 Hong Kong",
					Strategy: "select",
				}),
			},
		},
	}

	result, err := groupFromConfig(cfg, proxies)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	// 应有 2 个节点组：区域组 + 链式组。
	if len(result.NodeGroups) != 2 {
		t.Fatalf("得到 %d 个节点组，期望 2 个", len(result.NodeGroups))
	}

	chainGroup := result.NodeGroups[1]
	if chainGroup.Name != "HK-ISP" {
		t.Errorf("链式组名称 = %q，期望 %q", chainGroup.Name, "HK-ISP")
	}
	if chainGroup.Strategy != "select" {
		t.Errorf("链式组策略 = %q，期望 %q", chainGroup.Strategy, "select")
	}

	wantMembers := []string{"HK-01→HK-ISP", "HK-02→HK-ISP"}
	if len(chainGroup.Members) != len(wantMembers) {
		t.Fatalf("链式组成员 = %v，期望 %v", chainGroup.Members, wantMembers)
	}
	for i, want := range wantMembers {
		if got := proxyNameByID(result.Proxies, chainGroup.Members[i]); got != want {
			t.Errorf("chain.Members[%d] 解析为 %q，期望 %q", i, got, want)
		}
	}

	// 2 个订阅节点 + 2 个链式节点 = 4。Source 阶段会丢弃带 relay_through 的
	// 自定义节点（见 convertCustomProxies），所以 HK-ISP 不会以 KindCustom
	// 条目进入节点列表。
	if len(result.Proxies) != 4 {
		t.Fatalf("得到 %d 个节点，期望 4 个", len(result.Proxies))
	}

	chained := result.Proxies[2] // 第一个链式节点
	if chained.Name != "HK-01→HK-ISP" {
		t.Errorf("chained.Name = %q，期望 %q", chained.Name, "HK-01→HK-ISP")
	}
	if chained.Kind != model.KindChained {
		t.Errorf("chained.Kind = %q，期望 %q", chained.Kind, model.KindChained)
	}
	if got := proxyNameByID(result.Proxies, chained.DialerID); got != "HK-01" {
		t.Errorf("chained.DialerID 解析为 %q，期望 %q", got, "HK-01")
	}
	if chained.Type != "socks5" {
		t.Errorf("chained.Type = %q，期望 %q", chained.Type, "socks5")
	}
	if chained.Server != "154.197.1.1" {
		t.Errorf("chained.Server = %q，期望 %q", chained.Server, "154.197.1.1")
	}
	if chained.Port != 45002 {
		t.Errorf("chained.Port = %d，期望 %d", chained.Port, 45002)
	}
}

// T-GRP-ID-001：已被过滤的来源节点编号永远不会被链式节点复用。
func TestGroup_ChainedIDStartsAfterSourceAllocationHighWatermark(t *testing.T) {
	source := &SourceResult{
		Proxies: []model.Proxy{
			makeSubProxy("HK-01"),
		},
		ChainTemplates: []ChainTemplate{
			{
				Name:   "RELAY",
				Type:   "socks5",
				Server: "relay.example.com",
				Port:   1080,
				RelayThrough: &config.PreparedRelayThrough{
					Type:     "all",
					Strategy: "select",
				},
			},
		},
		NextProxyID: 3, // ID 2 属于一个被 Filter 移除的原始节点。
	}

	result, err := Group(source, nil)
	if err != nil {
		t.Fatalf("Group 出错：%v", err)
	}
	if len(result.Proxies) != 2 {
		t.Fatalf("Proxies = %+v，期望一个原始节点和一个链式节点", result.Proxies)
	}
	if got := result.Proxies[1].ID; got != 3 {
		t.Fatalf("链式节点 ID = %d，期望 3；已过滤的来源 ID 2 不得被复用", got)
	}
}

// T-GRP-003：relay_through type=select
func TestGroup_ChainedTypeSelect(t *testing.T) {
	proxies := []model.Proxy{
		makeSubProxy("HK-01"),
		makeSubProxy("SG-01"),
		makeSubProxy("JP-01"),
	}

	cfg := &config.Config{
		Sources: config.Sources{
			CustomProxies: []config.CustomProxy{
				customProxy("PROXY-A", "http://10.0.0.1:8080", &config.RelayThrough{
					Type:     "select",
					Match:    "(HK|SG)",
					Strategy: "url-test",
				}),
			},
		},
	}

	result, err := groupFromConfig(cfg, proxies)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	if len(result.NodeGroups) != 1 {
		t.Fatalf("得到 %d 个节点组，期望 1 个（只有链式组）", len(result.NodeGroups))
	}

	chainGroup := result.NodeGroups[0]
	if chainGroup.Strategy != "url-test" {
		t.Errorf("链式组策略 = %q，期望 %q", chainGroup.Strategy, "url-test")
	}

	wantMembers := []string{"HK-01→PROXY-A", "SG-01→PROXY-A"}
	if len(chainGroup.Members) != len(wantMembers) {
		t.Fatalf("链式组成员 = %v，期望 %v", chainGroup.Members, wantMembers)
	}
	for i, want := range wantMembers {
		if got := proxyNameByID(result.Proxies, chainGroup.Members[i]); got != want {
			t.Errorf("Members[%d] 解析为 %q，期望 %q", i, got, want)
		}
	}
}

// relay_through type=select 的 regex 若匹配不到任何上游，Group 阶段不报错，
// 但必须留下一个成员为空的链式组——随后由 ValidateGraph 以 "节点组 … 没有成员"
// 兜住。此测试钉住两点：
//  1. Group 阶段对 no-match 保持宽松（不会静默丢弃链式组模板）——防止未来改
//     buildChainedNodesAndGroups 为 "upstreams 为空就不建组" 导致 ValidateGraph
//     失去兜底目标
//  2. 链式组名以模板名命名，供 ValidateGraph 的错误消息定位用户配置位置
func TestGroup_ChainedTypeSelectNoMatchingUpstreams(t *testing.T) {
	proxies := []model.Proxy{
		makeSubProxy("HK-01"),
		makeSubProxy("SG-01"),
	}

	cfg := &config.Config{
		Sources: config.Sources{
			CustomProxies: []config.CustomProxy{
				customProxy("UNREACHED-CHAIN", "http://10.0.0.1:8080", &config.RelayThrough{
					Type:     "select",
					Match:    "^NONE-",
					Strategy: "select",
				}),
			},
		},
	}

	result, err := groupFromConfig(cfg, proxies)
	if err != nil {
		t.Fatalf("Group 阶段应容忍匹配不到上游，却得到错误：%v", err)
	}

	if len(result.NodeGroups) != 1 {
		t.Fatalf("得到 %d 个节点组，期望 1 个（空链式组）", len(result.NodeGroups))
	}
	chainGroup := result.NodeGroups[0]
	if chainGroup.Name != "UNREACHED-CHAIN" {
		t.Errorf("链式组 Name = %q，期望 %q", chainGroup.Name, "UNREACHED-CHAIN")
	}
	if len(chainGroup.Members) != 0 {
		t.Fatalf("链式组 Members = %v，期望为空", chainGroup.Members)
	}

	// 把这个空链式组喂给 ValidateGraph，确认下游兜底链路依然生效——
	// 若未来有人在 ValidateGraph 里漏掉空节点组检查，此分支会失败。
	gr := &GroupResult{
		Proxies:    result.Proxies,
		NodeGroups: result.NodeGroups,
		AllProxies: result.AllProxies,
	}
	rr := &RouteResult{
		RouteGroups: []model.RouteGroup{
			{Name: "Final", Strategy: "select", Members: []model.PolicyRef{
				{Kind: model.PolicyRefNodeGroup, Name: "UNREACHED-CHAIN"},
				{Kind: model.PolicyRefReserved, Name: "DIRECT"},
			}},
		},
		PreparedRouteGroups: []config.PreparedRouteGroup{
			{
				Name: "Final",
				DeclaredMembers: []config.PreparedRouteMember{
					{Raw: "UNREACHED-CHAIN", Origin: config.RouteMemberOriginLiteral},
					{Raw: "DIRECT", Origin: config.RouteMemberOriginLiteral},
				},
				ExpandedMembers: []config.PreparedRouteMember{
					{Raw: "UNREACHED-CHAIN", Origin: config.RouteMemberOriginLiteral},
					{Raw: "DIRECT", Origin: config.RouteMemberOriginLiteral},
				},
			},
		},
		ResolvedRouteGroups: []ResolvedRouteGroup{
			{
				Name: "Final",
				Members: []ResolvedRouteMember{
					{Ref: model.PolicyRef{Kind: model.PolicyRefNodeGroup, Name: "UNREACHED-CHAIN"}, Raw: "UNREACHED-CHAIN", Origin: config.RouteMemberOriginLiteral},
					{Ref: model.PolicyRef{Kind: model.PolicyRefReserved, Name: "DIRECT"}, Raw: "DIRECT", Origin: config.RouteMemberOriginLiteral},
				},
			},
		},
		Fallback: "Final",
	}

	if _, err := ValidateGraph(gr, rr); err == nil {
		t.Fatal("ValidateGraph 应拒绝空链式组")
	} else if !strings.Contains(err.Error(), "UNREACHED-CHAIN") || !strings.Contains(err.Error(), "没有成员") {
		t.Errorf("ValidateGraph 的错误应点名这个空链式组，实际：%v", err)
	}
}

func TestGroup_ProxyInvariantValidation(t *testing.T) {
	result, err := Group(&SourceResult{
		Proxies: []model.Proxy{
			makeSubProxy("HK-01"),
		},
		ChainTemplates: []ChainTemplate{
			{
				Name:   "BROKEN-CHAIN",
				Type:   "ss",
				Server: "1.2.3.4",
				Port:   8388,
				Params: map[string]string{"cipher": "aes-256-gcm"},
				RelayThrough: &config.PreparedRelayThrough{
					Type:     "all",
					Strategy: "select",
				},
			},
		},
	}, nil)
	if err == nil {
		t.Fatal("期望不变量错误，实际为 nil")
	}
	if result != nil {
		t.Fatalf("result = %+v，期望 nil", result)
	}

	var buildErr *errtype.BuildError
	if !errors.As(err, &buildErr) {
		t.Fatalf("错误类型 = %T，期望 *errtype.BuildError", err)
	}
	if buildErr.Phase != "group" {
		t.Errorf("Phase = %q，期望 group", buildErr.Phase)
	}
	if !strings.Contains(buildErr.Message, "type=ss 缺少必填参数 password") {
		t.Fatalf("意外的错误信息：%s", buildErr.Message)
	}
}

// T-GRP-004：relay_through type=all
func TestGroup_ChainedTypeAll(t *testing.T) {
	proxies := []model.Proxy{
		makeSubProxy("HK-01"),
		makeSubProxy("SG-01"),
	}

	cfg := &config.Config{
		Sources: config.Sources{
			CustomProxies: []config.CustomProxy{
				customProxy("PROXY-B", "socks5://1.1.1.1:1080", &config.RelayThrough{
					Type:     "all",
					Strategy: "select",
				}),
			},
		},
	}

	result, err := groupFromConfig(cfg, proxies)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	if len(result.NodeGroups) != 1 {
		t.Fatalf("得到 %d 个节点组，期望 1 个", len(result.NodeGroups))
	}

	wantMembers := []string{"HK-01→PROXY-B", "SG-01→PROXY-B"}
	chainGroup := result.NodeGroups[0]
	if len(chainGroup.Members) != len(wantMembers) {
		t.Fatalf("链式组成员 = %v，期望 %v", chainGroup.Members, wantMembers)
	}
	for i, want := range wantMembers {
		if got := proxyNameByID(result.Proxies, chainGroup.Members[i]); got != want {
			t.Errorf("Members[%d] 解析为 %q，期望 %q", i, got, want)
		}
	}
}

// T-GRP-005：@all 不包含链式节点
func TestGroup_AllProxiesExcludesChained(t *testing.T) {
	proxies := []model.Proxy{
		makeSubProxy("HK-01"),
		makeSubProxy("SG-01"),
	}

	cfg := &config.Config{
		Sources: config.Sources{
			CustomProxies: []config.CustomProxy{
				customProxy("MY-PROXY", "socks5://1.1.1.1:1080", nil),
				customProxy("CHAIN-PROXY", "socks5://2.2.2.2:1080", &config.RelayThrough{
					Type:     "all",
					Strategy: "select",
				}),
			},
		},
	}

	result, err := groupFromConfig(cfg, proxies)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	// AllProxies 应包含订阅节点和自定义节点，但不包含链式节点。
	wantAll := []string{"HK-01", "SG-01", "MY-PROXY"}
	if len(result.AllProxies) != len(wantAll) {
		t.Fatalf("AllProxies = %v，期望 %v", result.AllProxies, wantAll)
	}
	for i, want := range wantAll {
		if got := proxyNameByID(result.Proxies, result.AllProxies[i]); got != want {
			t.Errorf("AllProxies[%d] 解析为 %q，期望 %q", i, got, want)
		}
	}

	// 链式节点不应出现在 AllProxies 中。
	allSet := make(map[string]bool)
	for _, id := range result.AllProxies {
		allSet[proxyNameByID(result.Proxies, id)] = true
	}
	for _, p := range result.Proxies {
		if p.Kind == model.KindChained && allSet[p.Name] {
			t.Errorf("链式节点 %q 不应出现在 AllProxies 中", p.Name)
		}
	}
}

// T-GRP-006：relay_through type=group 引用不存在的组
func TestGroup_ChainedGroupRefNotFound(t *testing.T) {
	proxies := []model.Proxy{
		makeSubProxy("HK-01"),
	}

	cfg := &config.Config{
		Groups: mustGroupsMap(t, `
"🇭🇰 Hong Kong": { match: "(HK)", strategy: select }
`),
		Sources: config.Sources{
			CustomProxies: []config.CustomProxy{
				customProxy("PROXY-X", "socks5://1.1.1.1:1080", &config.RelayThrough{
					Type:     "group",
					Name:     "🇰🇷 Korea",
					Strategy: "select",
				}),
			},
		},
	}

	_, err := groupFromConfig(cfg, proxies)
	if err == nil {
		t.Fatal("引用不存在的组时应返回错误")
	}

	var buildErr *errtype.BuildError
	if !errors.As(err, &buildErr) {
		t.Fatalf("错误类型 = %T，期望 *errtype.BuildError", err)
	}
	if buildErr.Phase != "group" {
		t.Errorf("Phase = %q，期望 %q", buildErr.Phase, "group")
	}
}

// T-GRP-007：没有带 relay_through 的自定义节点
func TestGroup_NoChaining(t *testing.T) {
	proxies := []model.Proxy{
		makeSubProxy("HK-01"),
		makeSubProxy("SG-01"),
	}

	cfg := &config.Config{
		Groups: mustGroupsMap(t, `
"🇭🇰 Hong Kong": { match: "(HK)", strategy: select }
`),
		Sources: config.Sources{
			CustomProxies: []config.CustomProxy{
				customProxy("MY-PROXY", "socks5://1.1.1.1:1080", nil),
			},
		},
	}

	result, err := groupFromConfig(cfg, proxies)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	// 只有 1 个区域组，没有链式组。
	if len(result.NodeGroups) != 1 {
		t.Fatalf("得到 %d 个节点组，期望 1 个", len(result.NodeGroups))
	}
	if result.NodeGroups[0].Name != "🇭🇰 Hong Kong" {
		t.Errorf("组名 = %q，期望区域组", result.NodeGroups[0].Name)
	}

	// Proxies 不变（没有新增链式节点）。
	if len(result.Proxies) != 3 {
		t.Errorf("得到 %d 个节点，期望 3 个（无链式节点）", len(result.Proxies))
	}
}

// T-GRP-008：多个带 relay_through 的自定义节点
func TestGroup_MultipleChainedGroups(t *testing.T) {
	proxies := []model.Proxy{
		makeSubProxy("HK-01"),
		makeSubProxy("SG-01"),
	}

	cfg := &config.Config{
		Groups: mustGroupsMap(t, `
"🇭🇰 Hong Kong": { match: "(HK)", strategy: select }
`),
		Sources: config.Sources{
			CustomProxies: []config.CustomProxy{
				customProxy("ISP-A", "socks5://1.1.1.1:1080", &config.RelayThrough{Type: "group", Name: "🇭🇰 Hong Kong", Strategy: "select"}),
				customProxy("ISP-B", "http://2.2.2.2:8080", &config.RelayThrough{Type: "all", Strategy: "url-test"}),
			},
		},
	}

	result, err := groupFromConfig(cfg, proxies)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	// 1 个区域组 + 2 个链式组 = 3。
	if len(result.NodeGroups) != 3 {
		t.Fatalf("得到 %d 个节点组，期望 3 个", len(result.NodeGroups))
	}

	// 链式组排在区域组之后，按自定义节点的声明顺序排列。
	if result.NodeGroups[1].Name != "ISP-A" {
		t.Errorf("group[1].Name = %q，期望 %q", result.NodeGroups[1].Name, "ISP-A")
	}
	if result.NodeGroups[2].Name != "ISP-B" {
		t.Errorf("group[2].Name = %q，期望 %q", result.NodeGroups[2].Name, "ISP-B")
	}

	// ISP-A 经由 HK 组链式连接（1 个成员：HK-01）。
	if len(result.NodeGroups[1].Members) != 1 {
		t.Errorf("ISP-A 链式组成员 = %v，期望 1 个成员", result.NodeGroups[1].Members)
	}
	// ISP-B 经由全部节点链式连接（2 个成员：HK-01、SG-01）。
	if len(result.NodeGroups[2].Members) != 2 {
		t.Errorf("ISP-B 链式组成员 = %v，期望 2 个成员", result.NodeGroups[2].Members)
	}
}

// T-GRP-009：链式节点的属性（username/password）
func TestGroup_ChainedNodeProperties(t *testing.T) {
	proxies := []model.Proxy{
		makeSubProxy("HK-01"),
	}

	cfg := &config.Config{
		Sources: config.Sources{
			CustomProxies: []config.CustomProxy{
				customProxy("AUTH-PROXY", "socks5://admin:secret@10.0.0.1:9090", &config.RelayThrough{
					Type:     "all",
					Strategy: "select",
				}),
			},
		},
	}

	result, err := groupFromConfig(cfg, proxies)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	// 找到链式节点。
	var chained *model.Proxy
	for i := range result.Proxies {
		if result.Proxies[i].Kind == model.KindChained {
			chained = &result.Proxies[i]
			break
		}
	}
	if chained == nil {
		t.Fatal("没有找到链式节点")
	}

	if chained.Type != "socks5" {
		t.Errorf("Type = %q，期望 socks5", chained.Type)
	}
	if chained.Server != "10.0.0.1" {
		t.Errorf("Server = %q，期望 10.0.0.1", chained.Server)
	}
	if chained.Port != 9090 {
		t.Errorf("Port = %d，期望 9090", chained.Port)
	}
	if chained.Params["username"] != "admin" {
		t.Errorf("username = %q，期望 admin", chained.Params["username"])
	}
	if chained.Params["password"] != "secret" {
		t.Errorf("password = %q，期望 secret", chained.Params["password"])
	}
	if got := proxyNameByID(result.Proxies, chained.DialerID); got != "HK-01" {
		t.Errorf("DialerID 解析为 %q，期望 HK-01", got)
	}
}

// T-GRP-SS-01：SS 链式模板的 plugin 传递到链式节点。
//
// 验证带 plugin 的 ss 类型 custom_proxy 会把 plugin 传给由它派生的每个链式节点。
// 如果不显式传递，链式 ss 节点会悄无声息地丢掉 obfs/v2ray-plugin 配置。
func TestGroup_ChainedSSNodePluginPropagation(t *testing.T) {
	proxies := []model.Proxy{
		makeSubProxy("HK-01"),
		makeSubProxy("SG-01"),
	}

	cfg := &config.Config{
		Sources: config.Sources{
			CustomProxies: []config.CustomProxy{
				customProxy("SS-Chain", "ss://YWVzLTI1Ni1nY206Y2hhaW5wYXNz@1.2.3.4:8388?plugin=obfs-local%3Bobfs%3Dhttp%3Bobfs-host%3Dexample.com", &config.RelayThrough{
					Type:     "all",
					Strategy: "select",
				}),
			},
		},
	}

	result, err := groupFromConfig(cfg, proxies)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	chainedCount := 0
	for _, px := range result.Proxies {
		if px.Kind != model.KindChained {
			continue
		}
		chainedCount++
		if px.Type != "ss" {
			t.Errorf("链式节点 %q Type = %q，期望 ss", px.Name, px.Type)
		}
		if px.Params["cipher"] != "aes-256-gcm" || px.Params["password"] != "chainpass" {
			t.Errorf("链式节点 %q Params = %v", px.Name, px.Params)
		}
		if _, ok := px.Params["username"]; ok {
			t.Errorf("链式节点 %q 是 ss 节点，Params 中不应带 username", px.Name)
		}
		if px.Plugin == nil {
			t.Errorf("链式节点 %q 丢失了链式模板的 Plugin", px.Name)
			continue
		}
		if px.Plugin.Name != "obfs-local" {
			t.Errorf("链式节点 %q Plugin.Name = %q", px.Name, px.Plugin.Name)
		}
		if px.Plugin.Opts["obfs"] != "http" || px.Plugin.Opts["obfs-host"] != "example.com" {
			t.Errorf("链式节点 %q Plugin.Opts = %v", px.Name, px.Plugin.Opts)
		}
	}
	if chainedCount != 2 {
		t.Errorf("得到 %d 个链式节点，期望 2 个（每个上游一个）", chainedCount)
	}
}

// T-GRP-010：Proxies 合并顺序（先原始节点，后链式节点）
func TestGroup_ProxiesMergeOrder(t *testing.T) {
	proxies := []model.Proxy{
		makeSubProxy("HK-01"),
		makeCustomProxyModel("MY-PROXY"),
	}

	cfg := &config.Config{
		Sources: config.Sources{
			CustomProxies: []config.CustomProxy{
				customProxy("CHAIN-PROXY", "socks5://1.1.1.1:1080", &config.RelayThrough{Type: "all", Strategy: "select"}),
			},
		},
	}

	result, err := groupFromConfig(cfg, proxies)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	if len(result.Proxies) != 3 {
		t.Fatalf("得到 %d 个节点，期望 3 个", len(result.Proxies))
	}

	// 原始节点在前。
	if result.Proxies[0].Name != "HK-01" || result.Proxies[0].Kind != model.KindSubscription {
		t.Errorf("proxy[0] = %q (%s)，期望 HK-01 (subscription)", result.Proxies[0].Name, result.Proxies[0].Kind)
	}
	if result.Proxies[1].Name != "MY-PROXY" || result.Proxies[1].Kind != model.KindCustom {
		t.Errorf("proxy[1] = %q (%s)，期望 MY-PROXY (custom)", result.Proxies[1].Name, result.Proxies[1].Kind)
	}
	// 链式节点在最后。
	if result.Proxies[2].Kind != model.KindChained {
		t.Errorf("proxy[2].Kind = %q，期望 chained", result.Proxies[2].Kind)
	}
	if result.Proxies[2].Name != "HK-01→CHAIN-PROXY" {
		t.Errorf("proxy[2].Name = %q，期望 HK-01→CHAIN-PROXY", result.Proxies[2].Name)
	}
}

// T-GRP-SNELL-001：Snell 节点与订阅节点一起参与区域组正则匹配。
// 防止将来的改动把 isFetchedKind/fetchedProxies 收窄回只认 KindSubscription。
func TestGroup_SnellParticipatesInRegionMatch(t *testing.T) {
	proxies := []model.Proxy{
		makeSubProxy("HK-01"),
		{Name: "HK-Snell", Type: "snell", Server: "1.2.3.4", Port: 57891, Params: map[string]string{"psk": "x", "version": "4"}, Kind: model.KindSnell},
		{Name: "SG-Snell", Type: "snell", Server: "5.6.7.8", Port: 8989, Params: map[string]string{"psk": "y", "version": "4"}, Kind: model.KindSnell},
		makeCustomProxyModel("MY-PROXY"),
	}

	cfg := &config.Config{
		Groups: mustGroupsMap(t, `
"HK": { match: "(HK)", strategy: select }
"SG": { match: "(SG)", strategy: select }
`),
	}

	result, err := groupFromConfig(cfg, proxies)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	// HK 组应包含 HK-01（订阅）和 HK-Snell（snell）。
	hk := result.NodeGroups[0]
	if hk.Name != "HK" {
		t.Fatalf("group[0].Name = %q，期望 HK", hk.Name)
	}
	gotHK := map[string]bool{}
	for _, m := range hk.Members {
		gotHK[proxyNameByID(result.Proxies, m)] = true
	}
	if !gotHK["HK-01"] || !gotHK["HK-Snell"] {
		t.Errorf("HK 成员 = %v，期望同时包含 HK-01 和 HK-Snell", hk.Members)
	}

	// SG 组应只包含 SG-Snell。
	sg := result.NodeGroups[1]
	if len(sg.Members) != 1 || proxyNameByID(result.Proxies, sg.Members[0]) != "SG-Snell" {
		t.Errorf("SG 成员 = %v，期望 [SG-Snell]", sg.Members)
	}

	// 自定义节点不得出现在任何区域组中。
	for _, g := range result.NodeGroups {
		for _, m := range g.Members {
			if proxyNameByID(result.Proxies, m) == "MY-PROXY" {
				t.Errorf("自定义节点 MY-PROXY 混入了区域组 %q", g.Name)
			}
		}
	}

	// @all 展开结果包含 snell 节点。
	gotAll := map[string]bool{}
	for _, id := range result.AllProxies {
		gotAll[proxyNameByID(result.Proxies, id)] = true
	}
	for _, want := range []string{"HK-01", "HK-Snell", "SG-Snell", "MY-PROXY"} {
		if !gotAll[want] {
			t.Errorf("AllProxies 缺少 %q（实际 %v）", want, result.AllProxies)
		}
	}
}

// T-GROUP-VLESS-001 + T-GROUP-VLESS-003：VLESS 节点参与区域组正则匹配，
// 并被包含在 @all 中（与 Snell 的行为一致）。
func TestGroup_VLessParticipatesInRegionMatch(t *testing.T) {
	proxies := []model.Proxy{
		makeSubProxy("HK-01"),
		{Name: "HK-VL", Type: "vless", Server: "hk.example.com", Port: 443,
			Params: map[string]string{"uuid": "11111111-2222-3333-4444-555555555555", "security": "tls", "network": "tcp"},
			Kind:   model.KindVLess},
		{Name: "SG-VL", Type: "vless", Server: "sg.example.com", Port: 443,
			Params: map[string]string{"uuid": "11111111-2222-3333-4444-555555555555", "security": "tls", "network": "tcp"},
			Kind:   model.KindVLess},
		makeCustomProxyModel("MY-PROXY"),
	}

	cfg := &config.Config{
		Groups: mustGroupsMap(t, `
"HK": { match: "(HK)", strategy: select }
"SG": { match: "(SG)", strategy: select }
`),
	}

	result, err := groupFromConfig(cfg, proxies)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	// HK 组应包含 HK-01（订阅）和 HK-VL（vless）。
	hk := result.NodeGroups[0]
	if hk.Name != "HK" {
		t.Fatalf("group[0].Name = %q，期望 HK", hk.Name)
	}
	gotHK := map[string]bool{}
	for _, m := range hk.Members {
		gotHK[proxyNameByID(result.Proxies, m)] = true
	}
	if !gotHK["HK-01"] || !gotHK["HK-VL"] {
		t.Errorf("HK 成员 = %v，期望同时包含 HK-01 和 HK-VL", hk.Members)
	}

	// SG 组应只包含 SG-VL。
	sg := result.NodeGroups[1]
	if len(sg.Members) != 1 || proxyNameByID(result.Proxies, sg.Members[0]) != "SG-VL" {
		t.Errorf("SG 成员 = %v，期望 [SG-VL]", sg.Members)
	}

	// 自定义节点不得出现在任何区域组中。
	for _, g := range result.NodeGroups {
		for _, m := range g.Members {
			if proxyNameByID(result.Proxies, m) == "MY-PROXY" {
				t.Errorf("自定义节点 MY-PROXY 混入了区域组 %q", g.Name)
			}
		}
	}

	// @all 展开结果包含 VLESS 节点。
	gotAll := map[string]bool{}
	for _, id := range result.AllProxies {
		gotAll[proxyNameByID(result.Proxies, id)] = true
	}
	for _, want := range []string{"HK-01", "HK-VL", "SG-VL", "MY-PROXY"} {
		if !gotAll[want] {
			t.Errorf("AllProxies 缺少 %q（实际 %v）", want, result.AllProxies)
		}
	}
}

// T-GROUP-VLESS-002：VLESS 节点可以作为 relay_through 链的上游。
// 带 relay_through{type:all} 的自定义节点生成的链式节点，其 DialerID
// 指向 VLESS 节点的编号。
func TestGroup_VLessEligibleAsChainUpstream(t *testing.T) {
	proxies := []model.Proxy{
		{Name: "HK-VL", Type: "vless", Server: "hk.example.com", Port: 443,
			Params: map[string]string{"uuid": "11111111-2222-3333-4444-555555555555", "security": "tls", "network": "tcp"},
			Kind:   model.KindVLess},
	}

	cfg := &config.Config{
		Sources: config.Sources{
			CustomProxies: []config.CustomProxy{
				customProxy("MY-CHAIN", "socks5://127.0.0.1:1080", &config.RelayThrough{
					Type:     "all",
					Strategy: "select",
				}),
			},
		},
	}

	result, err := groupFromConfig(cfg, proxies)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	// 期望恰好一个链式节点，其 DialerID 解析为 HK-VL。
	var chained model.Proxy
	found := false
	for _, p := range result.Proxies {
		if p.Kind == model.KindChained {
			chained = p
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("没有生成链式节点；Proxies=%v", result.Proxies)
	}
	if got := proxyNameByID(result.Proxies, chained.DialerID); got != "HK-VL" {
		t.Errorf("chained.DialerID 解析为 %q，期望 HK-VL", got)
	}
	if chained.Name != "HK-VL→MY-CHAIN" {
		t.Errorf("chained.Name = %q，期望 HK-VL→MY-CHAIN", chained.Name)
	}
}

// 链式组名就是自定义节点名原样照搬，系统不注入任何前缀。
// 防止回归到在 cp.Name 前重新加上 🔗（或其他）硬编码标记。
func TestGroup_ChainedGroupNameEqualsCustomProxyName(t *testing.T) {
	proxies := []model.Proxy{makeSubProxy("HK-01")}

	cfg := &config.Config{
		Sources: config.Sources{
			CustomProxies: []config.CustomProxy{
				customProxy("HK-ISP", "socks5://1.1.1.1:1080", &config.RelayThrough{Type: "all", Strategy: "select"}),
			},
		},
	}

	result, err := groupFromConfig(cfg, proxies)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	if len(result.NodeGroups) != 1 {
		t.Fatalf("得到 %d 个节点组，期望 1 个", len(result.NodeGroups))
	}
	if got := result.NodeGroups[0].Name; got != "HK-ISP" {
		t.Errorf("链式组名称 = %q，期望 %q（无前缀）", got, "HK-ISP")
	}
}

// 用户在 cp.Name 里自带的 emoji 前缀必须原样带到链式组名上——系统从不添加
// 或去除视觉标记。这是上面那条回归防护的正向搭档：*想要* 🔗（或其他装饰）
// 的用户，得到的正是自己写的内容。
//
// 直接用 Go 构造（不用 YAML），以避开 AGENTS.md「几个容易出错的地方」中提到的
// 辅助平面字符转义问题。
func TestGroup_ChainedGroupNamePreservesUserEmojiPrefix(t *testing.T) {
	proxies := []model.Proxy{makeSubProxy("HK-01")}

	cfg := &config.Config{
		Sources: config.Sources{
			CustomProxies: []config.CustomProxy{
				customProxy("🔗 HK-ISP", "socks5://1.1.1.1:1080", &config.RelayThrough{Type: "all", Strategy: "select"}),
			},
		},
	}

	result, err := groupFromConfig(cfg, proxies)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	if len(result.NodeGroups) != 1 {
		t.Fatalf("得到 %d 个节点组，期望 1 个", len(result.NodeGroups))
	}
	if got := result.NodeGroups[0].Name; got != "🔗 HK-ISP" {
		t.Errorf("链式组名称 = %q，期望 %q（保留用户前缀）", got, "🔗 HK-ISP")
	}
	// 链式节点名在 "→" 之后同样原样使用 cp.Name。
	wantChainedName := "HK-01→🔗 HK-ISP"
	found := false
	for _, p := range result.Proxies {
		if p.Kind == model.KindChained && p.Name == wantChainedName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("在 proxies 中没有找到链式节点 %q；实际 %+v", wantChainedName, result.Proxies)
	}
}
