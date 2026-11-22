package target

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// T-TGT-LOON-001：Loon 原生节点只在 Loon 输出中保留，在其他格式中被剔除并级联清理。
func TestTarget_LoonNativeIsolation(t *testing.T) {
	for _, tt := range []struct {
		name     string
		project  func(*model.Pipeline) (*model.TargetPipeline, error)
		wantCode errtype.Code
	}{
		{"clash", ForClash, errtype.CodeTargetClashFallbackEmpty},
		{"surge", ForSurge, errtype.CodeTargetSurgeFallbackEmpty},
		{"loon", ForLoon, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := &model.Pipeline{
				Proxies:     []model.Proxy{nativeProxy(1, "HK", model.NativeProxyFormatLoon), chainedProxy(2, "CHAIN", 1, "EXIT")},
				NodeGroups:  []model.NodeGroup{nodeGroup("NODES", 2)},
				RouteGroups: []model.RouteGroup{routeGroup("OUT", nodePolicy("NODES"))}, Fallback: "OUT",
			}
			got, err := tt.project(p)
			if tt.wantCode == "" {
				if err != nil || len(got.ProxyChains) != 1 {
					t.Fatalf("projection=%+v error=%v，期望恰好一条链路", got, err)
				}
			} else {
				var targetErr *errtype.TargetError
				if !errors.As(err, &targetErr) || targetErr.Code != tt.wantCode || !strings.Contains(err.Error(), "Loon CONF 专属") {
					t.Fatalf("error=%v，期望 %s 且根因为 Loon 原生节点", err, tt.wantCode)
				}
			}
		})
	}
}

// T-TGT-LOON-002：不支持的格式和 VLESS 传输方式被剔除后，影响级联到分组和规则。
func TestTarget_LoonFiltersUnsupportedProxies(t *testing.T) {
	grpc := vlessProxy(4, "GRPC")
	grpc.Params["network"] = "grpc"
	p := &model.Pipeline{
		Proxies:     []model.Proxy{snellProxy(1, "SNELL"), nativeProxy(2, "SURGE", model.NativeProxyFormatSurge), nativeProxy(3, "CLASH", model.NativeProxyFormatClash), grpc, vlessProxy(5, "TCP")},
		NodeGroups:  []model.NodeGroup{nodeGroup("DROP", 1, 2, 3, 4), nodeGroup("KEEP", 5)},
		RouteGroups: []model.RouteGroup{routeGroup("DEAD", nodePolicy("DROP")), routeGroup("OUT", routePolicy("DEAD"), nodePolicy("KEEP"))},
		Rulesets:    []model.Ruleset{{Policy: "DEAD", URLs: []string{"https://example.com/dead"}}},
		Rules:       []model.Rule{{Raw: "DOMAIN,dead.example.com,DEAD", Policy: "DEAD"}}, Fallback: "OUT",
	}
	got, err := ForLoon(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Proxies) != 1 || got.Proxies[0].Name != "TCP" || len(got.NodeGroups) != 1 || len(got.RouteGroups) != 1 || len(got.Rules) != 0 || len(got.Rulesets) != 0 {
		t.Fatalf("projection=%+v，期望只剩 TCP 节点和保留下来的分组", got)
	}
	p.Fallback = "DEAD"
	_, err = ForLoon(p)
	var targetErr *errtype.TargetError
	if !errors.As(err, &targetErr) || targetErr.Code != errtype.CodeTargetLoonFallbackEmpty || !strings.Contains(err.Error(), "grpc") {
		t.Fatalf("error=%v，期望 Loon fallback 失败且根因为传输方式", err)
	}
}

// T-TGT-LOON-003：辅助名字避开已有节点和分组；投影不修改输入。
func TestTarget_LoonChainNames(t *testing.T) {
	p := &model.Pipeline{
		Proxies:     []model.Proxy{ssProxy(1, "HK"), chainedProxy(2, "CHAIN", 1, "EXIT"), ssProxy(3, "HK→EXIT-exit")},
		NodeGroups:  []model.NodeGroup{nodeGroup("HK→EXIT-exit②", 1), nodeGroup("CHAIN", 2)},
		RouteGroups: []model.RouteGroup{routeGroup("OUT", nodePolicy("CHAIN"))}, Fallback: "OUT",
	}
	p.Proxies[1].Params = map[string]string{"udp-relay": "true"}
	before := append([]model.Proxy(nil), p.Proxies...)
	got, err := ForLoon(p)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Proxies, before) {
		t.Fatal("输入的 proxies 被修改了")
	}
	chain := got.ProxyChains[0]
	if !chain.UDP || chain.Name != "HK→EXIT" || !reflect.DeepEqual(chain.Members, []string{"HK", "HK→EXIT-exit③"}) {
		t.Fatalf("chain=%+v，期望出口名不与其他名字冲突", chain)
	}
	if got.NodeGroups[1].Members[0] != chain.Name {
		t.Fatalf("group=%+v 没有引用链路 %q", got.NodeGroups[1], chain.Name)
	}
}

// T-TGT-LOON-004：带有必需 VLESS 扩展的节点会被过滤，而不是悄悄丢掉扩展。
func TestTarget_LoonVLessExtensions(t *testing.T) {
	for _, key := range []string{"encryption", "flow"} {
		t.Run(key, func(t *testing.T) {
			proxy := vlessProxy(1, "VL")
			proxy.Params[key] = "unadapted-value"
			p := &model.Pipeline{Proxies: []model.Proxy{proxy}, NodeGroups: []model.NodeGroup{nodeGroup("N", 1)}, RouteGroups: []model.RouteGroup{routeGroup("OUT", nodePolicy("N"))}, Fallback: "OUT"}
			_, err := ForLoon(p)
			var targetErr *errtype.TargetError
			if !errors.As(err, &targetErr) || targetErr.Code != errtype.CodeTargetLoonFallbackEmpty || !strings.Contains(err.Error(), key) {
				t.Fatalf("error=%v，期望根因为 %s 扩展", err, key)
			}
		})
	}
}
