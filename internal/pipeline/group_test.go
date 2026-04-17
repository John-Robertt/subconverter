package pipeline

import (
	"errors"
	"testing"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
	"gopkg.in/yaml.v3"
)

// --- test helpers ---

func makeSubProxy(name string) model.Proxy {
	return model.Proxy{
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
		t.Fatalf("mustGroupsMap: %v", err)
	}
	return m
}

func mustRoutingMap(t *testing.T, yamlStr string) config.OrderedMap[[]string] {
	t.Helper()
	var m config.OrderedMap[[]string]
	if err := yaml.Unmarshal([]byte(yamlStr), &m); err != nil {
		t.Fatalf("mustRoutingMap: %v", err)
	}
	return m
}

// --- Group tests ---

// T-GRP-001: Region group regex matching
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

	result, err := Group(proxies, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.NodeGroups) != 2 {
		t.Fatalf("got %d node groups, want 2", len(result.NodeGroups))
	}

	// HK group
	hk := result.NodeGroups[0]
	if hk.Name != "🇭🇰 Hong Kong" {
		t.Errorf("group[0].Name = %q, want %q", hk.Name, "🇭🇰 Hong Kong")
	}
	if hk.Scope != model.ScopeNode {
		t.Errorf("group[0].Scope = %q, want %q", hk.Scope, model.ScopeNode)
	}
	if hk.Strategy != "select" {
		t.Errorf("group[0].Strategy = %q, want %q", hk.Strategy, "select")
	}
	wantHK := []string{"HK-01", "HK-02"}
	if len(hk.Members) != len(wantHK) {
		t.Fatalf("HK members = %v, want %v", hk.Members, wantHK)
	}
	for i, want := range wantHK {
		if hk.Members[i] != want {
			t.Errorf("HK.Members[%d] = %q, want %q", i, hk.Members[i], want)
		}
	}

	// SG group
	sg := result.NodeGroups[1]
	if sg.Name != "🇸🇬 Singapore" {
		t.Errorf("group[1].Name = %q, want %q", sg.Name, "🇸🇬 Singapore")
	}
	if sg.Strategy != "url-test" {
		t.Errorf("group[1].Strategy = %q, want %q", sg.Strategy, "url-test")
	}
	if len(sg.Members) != 1 || sg.Members[0] != "SG-01" {
		t.Errorf("SG.Members = %v, want [SG-01]", sg.Members)
	}

	// Custom proxy "MY-PROXY" should NOT appear in any region group.
	for _, g := range result.NodeGroups {
		for _, m := range g.Members {
			if m == "MY-PROXY" {
				t.Errorf("custom proxy MY-PROXY should not be matched by region group %q", g.Name)
			}
		}
	}
}

// T-GRP-002: relay_through type=group
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
				{
					URL:  "socks5://user1:pass1@154.197.1.1:45002",
					Name: "HK-ISP",
					Type: "socks5", Server: "154.197.1.1", Port: 45002,
					Params: map[string]string{"username": "user1", "password": "pass1"},
					RelayThrough: &config.RelayThrough{
						Type:     "group",
						Name:     "🇭🇰 Hong Kong",
						Strategy: "select",
					},
				},
			},
		},
	}

	result, err := Group(proxies, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 2 node groups: region + chain.
	if len(result.NodeGroups) != 2 {
		t.Fatalf("got %d node groups, want 2", len(result.NodeGroups))
	}

	chainGroup := result.NodeGroups[1]
	if chainGroup.Name != "HK-ISP" {
		t.Errorf("chain group name = %q, want %q", chainGroup.Name, "HK-ISP")
	}
	if chainGroup.Scope != model.ScopeNode {
		t.Errorf("chain group scope = %q, want %q", chainGroup.Scope, model.ScopeNode)
	}
	if chainGroup.Strategy != "select" {
		t.Errorf("chain group strategy = %q, want %q", chainGroup.Strategy, "select")
	}

	wantMembers := []string{"HK-01→HK-ISP", "HK-02→HK-ISP"}
	if len(chainGroup.Members) != len(wantMembers) {
		t.Fatalf("chain members = %v, want %v", chainGroup.Members, wantMembers)
	}
	for i, want := range wantMembers {
		if chainGroup.Members[i] != want {
			t.Errorf("chain.Members[%d] = %q, want %q", i, chainGroup.Members[i], want)
		}
	}

	// 2 sub + 2 chained = 4. Source stage drops custom proxies with
	// relay_through (see convertCustomProxies), so HK-ISP never enters the
	// proxy list as a KindCustom entry.
	if len(result.Proxies) != 4 {
		t.Fatalf("got %d proxies, want 4", len(result.Proxies))
	}

	chained := result.Proxies[2] // first chained proxy
	if chained.Name != "HK-01→HK-ISP" {
		t.Errorf("chained.Name = %q, want %q", chained.Name, "HK-01→HK-ISP")
	}
	if chained.Kind != model.KindChained {
		t.Errorf("chained.Kind = %q, want %q", chained.Kind, model.KindChained)
	}
	if chained.Dialer != "HK-01" {
		t.Errorf("chained.Dialer = %q, want %q", chained.Dialer, "HK-01")
	}
	if chained.Type != "socks5" {
		t.Errorf("chained.Type = %q, want %q", chained.Type, "socks5")
	}
	if chained.Server != "154.197.1.1" {
		t.Errorf("chained.Server = %q, want %q", chained.Server, "154.197.1.1")
	}
	if chained.Port != 45002 {
		t.Errorf("chained.Port = %d, want %d", chained.Port, 45002)
	}
}

// T-GRP-003: relay_through type=select
func TestGroup_ChainedTypeSelect(t *testing.T) {
	proxies := []model.Proxy{
		makeSubProxy("HK-01"),
		makeSubProxy("SG-01"),
		makeSubProxy("JP-01"),
		makeCustomProxyModel("PROXY-A"),
	}

	cfg := &config.Config{
		Sources: config.Sources{
			CustomProxies: []config.CustomProxy{
				{
					URL:  "http://10.0.0.1:8080",
					Name: "PROXY-A",
					Type: "http", Server: "10.0.0.1", Port: 8080,
					RelayThrough: &config.RelayThrough{
						Type:     "select",
						Match:    "(HK|SG)",
						Strategy: "url-test",
					},
				},
			},
		},
	}

	result, err := Group(proxies, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.NodeGroups) != 1 {
		t.Fatalf("got %d node groups, want 1 (chain group only)", len(result.NodeGroups))
	}

	chainGroup := result.NodeGroups[0]
	if chainGroup.Strategy != "url-test" {
		t.Errorf("chain group strategy = %q, want %q", chainGroup.Strategy, "url-test")
	}

	wantMembers := []string{"HK-01→PROXY-A", "SG-01→PROXY-A"}
	if len(chainGroup.Members) != len(wantMembers) {
		t.Fatalf("chain members = %v, want %v", chainGroup.Members, wantMembers)
	}
	for i, want := range wantMembers {
		if chainGroup.Members[i] != want {
			t.Errorf("Members[%d] = %q, want %q", i, chainGroup.Members[i], want)
		}
	}
}

// T-GRP-004: relay_through type=all
func TestGroup_ChainedTypeAll(t *testing.T) {
	proxies := []model.Proxy{
		makeSubProxy("HK-01"),
		makeSubProxy("SG-01"),
		makeCustomProxyModel("PROXY-B"),
	}

	cfg := &config.Config{
		Sources: config.Sources{
			CustomProxies: []config.CustomProxy{
				{
					URL:  "socks5://1.1.1.1:1080",
					Name: "PROXY-B",
					Type: "socks5", Server: "1.1.1.1", Port: 1080,
					RelayThrough: &config.RelayThrough{
						Type:     "all",
						Strategy: "select",
					},
				},
			},
		},
	}

	result, err := Group(proxies, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.NodeGroups) != 1 {
		t.Fatalf("got %d node groups, want 1", len(result.NodeGroups))
	}

	wantMembers := []string{"HK-01→PROXY-B", "SG-01→PROXY-B"}
	chainGroup := result.NodeGroups[0]
	if len(chainGroup.Members) != len(wantMembers) {
		t.Fatalf("chain members = %v, want %v", chainGroup.Members, wantMembers)
	}
	for i, want := range wantMembers {
		if chainGroup.Members[i] != want {
			t.Errorf("Members[%d] = %q, want %q", i, chainGroup.Members[i], want)
		}
	}
}

// T-GRP-005: @all excludes chained nodes
func TestGroup_AllProxiesExcludesChained(t *testing.T) {
	proxies := []model.Proxy{
		makeSubProxy("HK-01"),
		makeSubProxy("SG-01"),
		makeCustomProxyModel("MY-PROXY"),
	}

	cfg := &config.Config{
		Sources: config.Sources{
			CustomProxies: []config.CustomProxy{
				{
					URL:  "socks5://1.1.1.1:1080",
					Name: "MY-PROXY",
					Type: "socks5", Server: "1.1.1.1", Port: 1080,
				},
				{
					URL:  "socks5://2.2.2.2:1080",
					Name: "CHAIN-PROXY",
					Type: "socks5", Server: "2.2.2.2", Port: 1080,
					RelayThrough: &config.RelayThrough{
						Type:     "all",
						Strategy: "select",
					},
				},
			},
		},
	}

	result, err := Group(proxies, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// AllProxies should contain subscription + custom, but NOT chained.
	wantAll := []string{"HK-01", "SG-01", "MY-PROXY"}
	if len(result.AllProxies) != len(wantAll) {
		t.Fatalf("AllProxies = %v, want %v", result.AllProxies, wantAll)
	}
	for i, want := range wantAll {
		if result.AllProxies[i] != want {
			t.Errorf("AllProxies[%d] = %q, want %q", i, result.AllProxies[i], want)
		}
	}

	// Chained proxies should NOT be in AllProxies.
	allSet := make(map[string]bool)
	for _, name := range result.AllProxies {
		allSet[name] = true
	}
	for _, p := range result.Proxies {
		if p.Kind == model.KindChained && allSet[p.Name] {
			t.Errorf("chained proxy %q should not be in AllProxies", p.Name)
		}
	}
}

// T-GRP-006: relay_through type=group referencing non-existent group
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
				{
					URL:  "socks5://1.1.1.1:1080",
					Name: "PROXY-X",
					Type: "socks5", Server: "1.1.1.1", Port: 1080,
					RelayThrough: &config.RelayThrough{
						Type:     "group",
						Name:     "🇰🇷 Korea",
						Strategy: "select",
					},
				},
			},
		},
	}

	_, err := Group(proxies, cfg)
	if err == nil {
		t.Fatal("expected error for non-existent group reference")
	}

	var buildErr *errtype.BuildError
	if !errors.As(err, &buildErr) {
		t.Fatalf("error type = %T, want *errtype.BuildError", err)
	}
	if buildErr.Phase != "group" {
		t.Errorf("Phase = %q, want %q", buildErr.Phase, "group")
	}
}

// T-GRP-007: No custom proxies with relay_through
func TestGroup_NoChaining(t *testing.T) {
	proxies := []model.Proxy{
		makeSubProxy("HK-01"),
		makeSubProxy("SG-01"),
		makeCustomProxyModel("MY-PROXY"),
	}

	cfg := &config.Config{
		Groups: mustGroupsMap(t, `
"🇭🇰 Hong Kong": { match: "(HK)", strategy: select }
`),
		Sources: config.Sources{
			CustomProxies: []config.CustomProxy{
				{URL: "socks5://1.1.1.1:1080", Name: "MY-PROXY", Type: "socks5", Server: "1.1.1.1", Port: 1080},
			},
		},
	}

	result, err := Group(proxies, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only 1 region group, no chain groups.
	if len(result.NodeGroups) != 1 {
		t.Fatalf("got %d node groups, want 1", len(result.NodeGroups))
	}
	if result.NodeGroups[0].Name != "🇭🇰 Hong Kong" {
		t.Errorf("group name = %q, want region group", result.NodeGroups[0].Name)
	}

	// Proxies unchanged (no chained added).
	if len(result.Proxies) != 3 {
		t.Errorf("got %d proxies, want 3 (no chained)", len(result.Proxies))
	}
}

// T-GRP-008: Multiple custom proxies with relay_through
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
				{
					URL: "socks5://1.1.1.1:1080", Name: "ISP-A",
					Type: "socks5", Server: "1.1.1.1", Port: 1080,
					RelayThrough: &config.RelayThrough{Type: "group", Name: "🇭🇰 Hong Kong", Strategy: "select"},
				},
				{
					URL: "http://2.2.2.2:8080", Name: "ISP-B",
					Type: "http", Server: "2.2.2.2", Port: 8080,
					RelayThrough: &config.RelayThrough{Type: "all", Strategy: "url-test"},
				},
			},
		},
	}

	result, err := Group(proxies, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1 region group + 2 chain groups = 3.
	if len(result.NodeGroups) != 3 {
		t.Fatalf("got %d node groups, want 3", len(result.NodeGroups))
	}

	// Chain groups follow region groups, in custom proxy declaration order.
	if result.NodeGroups[1].Name != "ISP-A" {
		t.Errorf("group[1].Name = %q, want %q", result.NodeGroups[1].Name, "ISP-A")
	}
	if result.NodeGroups[2].Name != "ISP-B" {
		t.Errorf("group[2].Name = %q, want %q", result.NodeGroups[2].Name, "ISP-B")
	}

	// ISP-A chains through HK group (1 member: HK-01).
	if len(result.NodeGroups[1].Members) != 1 {
		t.Errorf("ISP-A chain members = %v, want 1 member", result.NodeGroups[1].Members)
	}
	// ISP-B chains through all (2 members: HK-01, SG-01).
	if len(result.NodeGroups[2].Members) != 2 {
		t.Errorf("ISP-B chain members = %v, want 2 members", result.NodeGroups[2].Members)
	}
}

// T-GRP-009: Chained node properties (username/password)
func TestGroup_ChainedNodeProperties(t *testing.T) {
	proxies := []model.Proxy{
		makeSubProxy("HK-01"),
	}

	cfg := &config.Config{
		Sources: config.Sources{
			CustomProxies: []config.CustomProxy{
				{
					URL:  "socks5://admin:secret@10.0.0.1:9090",
					Name: "AUTH-PROXY",
					Type: "socks5", Server: "10.0.0.1", Port: 9090,
					Params: map[string]string{"username": "admin", "password": "secret"},
					RelayThrough: &config.RelayThrough{
						Type:     "all",
						Strategy: "select",
					},
				},
			},
		},
	}

	result, err := Group(proxies, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the chained proxy.
	var chained *model.Proxy
	for i := range result.Proxies {
		if result.Proxies[i].Kind == model.KindChained {
			chained = &result.Proxies[i]
			break
		}
	}
	if chained == nil {
		t.Fatal("no chained proxy found")
	}

	if chained.Type != "socks5" {
		t.Errorf("Type = %q, want socks5", chained.Type)
	}
	if chained.Server != "10.0.0.1" {
		t.Errorf("Server = %q, want 10.0.0.1", chained.Server)
	}
	if chained.Port != 9090 {
		t.Errorf("Port = %d, want 9090", chained.Port)
	}
	if chained.Params["username"] != "admin" {
		t.Errorf("username = %q, want admin", chained.Params["username"])
	}
	if chained.Params["password"] != "secret" {
		t.Errorf("password = %q, want secret", chained.Params["password"])
	}
	if chained.Dialer != "HK-01" {
		t.Errorf("Dialer = %q, want HK-01", chained.Dialer)
	}
}

// T-GRP-SS-01: SS chain template plugin propagation to chained nodes.
//
// Verifies that an ss-typed custom_proxy with a plugin propagates the plugin
// into every chained node derived from it. Without explicit propagation,
// chained ss nodes would silently drop their obfs/v2ray-plugin spec.
func TestGroup_ChainedSSNodePluginPropagation(t *testing.T) {
	proxies := []model.Proxy{
		makeSubProxy("HK-01"),
		makeSubProxy("SG-01"),
	}

	cfg := &config.Config{
		Sources: config.Sources{
			CustomProxies: []config.CustomProxy{
				{
					URL:  "ss://YWVzLTI1Ni1nY206Y2hhaW5wYXNz@1.2.3.4:8388?plugin=obfs-local%3Bobfs%3Dhttp",
					Name: "SS-Chain",
					Type: "ss", Server: "1.2.3.4", Port: 8388,
					Params: map[string]string{"cipher": "aes-256-gcm", "password": "chainpass"},
					Plugin: &model.Plugin{
						Name: "obfs-local",
						Opts: map[string]string{"obfs": "http", "obfs-host": "example.com"},
					},
					RelayThrough: &config.RelayThrough{Type: "all", Strategy: "select"},
				},
			},
		},
	}

	result, err := Group(proxies, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	chainedCount := 0
	for _, px := range result.Proxies {
		if px.Kind != model.KindChained {
			continue
		}
		chainedCount++
		if px.Type != "ss" {
			t.Errorf("chained %q Type = %q, want ss", px.Name, px.Type)
		}
		if px.Params["cipher"] != "aes-256-gcm" || px.Params["password"] != "chainpass" {
			t.Errorf("chained %q Params = %v", px.Name, px.Params)
		}
		if _, ok := px.Params["username"]; ok {
			t.Errorf("chained %q ss node should not carry username in Params", px.Name)
		}
		if px.Plugin == nil {
			t.Errorf("chained %q dropped Plugin from chain template", px.Name)
			continue
		}
		if px.Plugin.Name != "obfs-local" {
			t.Errorf("chained %q Plugin.Name = %q", px.Name, px.Plugin.Name)
		}
		if px.Plugin.Opts["obfs"] != "http" || px.Plugin.Opts["obfs-host"] != "example.com" {
			t.Errorf("chained %q Plugin.Opts = %v", px.Name, px.Plugin.Opts)
		}
	}
	if chainedCount != 2 {
		t.Errorf("got %d chained nodes, want 2 (one per upstream)", chainedCount)
	}
}

// T-GRP-010: Proxies merge order (original first, then chained)
func TestGroup_ProxiesMergeOrder(t *testing.T) {
	proxies := []model.Proxy{
		makeSubProxy("HK-01"),
		makeCustomProxyModel("MY-PROXY"),
	}

	cfg := &config.Config{
		Sources: config.Sources{
			CustomProxies: []config.CustomProxy{
				{
					URL: "socks5://1.1.1.1:1080", Name: "MY-PROXY",
					Type: "socks5", Server: "1.1.1.1", Port: 1080,
					RelayThrough: &config.RelayThrough{Type: "all", Strategy: "select"},
				},
			},
		},
	}

	result, err := Group(proxies, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Proxies) != 3 {
		t.Fatalf("got %d proxies, want 3", len(result.Proxies))
	}

	// Original proxies first.
	if result.Proxies[0].Name != "HK-01" || result.Proxies[0].Kind != model.KindSubscription {
		t.Errorf("proxy[0] = %q (%s), want HK-01 (subscription)", result.Proxies[0].Name, result.Proxies[0].Kind)
	}
	if result.Proxies[1].Name != "MY-PROXY" || result.Proxies[1].Kind != model.KindCustom {
		t.Errorf("proxy[1] = %q (%s), want MY-PROXY (custom)", result.Proxies[1].Name, result.Proxies[1].Kind)
	}
	// Chained proxy last.
	if result.Proxies[2].Kind != model.KindChained {
		t.Errorf("proxy[2].Kind = %q, want chained", result.Proxies[2].Kind)
	}
}

// T-GRP-SNELL-001: Snell nodes participate in region group regex matching
// alongside subscription nodes. Guards against a future change that narrows
// isFetchedKind/fetchedProxies back to KindSubscription only.
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

	result, err := Group(proxies, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// HK group should contain HK-01 (sub) and HK-Snell (snell).
	hk := result.NodeGroups[0]
	if hk.Name != "HK" {
		t.Fatalf("group[0].Name = %q, want HK", hk.Name)
	}
	gotHK := map[string]bool{}
	for _, m := range hk.Members {
		gotHK[m] = true
	}
	if !gotHK["HK-01"] || !gotHK["HK-Snell"] {
		t.Errorf("HK members = %v, want both HK-01 and HK-Snell", hk.Members)
	}

	// SG group should contain SG-Snell only.
	sg := result.NodeGroups[1]
	if len(sg.Members) != 1 || sg.Members[0] != "SG-Snell" {
		t.Errorf("SG members = %v, want [SG-Snell]", sg.Members)
	}

	// Custom proxy must not appear in any region group.
	for _, g := range result.NodeGroups {
		for _, m := range g.Members {
			if m == "MY-PROXY" {
				t.Errorf("custom proxy MY-PROXY leaked into region group %q", g.Name)
			}
		}
	}

	// @all expansion includes snell nodes.
	gotAll := map[string]bool{}
	for _, n := range result.AllProxies {
		gotAll[n] = true
	}
	for _, want := range []string{"HK-01", "HK-Snell", "SG-Snell", "MY-PROXY"} {
		if !gotAll[want] {
			t.Errorf("AllProxies missing %q (got %v)", want, result.AllProxies)
		}
	}
}

// T-GROUP-VLESS-001 + T-GROUP-VLESS-003: VLESS nodes participate in region
// group regex matching and are included in @all (mirror Snell behaviour).
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

	result, err := Group(proxies, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// HK group should contain HK-01 (sub) and HK-VL (vless).
	hk := result.NodeGroups[0]
	if hk.Name != "HK" {
		t.Fatalf("group[0].Name = %q, want HK", hk.Name)
	}
	gotHK := map[string]bool{}
	for _, m := range hk.Members {
		gotHK[m] = true
	}
	if !gotHK["HK-01"] || !gotHK["HK-VL"] {
		t.Errorf("HK members = %v, want both HK-01 and HK-VL", hk.Members)
	}

	// SG group should contain SG-VL only.
	sg := result.NodeGroups[1]
	if len(sg.Members) != 1 || sg.Members[0] != "SG-VL" {
		t.Errorf("SG members = %v, want [SG-VL]", sg.Members)
	}

	// Custom proxy must not appear in any region group.
	for _, g := range result.NodeGroups {
		for _, m := range g.Members {
			if m == "MY-PROXY" {
				t.Errorf("custom proxy MY-PROXY leaked into region group %q", g.Name)
			}
		}
	}

	// @all expansion includes VLESS nodes.
	gotAll := map[string]bool{}
	for _, n := range result.AllProxies {
		gotAll[n] = true
	}
	for _, want := range []string{"HK-01", "HK-VL", "SG-VL", "MY-PROXY"} {
		if !gotAll[want] {
			t.Errorf("AllProxies missing %q (got %v)", want, result.AllProxies)
		}
	}
}

// T-GROUP-VLESS-002: VLESS nodes are valid upstreams for relay_through
// chains. Custom proxies with relay_through{type:all} produce chained
// proxies whose Dialer points at the VLESS node's name.
func TestGroup_VLessEligibleAsChainUpstream(t *testing.T) {
	proxies := []model.Proxy{
		{Name: "HK-VL", Type: "vless", Server: "hk.example.com", Port: 443,
			Params: map[string]string{"uuid": "11111111-2222-3333-4444-555555555555", "security": "tls", "network": "tcp"},
			Kind:   model.KindVLess},
	}

	cfg := &config.Config{
		Sources: config.Sources{
			CustomProxies: []config.CustomProxy{{
				URL:  "socks5://127.0.0.1:1080",
				Name: "MY-CHAIN",
				Type: "socks5", Server: "127.0.0.1", Port: 1080,
				RelayThrough: &config.RelayThrough{
					Type:     "all",
					Strategy: "select",
				},
			}},
		},
	}

	result, err := Group(proxies, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expect exactly one chained proxy with Dialer=HK-VL.
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
		t.Fatalf("no chained proxy generated; Proxies=%v", result.Proxies)
	}
	if chained.Dialer != "HK-VL" {
		t.Errorf("chained.Dialer = %q, want HK-VL", chained.Dialer)
	}
	if chained.Name != "HK-VL→MY-CHAIN" {
		t.Errorf("chained.Name = %q, want HK-VL→MY-CHAIN", chained.Name)
	}
}

// Chain group name is the custom proxy name verbatim — no system-injected
// prefix. Guards against regressions that would reintroduce a 🔗 (or any
// other) hard-coded marker in front of cp.Name.
func TestGroup_ChainedGroupNameEqualsCustomProxyName(t *testing.T) {
	proxies := []model.Proxy{makeSubProxy("HK-01")}

	cfg := &config.Config{
		Sources: config.Sources{
			CustomProxies: []config.CustomProxy{{
				URL: "socks5://1.1.1.1:1080", Name: "HK-ISP",
				Type: "socks5", Server: "1.1.1.1", Port: 1080,
				RelayThrough: &config.RelayThrough{Type: "all", Strategy: "select"},
			}},
		},
	}

	result, err := Group(proxies, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.NodeGroups) != 1 {
		t.Fatalf("got %d node groups, want 1", len(result.NodeGroups))
	}
	if got := result.NodeGroups[0].Name; got != "HK-ISP" {
		t.Errorf("chain group name = %q, want %q (no prefix)", got, "HK-ISP")
	}
}

// A user-supplied emoji prefix in cp.Name must flow through verbatim to the
// chain group name — the system never adds or strips visual markers. This
// is the positive companion to the regression guard above: users who *want*
// a 🔗 (or any other decoration) get exactly what they wrote.
//
// Built in Go directly (not YAML) to sidestep the astral-plane escape issue
// flagged in CLAUDE.md §"YAML 断言避坑".
func TestGroup_ChainedGroupNamePreservesUserEmojiPrefix(t *testing.T) {
	proxies := []model.Proxy{makeSubProxy("HK-01")}

	cfg := &config.Config{
		Sources: config.Sources{
			CustomProxies: []config.CustomProxy{{
				URL: "socks5://1.1.1.1:1080", Name: "🔗 HK-ISP",
				Type: "socks5", Server: "1.1.1.1", Port: 1080,
				RelayThrough: &config.RelayThrough{Type: "all", Strategy: "select"},
			}},
		},
	}

	result, err := Group(proxies, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.NodeGroups) != 1 {
		t.Fatalf("got %d node groups, want 1", len(result.NodeGroups))
	}
	if got := result.NodeGroups[0].Name; got != "🔗 HK-ISP" {
		t.Errorf("chain group name = %q, want %q (user prefix preserved)", got, "🔗 HK-ISP")
	}
	// The chained proxy name still uses cp.Name verbatim after the "→".
	wantChainedName := "HK-01→🔗 HK-ISP"
	found := false
	for _, p := range result.Proxies {
		if p.Kind == model.KindChained && p.Name == wantChainedName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("chained proxy %q not found in proxies; got %+v", wantChainedName, result.Proxies)
	}
}
