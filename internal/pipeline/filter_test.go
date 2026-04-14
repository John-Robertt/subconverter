package pipeline

import (
	"errors"
	"testing"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

func makeProxy(name string, kind model.ProxyKind) model.Proxy {
	return model.Proxy{
		Name: name,
		Type: "ss",
		Kind: kind,
	}
}

// T-FLT-001: exclude filters subscription nodes
func TestFilter_ExcludeSubscriptionNodes(t *testing.T) {
	proxies := []model.Proxy{
		makeProxy("HK-01", model.KindSubscription),
		makeProxy("过期-节点", model.KindSubscription),
		makeProxy("SG-01", model.KindSubscription),
		makeProxy("剩余流量-info", model.KindSubscription),
		makeProxy("US-01", model.KindSubscription),
	}

	result, err := Filter(proxies, "(过期|剩余流量)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("got %d proxies, want 3", len(result))
	}

	wantNames := []string{"HK-01", "SG-01", "US-01"}
	for i, want := range wantNames {
		if result[i].Name != want {
			t.Errorf("result[%d].Name = %q, want %q", i, result[i].Name, want)
		}
	}
}

// T-FLT-002: custom proxies not filtered
func TestFilter_CustomProxiesNotFiltered(t *testing.T) {
	proxies := []model.Proxy{
		makeProxy("过期-sub-node", model.KindSubscription),
		makeProxy("过期-custom-proxy", model.KindCustom),
		makeProxy("HK-01", model.KindSubscription),
	}

	result, err := Filter(proxies, "过期")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("got %d proxies, want 2", len(result))
	}

	// Custom proxy survives even though name matches.
	if result[0].Name != "过期-custom-proxy" {
		t.Errorf("result[0].Name = %q, want custom proxy to survive", result[0].Name)
	}
	if result[0].Kind != model.KindCustom {
		t.Errorf("result[0].Kind = %q, want custom", result[0].Kind)
	}
	if result[1].Name != "HK-01" {
		t.Errorf("result[1].Name = %q, want HK-01", result[1].Name)
	}
}

func TestFilter_EmptyPattern(t *testing.T) {
	proxies := []model.Proxy{
		makeProxy("HK-01", model.KindSubscription),
		makeProxy("SG-01", model.KindSubscription),
	}

	result, err := Filter(proxies, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("got %d proxies, want 2 (no filtering)", len(result))
	}
}

func TestFilter_AllFiltered(t *testing.T) {
	proxies := []model.Proxy{
		makeProxy("test-1", model.KindSubscription),
		makeProxy("test-2", model.KindSubscription),
	}

	result, err := Filter(proxies, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 0 {
		t.Fatalf("got %d proxies, want 0", len(result))
	}
}

func TestFilter_NoMatch(t *testing.T) {
	proxies := []model.Proxy{
		makeProxy("HK-01", model.KindSubscription),
		makeProxy("SG-01", model.KindSubscription),
	}

	result, err := Filter(proxies, "NOMATCH")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("got %d proxies, want 2", len(result))
	}
}

// T-FLT-SNELL: Snell nodes share subscription-like filtering semantics.
func TestFilter_SnellNodesFiltered(t *testing.T) {
	proxies := []model.Proxy{
		makeProxy("HK-Snell", model.KindSnell),
		makeProxy("过期-Snell", model.KindSnell),
		makeProxy("HK-01", model.KindSubscription),
		makeProxy("过期-sub", model.KindSubscription),
	}

	result, err := Filter(proxies, "过期")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("got %d proxies, want 2 (过期-Snell and 过期-sub both filtered)", len(result))
	}
	wantNames := []string{"HK-Snell", "HK-01"}
	for i, want := range wantNames {
		if result[i].Name != want {
			t.Errorf("result[%d].Name = %q, want %q", i, result[i].Name, want)
		}
	}
}

// T-FLT-VLESS: VLESS nodes share subscription-like filtering semantics.
// Guards the isFetchedKind extension — if someone narrows it back to only
// Subscription/Snell, this test trips.
func TestFilter_ExcludesVLessNodesByName(t *testing.T) {
	proxies := []model.Proxy{
		makeProxy("HK-VL", model.KindVLess),
		makeProxy("过期-VL", model.KindVLess),
		makeProxy("HK-01", model.KindSubscription),
	}

	result, err := Filter(proxies, "过期")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("got %d proxies, want 2 (过期-VL filtered)", len(result))
	}
	wantNames := []string{"HK-VL", "HK-01"}
	for i, want := range wantNames {
		if result[i].Name != want {
			t.Errorf("result[%d].Name = %q, want %q", i, result[i].Name, want)
		}
	}
}

func TestFilter_ChainedProxiesNotFiltered(t *testing.T) {
	proxies := []model.Proxy{
		makeProxy("过期-chain", model.KindChained),
		makeProxy("HK-01", model.KindSubscription),
	}

	result, err := Filter(proxies, "过期")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("got %d proxies, want 2 (chained not filtered)", len(result))
	}
	if result[0].Name != "过期-chain" {
		t.Errorf("chained proxy should survive filter, got %q", result[0].Name)
	}
}

func TestFilter_InvalidRegex(t *testing.T) {
	_, err := Filter(nil, "[invalid")
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}

	var buildErr *errtype.BuildError
	if !errors.As(err, &buildErr) {
		t.Fatalf("error type = %T, want *errtype.BuildError", err)
	}
	if buildErr.Phase != "filter" {
		t.Errorf("Phase = %q, want %q", buildErr.Phase, "filter")
	}
}
