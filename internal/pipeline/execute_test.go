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

// TestExecute_HappyPath verifies the full pipeline produces a valid Pipeline.
func TestExecute_HappyPath(t *testing.T) {
	subURL := "https://sub.example.com/api"
	body := makeSubResponse(
		"ss://YWVzLTI1Ni1jZmI6cGFzcw@hk.example.com:8388#HK-01",
		"ss://YWVzLTI1Ni1jZmI6cGFzcw@sg.example.com:8388#SG-01",
	)

	f := &fakeFetcher{responses: map[string][]byte{subURL: body}}

	cfg := &config.Config{
		Sources: config.Sources{
			Subscriptions: []config.Subscription{{URL: subURL}},
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
		t.Fatalf("unexpected error: %v", err)
	}

	// Pipeline should contain the parsed proxies.
	if len(p.Proxies) != 2 {
		t.Errorf("got %d proxies, want 2", len(p.Proxies))
	}

	// Should have 1 node group (HK) and 1 route group (proxy).
	if len(p.NodeGroups) != 1 {
		t.Errorf("got %d node groups, want 1", len(p.NodeGroups))
	}
	if len(p.RouteGroups) != 1 {
		t.Errorf("got %d route groups, want 1", len(p.RouteGroups))
	}

	// Fallback recorded.
	if p.Fallback != "proxy" {
		t.Errorf("fallback = %q, want %q", p.Fallback, "proxy")
	}

	// @all should contain original proxies only.
	if len(p.AllProxies) != 2 {
		t.Errorf("AllProxies = %d, want 2", len(p.AllProxies))
	}
}

// TestExecute_FetchError verifies that a subscription fetch error propagates.
func TestExecute_FetchError(t *testing.T) {
	f := &fakeFetcher{err: errors.New("connection refused")}

	cfg := &config.Config{
		Sources: config.Sources{
			Subscriptions: []config.Subscription{{URL: "https://sub.example.com/api"}},
		},
	}

	_, err := Execute(context.Background(), cfg, f)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var fetchErr *errtype.FetchError
	if !errors.As(err, &fetchErr) {
		t.Errorf("expected *errtype.FetchError, got %T: %v", err, err)
	}
}

// TestExecute_SnellSourceEndToEnd verifies that a Snell source flows through
// the full Source→Filter→Group→Route→ValidateGraph pipeline and the resulting
// Pipeline contains KindSnell proxies tagged with Type=snell, ready for the
// format-specific renderers to handle.
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
			Subscriptions: []config.Subscription{{URL: subURL}},
			Snell:         []config.SnellSource{{URL: snellURL}},
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
		t.Fatalf("unexpected error: %v", err)
	}

	// Pipeline should carry all three proxies: 1 ss + 2 snell.
	if len(p.Proxies) != 3 {
		t.Fatalf("got %d proxies, want 3", len(p.Proxies))
	}

	// Count by Kind.
	snellCount, subCount := 0, 0
	for _, px := range p.Proxies {
		switch px.Kind {
		case model.KindSnell:
			snellCount++
			if px.Type != "snell" {
				t.Errorf("KindSnell proxy %q has Type=%q, want snell", px.Name, px.Type)
			}
		case model.KindSubscription:
			subCount++
		}
	}
	if snellCount != 2 {
		t.Errorf("snell proxies = %d, want 2", snellCount)
	}
	if subCount != 1 {
		t.Errorf("subscription proxies = %d, want 1", subCount)
	}

	// @all expansion includes snell nodes (they are original nodes).
	if len(p.AllProxies) != 3 {
		t.Errorf("AllProxies = %d, want 3 (ss + 2 snell)", len(p.AllProxies))
	}

	// HK node group should contain both HK-01 and HK-Snell (regex-matched).
	var hkGroup *model.ProxyGroup
	for i, g := range p.NodeGroups {
		if g.Name == "HK" {
			hkGroup = &p.NodeGroups[i]
			break
		}
	}
	if hkGroup == nil {
		t.Fatal("HK node group not found")
	}
	wantMembers := map[string]bool{"HK-01": true, "HK-Snell": true}
	gotMembers := make(map[string]bool)
	for _, m := range hkGroup.Members {
		gotMembers[m] = true
	}
	for m := range wantMembers {
		if !gotMembers[m] {
			t.Errorf("HK group missing member %q (got %v)", m, hkGroup.Members)
		}
	}
}

// TestExecute_SnellAsRelayThroughUpstream verifies that Snell nodes are valid
// upstream candidates for a custom proxy's relay_through chain. The resulting
// Pipeline should contain a HK-Snell→MY-PROXY chained node with Dialer set
// to the Snell node's name; Clash rendering later cascades it out, while
// Surge includes it. This test guards against accidentally narrowing the
// upstream pool back to KindSubscription only.
func TestExecute_SnellAsRelayThroughUpstream(t *testing.T) {
	snellURL := "https://snell.example.com/nodes.txt"
	snellBody := []byte("HK-Snell = snell, 1.2.3.4, 57891, psk=xxx, version=4\n")

	f := &fakeFetcher{responses: map[string][]byte{snellURL: snellBody}}

	cfg := &config.Config{
		Sources: config.Sources{
			Snell: []config.SnellSource{{URL: snellURL}},
			CustomProxies: []config.CustomProxy{{
				URL: "socks5://u:p@10.0.0.1:1080", Name: "MY-PROXY",
				Type: "socks5", Server: "10.0.0.1", Port: 1080,
				Params:       map[string]string{"username": "u", "password": "p"},
				RelayThrough: &config.RelayThrough{Type: "all", Strategy: "select"},
			}},
		},
		Groups:   mustGroupsMap(t, `"HK": { match: "(HK)", strategy: select }`),
		Routing:  mustRoutingMap(t, `"proxy": ["HK", "MY-PROXY", "DIRECT"]`),
		Fallback: "proxy",
	}

	p, err := Execute(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the chained proxy.
	var chained *model.Proxy
	for i, px := range p.Proxies {
		if px.Kind == model.KindChained {
			chained = &p.Proxies[i]
			break
		}
	}
	if chained == nil {
		t.Fatal("no chained proxy found; Snell should have been a valid upstream")
	}
	if chained.Name != "HK-Snell→MY-PROXY" {
		t.Errorf("chained.Name = %q, want HK-Snell→MY-PROXY", chained.Name)
	}
	if chained.Dialer != "HK-Snell" {
		t.Errorf("chained.Dialer = %q, want HK-Snell (the snell upstream)", chained.Dialer)
	}

	// AllProxies must NOT include the chained node but MUST include HK-Snell.
	inAll := map[string]bool{}
	for _, n := range p.AllProxies {
		inAll[n] = true
	}
	if !inAll["HK-Snell"] {
		t.Error("AllProxies should include HK-Snell (original node)")
	}
	if inAll["HK-Snell→MY-PROXY"] {
		t.Error("AllProxies must not include chained node")
	}
}

// TestExecute_SnellFilterAppliesExclude verifies that filters.exclude removes
// Snell nodes the same way it removes subscription nodes.
func TestExecute_SnellFilterAppliesExclude(t *testing.T) {
	snellURL := "https://snell.example.com/nodes.txt"
	snellBody := []byte("HK-Snell = snell, 1.2.3.4, 57891, psk=xxx, version=4\n" +
		"过期-Snell = snell, 5.6.7.8, 8989, psk=yyy, version=4\n")

	f := &fakeFetcher{responses: map[string][]byte{snellURL: snellBody}}

	cfg := &config.Config{
		Sources: config.Sources{
			Snell: []config.SnellSource{{URL: snellURL}},
		},
		Filters:  config.Filters{Exclude: "(过期)"},
		Groups:   mustGroupsMap(t, `"HK": { match: "(HK)", strategy: select }`),
		Routing:  mustRoutingMap(t, `"proxy": ["HK", "DIRECT"]`),
		Fallback: "proxy",
	}

	p, err := Execute(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(p.Proxies) != 1 {
		t.Fatalf("got %d proxies, want 1 (one filtered)", len(p.Proxies))
	}
	if p.Proxies[0].Name != "HK-Snell" {
		t.Errorf("surviving proxy = %q, want HK-Snell", p.Proxies[0].Name)
	}
}

// TestExecute_FilterExcludes verifies that exclude filter reduces proxy count.
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
			Subscriptions: []config.Subscription{{URL: subURL}},
		},
		Filters:  config.Filters{Exclude: "(过期)"},
		Groups:   mustGroupsMap(t, `"HK": { match: "(HK)", strategy: select }`),
		Routing:  mustRoutingMap(t, `"proxy": ["HK", "DIRECT"]`),
		Fallback: "proxy",
	}

	p, err := Execute(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The "过期提醒" node should be filtered out.
	if len(p.Proxies) != 2 {
		t.Errorf("got %d proxies, want 2 (one filtered)", len(p.Proxies))
	}
	for _, px := range p.Proxies {
		if px.Kind != model.KindSubscription {
			t.Errorf("unexpected kind %q for %q", px.Kind, px.Name)
		}
	}
}

// T-EXECUTE-VLESS-001: Mixed subscription + VLESS source end-to-end.
// Pipeline should carry both kinds; region groups match across kinds; unknown
// VLESS transport falls back to tcp; non-none encryption is preserved; and
// FetchOrder (default fallback here since cfg built in-memory) produces
// subscription → snell → vless ordering.
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
			Subscriptions: []config.Subscription{{URL: subURL}},
			VLess:         []config.VLessSource{{URL: vlessURL}},
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
		t.Fatalf("unexpected error: %v", err)
	}

	// 1 ss + 2 vless
	if len(p.Proxies) != 3 {
		t.Fatalf("got %d proxies, want 3", len(p.Proxies))
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
				t.Errorf("KindVLess proxy %q has Type=%q, want vless", px.Name, px.Type)
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
		t.Errorf("vless proxies = %d, want 2", vlessCount)
	}
	if subCount != 1 {
		t.Errorf("subscription proxies = %d, want 1", subCount)
	}
	if !foundHK {
		t.Fatal("HK-VL proxy not found")
	}
	if hkVL.Params["network"] != "tcp" {
		t.Errorf("HK-VL network = %q, want tcp fallback", hkVL.Params["network"])
	}
	if hkVL.Params["encryption"] != "mlkem768x25519plus.native" {
		t.Errorf("HK-VL encryption = %q, want passthrough value", hkVL.Params["encryption"])
	}

	// HK region group: HK-01 (ss) + HK-VL (vless).
	var hk *model.ProxyGroup
	for i, g := range p.NodeGroups {
		if g.Name == "HK" {
			hk = &p.NodeGroups[i]
			break
		}
	}
	if hk == nil {
		t.Fatal("HK region group not found")
	}
	wantHK := map[string]bool{"HK-01": true, "HK-VL": true}
	gotHK := map[string]bool{}
	for _, m := range hk.Members {
		gotHK[m] = true
	}
	for m := range wantHK {
		if !gotHK[m] {
			t.Errorf("HK group missing %q (got %v)", m, hk.Members)
		}
	}
}

// T-EXECUTE-VLESS-003: VLESS node is a valid upstream for relay_through
// chaining (sibling of TestExecute_SnellAsRelayThroughUpstream).
func TestExecute_VLessAsRelayThroughUpstream(t *testing.T) {
	vlessURL := "https://vless.example.com/nodes.txt"
	vlessBody := []byte(
		"vless://11111111-2222-3333-4444-555555555555@hk.example.com:443?security=tls&sni=hk.example.com&type=tcp#HK-VL\n")

	f := &fakeFetcher{responses: map[string][]byte{vlessURL: vlessBody}}

	cfg := &config.Config{
		Sources: config.Sources{
			VLess: []config.VLessSource{{URL: vlessURL}},
			CustomProxies: []config.CustomProxy{{
				URL: "socks5://u:p@10.0.0.1:1080", Name: "MY-PROXY",
				Type: "socks5", Server: "10.0.0.1", Port: 1080,
				Params:       map[string]string{"username": "u", "password": "p"},
				RelayThrough: &config.RelayThrough{Type: "all", Strategy: "select"},
			}},
		},
		Groups:   mustGroupsMap(t, `"HK": { match: "(HK)", strategy: select }`),
		Routing:  mustRoutingMap(t, `"proxy": ["HK", "MY-PROXY", "DIRECT"]`),
		Fallback: "proxy",
	}

	p, err := Execute(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var chained *model.Proxy
	for i, px := range p.Proxies {
		if px.Kind == model.KindChained {
			chained = &p.Proxies[i]
			break
		}
	}
	if chained == nil {
		t.Fatal("no chained proxy found; VLESS should have been a valid upstream")
	}
	if chained.Name != "HK-VL→MY-PROXY" {
		t.Errorf("chained.Name = %q, want HK-VL→MY-PROXY", chained.Name)
	}
	if chained.Dialer != "HK-VL" {
		t.Errorf("chained.Dialer = %q, want HK-VL (the vless upstream)", chained.Dialer)
	}

	// @all includes original vless node, not the chained derivative.
	inAll := map[string]bool{}
	for _, n := range p.AllProxies {
		inAll[n] = true
	}
	if !inAll["HK-VL"] {
		t.Error("AllProxies should include HK-VL (original node)")
	}
	if inAll["HK-VL→MY-PROXY"] {
		t.Error("AllProxies must not include chained node")
	}
}

// End-to-end guard: if a user declares a region group and a chain-template
// custom_proxy with the same name, the collision must surface via Execute —
// specifically at the ValidateGraph stage, since Source only checks cp.Name
// against fetched node names (different namespace) and Group generates both
// groups without per-stage collision checks.
//
// This pairs with TestValidateGraph_ChainGroupNameCollidesWithRegionGroup
// (unit test on a hand-crafted GroupResult) by exercising the full YAML →
// Pipeline path.
func TestExecute_ChainGroupNameCollidesWithRegionGroup(t *testing.T) {
	subURL := "https://sub.example.com/api"
	body := makeSubResponse(
		"ss://YWVzLTI1Ni1jZmI6cGFzcw@hk.example.com:8388#HK-01",
	)

	f := &fakeFetcher{responses: map[string][]byte{subURL: body}}

	// Both a region group named "HK-ISP" and a chain-template cp named
	// "HK-ISP" — the chain template will try to create a node group with the
	// same name, triggering a duplicate-declaration error at ValidateGraph.
	cfg := &config.Config{
		Sources: config.Sources{
			Subscriptions: []config.Subscription{{URL: subURL}},
			CustomProxies: []config.CustomProxy{{
				URL: "socks5://1.1.1.1:1080", Name: "HK-ISP",
				Type: "socks5", Server: "1.1.1.1", Port: 1080,
				RelayThrough: &config.RelayThrough{Type: "all", Strategy: "select"},
			}},
		},
		Groups:   mustGroupsMap(t, `"HK-ISP": { match: "(HK)", strategy: select }`),
		Routing:  mustRoutingMap(t, `"proxy": ["HK-ISP", "DIRECT"]`),
		Fallback: "proxy",
	}

	_, err := Execute(context.Background(), cfg, f)
	if err == nil {
		t.Fatal("expected duplicate-group error, got nil")
	}

	var be *errtype.BuildError
	if !errors.As(err, &be) {
		t.Fatalf("err type = %T, want *errtype.BuildError", err)
	}
	// ValidateGraph aggregates all collector messages into BuildError.Message,
	// including the "重复声明" entry emitted by registerName for the duplicate
	// chain + region group name.
	if !strings.Contains(err.Error(), `节点组 "HK-ISP" 重复声明`) {
		t.Errorf("error should mention duplicate node group HK-ISP, got: %v", err)
	}
}

// T-EXEC-SS-01: SS custom proxy end-to-end (no relay_through).
//
// Verifies that an ss-typed custom_proxy traverses the full pipeline and
// surfaces in Proxies + AllProxies as a KindCustom node with cipher/password
// in Params, while staying out of region group regex matching.
func TestExecute_SSCustomProxyEndToEnd(t *testing.T) {
	subURL := "https://sub.example.com/api"
	subBody := makeSubResponse(
		"ss://YWVzLTI1Ni1jZmI6cGFzcw@hk.example.com:8388#HK-01",
	)

	f := &fakeFetcher{responses: map[string][]byte{subURL: subBody}}

	cfg := &config.Config{
		Sources: config.Sources{
			Subscriptions: []config.Subscription{{URL: subURL}},
			CustomProxies: []config.CustomProxy{{
				URL: "ss://YWVzLTI1Ni1nY206bXlwYXNz@1.2.3.4:8388", Name: "MY-SS",
				Type: "ss", Server: "1.2.3.4", Port: 8388,
				Params: map[string]string{"cipher": "aes-256-gcm", "password": "mypass"},
			}},
		},
		Groups:   mustGroupsMap(t, `"HK": { match: "(HK)", strategy: select }`),
		Routing:  mustRoutingMap(t, `"proxy": ["HK", "@all", "DIRECT"]`),
		Fallback: "proxy",
	}

	p, err := Execute(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// MY-SS appears in Proxies as KindCustom with ss type.
	var mySS *model.Proxy
	for i := range p.Proxies {
		if p.Proxies[i].Name == "MY-SS" {
			mySS = &p.Proxies[i]
			break
		}
	}
	if mySS == nil {
		t.Fatal("MY-SS missing from Proxies")
	}
	if mySS.Type != "ss" {
		t.Errorf("MY-SS Type = %q, want ss", mySS.Type)
	}
	if mySS.Kind != model.KindCustom {
		t.Errorf("MY-SS Kind = %q, want custom", mySS.Kind)
	}
	if mySS.Params["cipher"] != "aes-256-gcm" || mySS.Params["password"] != "mypass" {
		t.Errorf("MY-SS Params = %v", mySS.Params)
	}

	// MY-SS appears in @all expansion.
	foundInAll := false
	for _, name := range p.AllProxies {
		if name == "MY-SS" {
			foundInAll = true
			break
		}
	}
	if !foundInAll {
		t.Error("MY-SS missing from AllProxies")
	}

	// MY-SS does NOT appear in HK region group (KindCustom is excluded
	// from region regex matching).
	for _, g := range p.NodeGroups {
		if g.Name != "HK" {
			continue
		}
		for _, m := range g.Members {
			if m == "MY-SS" {
				t.Errorf("MY-SS leaked into HK region group; KindCustom must not match region regex")
			}
		}
	}
}

// T-EXEC-SS-02: SS custom proxy chained end-to-end (with relay_through + plugin).
//
// Verifies that an ss-typed custom_proxy with relay_through generates
// KindChained nodes that inherit Type=ss + cipher/password + plugin from the
// chain template, do NOT appear as standalone proxies, and feed into a chain
// group named after cp.Name.
func TestExecute_SSCustomProxyChainedEndToEnd(t *testing.T) {
	subURL := "https://sub.example.com/api"
	subBody := makeSubResponse(
		"ss://YWVzLTI1Ni1jZmI6cGFzcw@hk.example.com:8388#HK-01",
		"ss://YWVzLTI1Ni1jZmI6cGFzcw@hk2.example.com:8388#HK-02",
	)

	f := &fakeFetcher{responses: map[string][]byte{subURL: subBody}}

	cfg := &config.Config{
		Sources: config.Sources{
			Subscriptions: []config.Subscription{{URL: subURL}},
			CustomProxies: []config.CustomProxy{{
				URL:  "ss://YWVzLTI1Ni1nY206Y2hhaW5wYXNz@1.2.3.4:8388?plugin=obfs-local%3Bobfs%3Dhttp",
				Name: "SS-Chain",
				Type: "ss", Server: "1.2.3.4", Port: 8388,
				Params:       map[string]string{"cipher": "aes-256-gcm", "password": "chainpass"},
				Plugin:       &model.Plugin{Name: "obfs-local", Opts: map[string]string{"obfs": "http"}},
				RelayThrough: &config.RelayThrough{Type: "all", Strategy: "select"},
			}},
		},
		Groups:   mustGroupsMap(t, `"HK": { match: "(HK)", strategy: select }`),
		Routing:  mustRoutingMap(t, `"proxy": ["HK", "SS-Chain", "DIRECT"]`),
		Fallback: "proxy",
	}

	p, err := Execute(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Chain template itself does NOT appear as a standalone proxy.
	for _, px := range p.Proxies {
		if px.Name == "SS-Chain" {
			t.Errorf("SS-Chain should not exist as a standalone proxy; it's a chain template")
		}
	}

	// Chained nodes exist: HK-01→SS-Chain and HK-02→SS-Chain.
	wantChained := map[string]bool{"HK-01→SS-Chain": false, "HK-02→SS-Chain": false}
	for _, px := range p.Proxies {
		if _, want := wantChained[px.Name]; !want {
			continue
		}
		wantChained[px.Name] = true
		if px.Type != "ss" {
			t.Errorf("chained %q Type = %q, want ss", px.Name, px.Type)
		}
		if px.Kind != model.KindChained {
			t.Errorf("chained %q Kind = %q, want chained", px.Name, px.Kind)
		}
		if px.Params["cipher"] != "aes-256-gcm" || px.Params["password"] != "chainpass" {
			t.Errorf("chained %q Params = %v", px.Name, px.Params)
		}
		if px.Plugin == nil {
			t.Errorf("chained %q lost Plugin from chain template", px.Name)
		} else if px.Plugin.Name != "obfs-local" || px.Plugin.Opts["obfs"] != "http" {
			t.Errorf("chained %q Plugin = %+v", px.Name, px.Plugin)
		}
		if px.Dialer == "" {
			t.Errorf("chained %q missing Dialer", px.Name)
		}
	}
	for name, found := range wantChained {
		if !found {
			t.Errorf("chained node %q not found in Proxies", name)
		}
	}

	// Chain group "SS-Chain" exists in NodeGroups containing both chained members.
	var chainGroup *model.ProxyGroup
	for i := range p.NodeGroups {
		if p.NodeGroups[i].Name == "SS-Chain" {
			chainGroup = &p.NodeGroups[i]
			break
		}
	}
	if chainGroup == nil {
		t.Fatal("chain group SS-Chain missing from NodeGroups")
	}
	if len(chainGroup.Members) != 2 {
		t.Errorf("chain group has %d members, want 2", len(chainGroup.Members))
	}

	// SS-Chain does NOT enter @all expansion (chained nodes excluded; chain
	// template doesn't produce a standalone node).
	for _, name := range p.AllProxies {
		if name == "SS-Chain" || strings.Contains(name, "→SS-Chain") {
			t.Errorf("chain-related %q leaked into @all", name)
		}
	}
}
