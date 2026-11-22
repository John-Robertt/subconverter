package pipeline

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// TestExecute_HappyPath 验证完整流水线产出有效的 Pipeline。
func TestExecute_HappyPath(t *testing.T) {
	subURL := "https://sub.example.com/api"
	body := makeSubResponse(
		"ss://YWVzLTI1Ni1jZmI6cGFzcw@hk.example.com:8388#HK-01",
		"ss://YWVzLTI1Ni1jZmI6cGFzcw@sg.example.com:8388#SG-01",
	)

	f := &fakeFetcher{responses: map[string][]byte{subURL: body}}

	cfg := &config.Config{
		Sources: config.Sources{
			Subscriptions: []config.Source{{URL: subURL}},
		},
		Groups:  mustGroupsMap(t, `"HK": { match: "(HK)", strategy: select }`),
		Routing: mustRoutingMap(t, `"proxy": ["HK", "DIRECT"]`),
		Rulesets: mustRoutingMap(t, `"proxy":
  - "https://example.com/rules.list"`),
		Rules:    []string{"GEOIP,CN,proxy"},
		Fallback: "proxy",
	}

	p, err := Execute(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	// Pipeline 应包含解析出的节点。
	if len(p.Proxies) != 2 {
		t.Errorf("得到 %d 个节点，期望 2 个", len(p.Proxies))
	}

	// 应有 1 个节点组（HK）和 1 个路由组（proxy）。
	if len(p.NodeGroups) != 1 {
		t.Errorf("得到 %d 个节点组，期望 1 个", len(p.NodeGroups))
	}
	if len(p.RouteGroups) != 1 {
		t.Errorf("得到 %d 个路由组，期望 1 个", len(p.RouteGroups))
	}

	// 记录了 Fallback。
	if p.Fallback != "proxy" {
		t.Errorf("fallback = %q，期望 %q", p.Fallback, "proxy")
	}

	// @all 应只包含原始节点。
	if len(p.AllProxies) != 2 {
		t.Errorf("AllProxies = %d，期望 2", len(p.AllProxies))
	}
}

// T-EXEC-ANYTLS-001：AnyTLS 订阅节点和其他原始订阅节点一样，
// 依次经过过滤、分组、路由和 @all。
func TestExecute_AnyTLSEndToEnd(t *testing.T) {
	subURL := "https://sub.example.com/anytls"
	body := makeSubResponse(
		"anytls://secret@hk.example.com:443/?sni=edge.example.com&insecure=1#HK-AnyTLS",
		"anytls://secret@sg.example.com:443/?sni=edge.example.com&insecure=1#SG-AnyTLS",
		"anytls://secret@info.example.com:443/?sni=edge.example.com&insecure=1#Traffic Reset",
	)

	f := &fakeFetcher{responses: map[string][]byte{subURL: body}}

	cfg := &config.Config{
		Sources: config.Sources{
			Subscriptions: []config.Source{{URL: subURL}},
		},
		Filters:  config.Filters{Exclude: "Traffic"},
		Groups:   mustGroupsMap(t, `"HK": { match: "(HK)", strategy: select }`),
		Routing:  mustRoutingMap(t, `"proxy": ["HK", "@all", "DIRECT"]`),
		Fallback: "proxy",
	}

	p, err := Execute(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	if len(p.Proxies) != 2 {
		t.Fatalf("得到 %d 个有效节点，期望 2 个：%+v", len(p.Proxies), p.Proxies)
	}
	for _, proxy := range p.Proxies {
		if proxy.Kind != model.KindSubscription || proxy.Type != "anytls" {
			t.Errorf("节点 %q Kind/Type = %q/%q，期望 subscription/anytls", proxy.Name, proxy.Kind, proxy.Type)
		}
	}
	if len(p.NodeGroups) != 1 || !containsProxyName(p.Proxies, p.NodeGroups[0].Members, "HK-AnyTLS") {
		t.Fatalf("HK 组成员 = %+v，期望 HK-AnyTLS", p.NodeGroups)
	}
	if !containsProxyName(p.Proxies, p.AllProxies, "HK-AnyTLS") || !containsProxyName(p.Proxies, p.AllProxies, "SG-AnyTLS") {
		t.Fatalf("@all = %+v，期望包含有效的 AnyTLS 节点", p.AllProxies)
	}
	if containsProxyName(p.Proxies, p.AllProxies, "Traffic Reset") {
		t.Fatalf("@all 混入了已过滤的信息节点：%+v", p.AllProxies)
	}
}

// T-EXEC-DOC-001：结构化订阅节点（各客户端原生格式）保持普通
// KindSubscription 在过滤、分组、@all 上的语义。
func TestExecute_StructuredSubscriptionsEndToEnd(t *testing.T) {
	const clashURL = "https://sub.example.com/nodes.yml"
	const surgeURL = "https://sub.example.com/nodes.conf"
	f := &fakeFetcher{responses: map[string][]byte{
		clashURL: []byte(`proxies:
  - { name: HK-YAML, type: trojan, server: hk.example.com, port: 443, password: pw }
  - { name: Traffic Reset, type: trojan, server: info.example.com, port: 443, password: pw }
`),
		surgeURL: []byte(`[Proxy]
DIRECT = direct
SG-CONF = trojan, sg.example.com, 443, password=pw
`),
	}}
	cfg := &config.Config{
		Sources: config.Sources{
			Subscriptions: []config.Source{{URL: clashURL}, {URL: surgeURL}},
		},
		Filters: config.Filters{Exclude: "Traffic"},
		Groups: mustGroupsMap(t,
			`"HK": { match: "(HK)", strategy: select }
"SG": { match: "(SG)", strategy: select }`,
		),
		Routing:  mustRoutingMap(t, `"proxy": ["HK", "SG", "@all", "DIRECT"]`),
		Fallback: "proxy",
	}

	p, err := Execute(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("Execute() 出错：%v", err)
	}
	if len(p.Proxies) != 2 {
		t.Fatalf("有效节点 = %d，期望 2：%+v", len(p.Proxies), p.Proxies)
	}
	if !containsProxyName(p.Proxies, p.AllProxies, "HK-YAML") || !containsProxyName(p.Proxies, p.AllProxies, "SG-CONF") {
		t.Fatalf("@all = %+v，期望同时包含两个结构化订阅节点", p.AllProxies)
	}
	if containsProxyName(p.Proxies, p.AllProxies, "Traffic Reset") || containsProxyName(p.Proxies, p.AllProxies, "DIRECT") {
		t.Fatalf("@all 混入了已过滤或保留的条目：%+v", p.AllProxies)
	}

	groupMembers := make(map[string][]model.ProxyID, len(p.NodeGroups))
	for _, group := range p.NodeGroups {
		groupMembers[group.Name] = group.Members
	}
	if !containsProxyName(p.Proxies, groupMembers["HK"], "HK-YAML") || !containsProxyName(p.Proxies, groupMembers["SG"], "SG-CONF") {
		t.Fatalf("结构化节点没有参与区域匹配：%+v", groupMembers)
	}
}

// TestExecute_FetchError 验证订阅拉取错误会向上传递。
func TestExecute_FetchError(t *testing.T) {
	f := &fakeFetcher{err: errors.New("connection refused")}

	cfg := &config.Config{
		Sources: config.Sources{
			Subscriptions: []config.Source{{URL: "https://sub.example.com/api"}},
		},
		Routing:  mustRoutingMap(t, `"proxy": ["DIRECT"]`),
		Fallback: "proxy",
	}

	_, err := Execute(context.Background(), cfg, f)
	if err == nil {
		t.Fatal("期望出错，实际为 nil")
	}

	var fetchErr *errtype.FetchError
	if !errors.As(err, &fetchErr) {
		t.Errorf("期望 *errtype.FetchError，实际 %T：%v", err, err)
	}
}

// TestExecute_SnellSourceEndToEnd 验证 Snell 来源能走完
// Source→Filter→Group→Route→ValidateGraph 整条流水线，产出的 Pipeline
// 包含标记为 Type=snell 的 KindSnell 节点，可交给各格式的渲染器处理。
func TestExecute_SnellSourceEndToEnd(t *testing.T) {
	subURL := "https://sub.example.com/api"
	subBody := makeSubResponse(
		"ss://YWVzLTI1Ni1jZmI6cGFzcw@hk.example.com:8388#HK-01",
	)

	snellURL := "https://snell.example.com/nodes.txt"
	snellBody := []byte("HK-Snell = snell, 1.2.3.4, 57891, psk=xxx, version=4, reuse=true\n" +
		"SG-Snell = snell, 5.6.7.8, 8989, psk=yyy, version=4\n")

	f := &fakeFetcher{responses: map[string][]byte{
		subURL:   subBody,
		snellURL: snellBody,
	}}

	cfg := &config.Config{
		Sources: config.Sources{
			Subscriptions: []config.Source{{URL: subURL}},
			Snell:         []config.Source{{URL: snellURL}},
		},
		Groups: mustGroupsMap(t,
			`"HK": { match: "(HK)", strategy: select }
"SG": { match: "(SG)", strategy: select }`,
		),
		Routing: mustRoutingMap(t,
			`"proxy": ["HK", "SG", "DIRECT"]
"final": ["proxy", "DIRECT"]`,
		),
		Fallback: "final",
	}

	p, err := Execute(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	// Pipeline 应带上全部三个节点：1 个 ss + 2 个 snell。
	if len(p.Proxies) != 3 {
		t.Fatalf("得到 %d 个节点，期望 3 个", len(p.Proxies))
	}

	// 按 Kind 计数。
	snellCount, subCount := 0, 0
	for _, px := range p.Proxies {
		switch px.Kind {
		case model.KindSnell:
			snellCount++
			if px.Type != "snell" {
				t.Errorf("KindSnell 节点 %q 的 Type=%q，期望 snell", px.Name, px.Type)
			}
		case model.KindSubscription:
			subCount++
		}
	}
	if snellCount != 2 {
		t.Errorf("snell 节点 = %d，期望 2", snellCount)
	}
	if subCount != 1 {
		t.Errorf("订阅节点 = %d，期望 1", subCount)
	}

	// @all 展开结果包含 snell 节点（它们是原始节点）。
	if len(p.AllProxies) != 3 {
		t.Errorf("AllProxies = %d，期望 3（ss + 2 个 snell）", len(p.AllProxies))
	}

	// HK 节点组应同时包含 HK-01 和 HK-Snell（由正则匹配）。
	var hkGroup *model.NodeGroup
	for i, g := range p.NodeGroups {
		if g.Name == "HK" {
			hkGroup = &p.NodeGroups[i]
			break
		}
	}
	if hkGroup == nil {
		t.Fatal("没有找到 HK 节点组")
	}
	wantMembers := map[string]bool{"HK-01": true, "HK-Snell": true}
	gotMembers := make(map[string]bool)
	for _, m := range hkGroup.Members {
		gotMembers[proxyNameByID(p.Proxies, m)] = true
	}
	for m := range wantMembers {
		if !gotMembers[m] {
			t.Errorf("HK 组缺少成员 %q（实际 %v）", m, hkGroup.Members)
		}
	}
}

// TestExecute_SnellAsRelayThroughUpstream 验证 Snell 节点可以作为自定义节点
// relay_through 链的上游候选。产出的 Pipeline 应包含链式节点 HK-Snell→MY-PROXY，
// 其 DialerID 指向该 Snell 节点的编号；之后 Clash 渲染会把它级联剔除，Surge 则保留它。
// 本测试防止上游候选池被意外收窄回只认 KindSubscription。
func TestExecute_SnellAsRelayThroughUpstream(t *testing.T) {
	snellURL := "https://snell.example.com/nodes.txt"
	snellBody := []byte("HK-Snell = snell, 1.2.3.4, 57891, psk=xxx, version=4\n")

	f := &fakeFetcher{responses: map[string][]byte{snellURL: snellBody}}

	cfg := &config.Config{
		Sources: config.Sources{
			Snell: []config.Source{{URL: snellURL}},
			CustomProxies: []config.CustomProxy{
				customProxy("MY-PROXY", "socks5://u:p@10.0.0.1:1080", &config.RelayThrough{Type: "all", Strategy: "select"}),
			},
		},
		Groups:   mustGroupsMap(t, `"HK": { match: "(HK)", strategy: select }`),
		Routing:  mustRoutingMap(t, `"proxy": ["HK", "MY-PROXY", "DIRECT"]`),
		Fallback: "proxy",
	}

	p, err := Execute(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	// 找到链式节点。
	var chained *model.Proxy
	for i, px := range p.Proxies {
		if px.Kind == model.KindChained {
			chained = &p.Proxies[i]
			break
		}
	}
	if chained == nil {
		t.Fatal("没有找到链式节点；Snell 本应是有效的上游")
	}
	if chained.Name != "HK-Snell→MY-PROXY" {
		t.Errorf("chained.Name = %q，期望 HK-Snell→MY-PROXY", chained.Name)
	}
	if got := proxyNameByID(p.Proxies, chained.DialerID); got != "HK-Snell" {
		t.Errorf("chained.DialerID 解析为 %q，期望 HK-Snell（snell 上游）", got)
	}

	// AllProxies 不得包含链式节点，但必须包含 HK-Snell。
	inAll := map[string]bool{}
	for _, id := range p.AllProxies {
		inAll[proxyNameByID(p.Proxies, id)] = true
	}
	if !inAll["HK-Snell"] {
		t.Error("AllProxies 应包含 HK-Snell（原始节点）")
	}
	if inAll["HK-Snell→MY-PROXY"] {
		t.Error("AllProxies 不得包含链式节点")
	}
}

// TestExecute_SnellFilterAppliesExclude 验证 filters.exclude 移除 Snell 节点的方式
// 与移除订阅节点相同。
func TestExecute_SnellFilterAppliesExclude(t *testing.T) {
	snellURL := "https://snell.example.com/nodes.txt"
	snellBody := []byte("HK-Snell = snell, 1.2.3.4, 57891, psk=xxx, version=4\n" +
		"过期-Snell = snell, 5.6.7.8, 8989, psk=yyy, version=4\n")

	f := &fakeFetcher{responses: map[string][]byte{snellURL: snellBody}}

	cfg := &config.Config{
		Sources: config.Sources{
			Snell: []config.Source{{URL: snellURL}},
		},
		Filters:  config.Filters{Exclude: "(过期)"},
		Groups:   mustGroupsMap(t, `"HK": { match: "(HK)", strategy: select }`),
		Routing:  mustRoutingMap(t, `"proxy": ["HK", "DIRECT"]`),
		Fallback: "proxy",
	}

	p, err := Execute(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	if len(p.Proxies) != 1 {
		t.Fatalf("得到 %d 个节点，期望 1 个（过滤掉一个）", len(p.Proxies))
	}
	if p.Proxies[0].Name != "HK-Snell" {
		t.Errorf("保留下来的节点 = %q，期望 HK-Snell", p.Proxies[0].Name)
	}
}

// TestExecute_FilterExcludes 验证 exclude 过滤会减少节点数量。
func TestExecute_FilterExcludes(t *testing.T) {
	subURL := "https://sub.example.com/api"
	body := makeSubResponse(
		"ss://YWVzLTI1Ni1jZmI6cGFzcw@hk.example.com:8388#HK-01",
		"ss://YWVzLTI1Ni1jZmI6cGFzcw@sg.example.com:8388#SG-01",
		"ss://YWVzLTI1Ni1jZmI6cGFzcw@expire.example.com:8388#过期提醒",
	)

	f := &fakeFetcher{responses: map[string][]byte{subURL: body}}

	cfg := &config.Config{
		Sources: config.Sources{
			Subscriptions: []config.Source{{URL: subURL}},
		},
		Filters:  config.Filters{Exclude: "(过期)"},
		Groups:   mustGroupsMap(t, `"HK": { match: "(HK)", strategy: select }`),
		Routing:  mustRoutingMap(t, `"proxy": ["HK", "DIRECT"]`),
		Fallback: "proxy",
	}

	p, err := Execute(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	// "过期提醒" 节点应被过滤掉。
	if len(p.Proxies) != 2 {
		t.Errorf("得到 %d 个节点，期望 2 个（过滤掉一个）", len(p.Proxies))
	}
	for _, px := range p.Proxies {
		if px.Kind != model.KindSubscription {
			t.Errorf("意外的 kind %q，节点 %q", px.Kind, px.Name)
		}
	}
}

// T-EXECUTE-VLESS-001：订阅 + VLESS 混合来源的端到端测试。
// Pipeline 应带上两类节点；区域组跨类型匹配；未知的 VLESS 传输方式回退为 tcp；
// 非 none 的 encryption 原样保留；FetchOrder（cfg 在内存中构造，这里走默认回退）
// 产出 subscription → snell → vless 的顺序。
func TestExecute_VLessEndToEnd(t *testing.T) {
	subURL := "https://sub.example.com/api"
	subBody := makeSubResponse(
		"ss://YWVzLTI1Ni1jZmI6cGFzcw@hk.example.com:8388#HK-01",
	)

	vlessURL := "https://vless.example.com/nodes.txt"
	vlessBody := []byte(
		"# reality nodes\n" +
			"vless://11111111-2222-3333-4444-555555555555@hk.example.com:443?security=tls&sni=hk.example.com&type=quic&encryption=mlkem768x25519plus.native#HK-VL\n" +
			"vless://11111111-2222-3333-4444-555555555555@sg.example.com:443?security=tls&sni=sg.example.com&type=tcp#SG-VL\n")

	f := &fakeFetcher{responses: map[string][]byte{
		subURL:   subBody,
		vlessURL: vlessBody,
	}}

	cfg := &config.Config{
		Sources: config.Sources{
			Subscriptions: []config.Source{{URL: subURL}},
			VLess:         []config.Source{{URL: vlessURL}},
		},
		Groups: mustGroupsMap(t,
			`"HK": { match: "(HK)", strategy: select }
"SG": { match: "(SG)", strategy: select }`,
		),
		Routing: mustRoutingMap(t,
			`"proxy": ["HK", "SG", "DIRECT"]
"final": ["proxy", "DIRECT"]`,
		),
		Fallback: "final",
	}

	p, err := Execute(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	// 1 个 ss + 2 个 vless
	if len(p.Proxies) != 3 {
		t.Fatalf("得到 %d 个节点，期望 3 个", len(p.Proxies))
	}

	vlessCount, subCount := 0, 0
	var (
		hkVL    model.Proxy
		foundHK bool
	)
	for _, px := range p.Proxies {
		switch px.Kind {
		case model.KindVLess:
			vlessCount++
			if px.Type != "vless" {
				t.Errorf("KindVLess 节点 %q 的 Type=%q，期望 vless", px.Name, px.Type)
			}
			if px.Name == "HK-VL" {
				hkVL = px
				foundHK = true
			}
		case model.KindSubscription:
			subCount++
		}
	}
	if vlessCount != 2 {
		t.Errorf("vless 节点 = %d，期望 2", vlessCount)
	}
	if subCount != 1 {
		t.Errorf("订阅节点 = %d，期望 1", subCount)
	}
	if !foundHK {
		t.Fatal("没有找到 HK-VL 节点")
	}
	if hkVL.Params["network"] != "tcp" {
		t.Errorf("HK-VL network = %q，期望回退为 tcp", hkVL.Params["network"])
	}
	if hkVL.Params["encryption"] != "mlkem768x25519plus.native" {
		t.Errorf("HK-VL encryption = %q，期望原样透传的值", hkVL.Params["encryption"])
	}

	// HK 区域组：HK-01（ss）+ HK-VL（vless）。
	var hk *model.NodeGroup
	for i, g := range p.NodeGroups {
		if g.Name == "HK" {
			hk = &p.NodeGroups[i]
			break
		}
	}
	if hk == nil {
		t.Fatal("没有找到 HK 区域组")
	}
	wantHK := map[string]bool{"HK-01": true, "HK-VL": true}
	gotHK := map[string]bool{}
	for _, m := range hk.Members {
		gotHK[proxyNameByID(p.Proxies, m)] = true
	}
	for m := range wantHK {
		if !gotHK[m] {
			t.Errorf("HK 组缺少 %q（实际 %v）", m, hk.Members)
		}
	}
}

// T-EXECUTE-VLESS-003：VLESS 节点可以作为 relay_through 链式连接的上游
// （与 TestExecute_SnellAsRelayThroughUpstream 对应）。
func TestExecute_VLessAsRelayThroughUpstream(t *testing.T) {
	vlessURL := "https://vless.example.com/nodes.txt"
	vlessBody := []byte(
		"vless://11111111-2222-3333-4444-555555555555@hk.example.com:443?security=tls&sni=hk.example.com&type=tcp#HK-VL\n")

	f := &fakeFetcher{responses: map[string][]byte{vlessURL: vlessBody}}

	cfg := &config.Config{
		Sources: config.Sources{
			VLess: []config.Source{{URL: vlessURL}},
			CustomProxies: []config.CustomProxy{
				customProxy("MY-PROXY", "socks5://u:p@10.0.0.1:1080", &config.RelayThrough{Type: "all", Strategy: "select"}),
			},
		},
		Groups:   mustGroupsMap(t, `"HK": { match: "(HK)", strategy: select }`),
		Routing:  mustRoutingMap(t, `"proxy": ["HK", "MY-PROXY", "DIRECT"]`),
		Fallback: "proxy",
	}

	p, err := Execute(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	var chained *model.Proxy
	for i, px := range p.Proxies {
		if px.Kind == model.KindChained {
			chained = &p.Proxies[i]
			break
		}
	}
	if chained == nil {
		t.Fatal("没有找到链式节点；VLESS 本应是有效的上游")
	}
	if chained.Name != "HK-VL→MY-PROXY" {
		t.Errorf("chained.Name = %q，期望 HK-VL→MY-PROXY", chained.Name)
	}
	if got := proxyNameByID(p.Proxies, chained.DialerID); got != "HK-VL" {
		t.Errorf("chained.DialerID 解析为 %q，期望 HK-VL（vless 上游）", got)
	}

	// @all 包含原始 vless 节点，不包含派生出的链式节点。
	inAll := map[string]bool{}
	for _, id := range p.AllProxies {
		inAll[proxyNameByID(p.Proxies, id)] = true
	}
	if !inAll["HK-VL"] {
		t.Error("AllProxies 应包含 HK-VL（原始节点）")
	}
	if inAll["HK-VL→MY-PROXY"] {
		t.Error("AllProxies 不得包含链式节点")
	}
}

// 端到端防护：用户声明了同名的区域组和链式模板 custom_proxy 时，
// 冲突必须在启动准备阶段暴露，而不是推迟到请求流水线里。
func TestExecute_ChainGroupNameCollidesWithRegionGroup(t *testing.T) {
	subURL := "https://sub.example.com/api"
	body := makeSubResponse(
		"ss://YWVzLTI1Ni1jZmI6cGFzcw@hk.example.com:8388#HK-01",
	)

	f := &fakeFetcher{responses: map[string][]byte{subURL: body}}

	// 区域组和链式模板 cp 都叫 "HK-ISP"——链式模板会尝试创建一个同名节点组，
	// 在 ValidateGraph 处触发重复声明错误。
	cfg := &config.Config{
		Sources: config.Sources{
			Subscriptions: []config.Source{{URL: subURL}},
			CustomProxies: []config.CustomProxy{
				customProxy("HK-ISP", "socks5://1.1.1.1:1080", &config.RelayThrough{Type: "all", Strategy: "select"}),
			},
		},
		Groups:   mustGroupsMap(t, `"HK-ISP": { match: "(HK)", strategy: select }`),
		Routing:  mustRoutingMap(t, `"proxy": ["HK-ISP", "DIRECT"]`),
		Fallback: "proxy",
	}

	_, err := Execute(context.Background(), cfg, f)
	if err == nil {
		t.Fatal("期望重复组错误，实际为 nil")
	}

	var configErr *errtype.ConfigError
	if !errors.As(err, &configErr) {
		t.Fatalf("错误类型 = %T，期望 *errtype.ConfigError", err)
	}
	if !strings.Contains(err.Error(), `名称 "HK-ISP" 同时被 节点组 和 链式组 使用`) {
		t.Errorf("错误应提到启动时的静态名称冲突，实际：%v", err)
	}
}

// T-EXEC-SS-01：SS 自定义节点端到端测试（不带 relay_through）。
//
// 验证 ss 类型的 custom_proxy 走完整条流水线，以 KindCustom 节点出现在
// Proxies 和 AllProxies 中，Params 里带 cipher/password，同时不参与区域组正则匹配。
func TestExecute_SSCustomProxyEndToEnd(t *testing.T) {
	subURL := "https://sub.example.com/api"
	subBody := makeSubResponse(
		"ss://YWVzLTI1Ni1jZmI6cGFzcw@hk.example.com:8388#HK-01",
	)

	f := &fakeFetcher{responses: map[string][]byte{subURL: subBody}}

	cfg := &config.Config{
		Sources: config.Sources{
			Subscriptions: []config.Source{{URL: subURL}},
			CustomProxies: []config.CustomProxy{
				customProxy("MY-SS", "ss://YWVzLTI1Ni1nY206bXlwYXNz@1.2.3.4:8388", nil),
			},
		},
		Groups:   mustGroupsMap(t, `"HK": { match: "(HK)", strategy: select }`),
		Routing:  mustRoutingMap(t, `"proxy": ["HK", "@all", "DIRECT"]`),
		Fallback: "proxy",
	}

	p, err := Execute(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("Execute 出错：%v", err)
	}

	// MY-SS 以 ss 类型的 KindCustom 节点出现在 Proxies 中。
	var mySS *model.Proxy
	for i := range p.Proxies {
		if p.Proxies[i].Name == "MY-SS" {
			mySS = &p.Proxies[i]
			break
		}
	}
	if mySS == nil {
		t.Fatal("Proxies 中缺少 MY-SS")
	}
	if mySS.Type != "ss" {
		t.Errorf("MY-SS Type = %q，期望 ss", mySS.Type)
	}
	if mySS.Kind != model.KindCustom {
		t.Errorf("MY-SS Kind = %q，期望 custom", mySS.Kind)
	}
	if mySS.Params["cipher"] != "aes-256-gcm" || mySS.Params["password"] != "mypass" {
		t.Errorf("MY-SS Params = %v", mySS.Params)
	}

	// MY-SS 出现在 @all 展开结果中。
	foundInAll := false
	for _, id := range p.AllProxies {
		if proxyNameByID(p.Proxies, id) == "MY-SS" {
			foundInAll = true
			break
		}
	}
	if !foundInAll {
		t.Error("AllProxies 中缺少 MY-SS")
	}

	// MY-SS 不出现在 HK 区域组中（KindCustom 不参与区域正则匹配）。
	for _, g := range p.NodeGroups {
		if g.Name != "HK" {
			continue
		}
		for _, m := range g.Members {
			if proxyNameByID(p.Proxies, m) == "MY-SS" {
				t.Errorf("MY-SS 混入了 HK 区域组；KindCustom 不得匹配区域正则")
			}
		}
	}
}

// T-EXEC-SS-02：SS 自定义节点链式连接端到端测试（带 relay_through + plugin）。
//
// 验证带 relay_through 的 ss 类型 custom_proxy 生成的 KindChained 节点从链式模板
// 继承 Type=ss、cipher/password 和 plugin；模板本身不作为独立节点出现；
// 这些链式节点归入以 cp.Name 命名的链式组。
func TestExecute_SSCustomProxyChainedEndToEnd(t *testing.T) {
	subURL := "https://sub.example.com/api"
	subBody := makeSubResponse(
		"ss://YWVzLTI1Ni1jZmI6cGFzcw@hk.example.com:8388#HK-01",
		"ss://YWVzLTI1Ni1jZmI6cGFzcw@hk2.example.com:8388#HK-02",
	)

	f := &fakeFetcher{responses: map[string][]byte{subURL: subBody}}

	cfg := &config.Config{
		Sources: config.Sources{
			Subscriptions: []config.Source{{URL: subURL}},
			CustomProxies: []config.CustomProxy{
				customProxy("SS-Chain", "ss://YWVzLTI1Ni1nY206Y2hhaW5wYXNz@1.2.3.4:8388?plugin=obfs-local%3Bobfs%3Dhttp", &config.RelayThrough{Type: "all", Strategy: "select"}),
			},
		},
		Groups:   mustGroupsMap(t, `"HK": { match: "(HK)", strategy: select }`),
		Routing:  mustRoutingMap(t, `"proxy": ["HK", "SS-Chain", "DIRECT"]`),
		Fallback: "proxy",
	}

	p, err := Execute(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("Execute 出错：%v", err)
	}

	// 链式模板本身不作为独立节点出现。
	for _, px := range p.Proxies {
		if px.Name == "SS-Chain" {
			t.Errorf("SS-Chain 不应作为独立节点存在；它是链式模板")
		}
	}

	// 存在链式节点：HK-01→SS-Chain 和 HK-02→SS-Chain。
	wantChained := map[string]bool{"HK-01→SS-Chain": false, "HK-02→SS-Chain": false}
	for _, px := range p.Proxies {
		if _, want := wantChained[px.Name]; !want {
			continue
		}
		wantChained[px.Name] = true
		if px.Type != "ss" {
			t.Errorf("链式节点 %q Type = %q，期望 ss", px.Name, px.Type)
		}
		if px.Kind != model.KindChained {
			t.Errorf("链式节点 %q Kind = %q，期望 chained", px.Name, px.Kind)
		}
		if px.Params["cipher"] != "aes-256-gcm" || px.Params["password"] != "chainpass" {
			t.Errorf("链式节点 %q Params = %v", px.Name, px.Params)
		}
		if px.Plugin == nil {
			t.Errorf("链式节点 %q 丢失了链式模板的 Plugin", px.Name)
		} else if px.Plugin.Name != "obfs-local" || px.Plugin.Opts["obfs"] != "http" {
			t.Errorf("链式节点 %q Plugin = %+v", px.Name, px.Plugin)
		}
		if px.DialerID == 0 {
			t.Errorf("链式节点 %q 缺少 DialerID", px.Name)
		}
	}
	for name, found := range wantChained {
		if !found {
			t.Errorf("Proxies 中没有找到链式节点 %q", name)
		}
	}

	// NodeGroups 中存在链式组 "SS-Chain"，并包含两个链式成员。
	var chainGroup *model.NodeGroup
	for i := range p.NodeGroups {
		if p.NodeGroups[i].Name == "SS-Chain" {
			chainGroup = &p.NodeGroups[i]
			break
		}
	}
	if chainGroup == nil {
		t.Fatal("NodeGroups 中缺少链式组 SS-Chain")
	}
	if len(chainGroup.Members) != 2 {
		t.Errorf("链式组有 %d 个成员，期望 2 个", len(chainGroup.Members))
	}

	// SS-Chain 不进入 @all 展开结果（链式节点被排除；链式模板不产生独立节点）。
	for _, id := range p.AllProxies {
		name := proxyNameByID(p.Proxies, id)
		if name == "SS-Chain" || strings.Contains(name, "→SS-Chain") {
			t.Errorf("与链式相关的 %q 混入了 @all", name)
		}
	}
}
