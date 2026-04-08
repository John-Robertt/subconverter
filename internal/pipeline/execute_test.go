package pipeline

import (
	"context"
	"errors"
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
