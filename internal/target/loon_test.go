package target

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// T-TGT-LOON-001: native Loon nodes survive only their target, with cascade cleanup elsewhere.
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
					t.Fatalf("projection=%+v error=%v, want one chain", got, err)
				}
			} else {
				var targetErr *errtype.TargetError
				if !errors.As(err, &targetErr) || targetErr.Code != tt.wantCode || !strings.Contains(err.Error(), "Loon CONF 专属") {
					t.Fatalf("error=%v, want %s with Loon root", err, tt.wantCode)
				}
			}
		})
	}
}

// T-TGT-LOON-002: unsupported formats and VLESS transports cascade through groups and rules.
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
		t.Fatalf("projection=%+v, want only TCP and surviving groups", got)
	}
	p.Fallback = "DEAD"
	_, err = ForLoon(p)
	var targetErr *errtype.TargetError
	if !errors.As(err, &targetErr) || targetErr.Code != errtype.CodeTargetLoonFallbackEmpty || !strings.Contains(err.Error(), "grpc") {
		t.Fatalf("error=%v, want Loon fallback failure with transport root", err)
	}
}

// T-TGT-LOON-003: helper names avoid nodes and groups; projection does not mutate the input.
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
		t.Fatal("input proxies mutated")
	}
	chain := got.ProxyChains[0]
	if !chain.UDP || chain.Name != "HK→EXIT" || !reflect.DeepEqual(chain.Members, []string{"HK", "HK→EXIT-exit③"}) {
		t.Fatalf("chain=%+v, want collision-free exit name", chain)
	}
	if got.NodeGroups[1].Members[0] != chain.Name {
		t.Fatalf("group=%+v does not refer to chain %q", got.NodeGroups[1], chain.Name)
	}
}

// T-TGT-LOON-004: required VLESS extensions are filtered instead of being silently discarded.
func TestTarget_LoonVLessExtensions(t *testing.T) {
	for _, key := range []string{"encryption", "flow"} {
		t.Run(key, func(t *testing.T) {
			proxy := vlessProxy(1, "VL")
			proxy.Params[key] = "unadapted-value"
			p := &model.Pipeline{Proxies: []model.Proxy{proxy}, NodeGroups: []model.NodeGroup{nodeGroup("N", 1)}, RouteGroups: []model.RouteGroup{routeGroup("OUT", nodePolicy("N"))}, Fallback: "OUT"}
			_, err := ForLoon(p)
			var targetErr *errtype.TargetError
			if !errors.As(err, &targetErr) || targetErr.Code != errtype.CodeTargetLoonFallbackEmpty || !strings.Contains(err.Error(), key) {
				t.Fatalf("error=%v, want %s extension root", err, key)
			}
		})
	}
}
