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

// T-FLT-001：exclude 过滤订阅节点
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
		t.Fatalf("意外错误：%v", err)
	}

	if len(result) != 3 {
		t.Fatalf("得到 %d 个节点，期望 3 个", len(result))
	}

	wantNames := []string{"HK-01", "SG-01", "US-01"}
	for i, want := range wantNames {
		if result[i].Name != want {
			t.Errorf("result[%d].Name = %q，期望 %q", i, result[i].Name, want)
		}
	}
}

// T-FLT-002：自定义节点不参与过滤
func TestFilter_CustomProxiesNotFiltered(t *testing.T) {
	proxies := []model.Proxy{
		makeProxy("过期-sub-node", model.KindSubscription),
		makeProxy("过期-custom-proxy", model.KindCustom),
		makeProxy("HK-01", model.KindSubscription),
	}

	result, err := Filter(proxies, "过期")
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	if len(result) != 2 {
		t.Fatalf("得到 %d 个节点，期望 2 个", len(result))
	}

	// 自定义节点即使名称匹配也会保留。
	if result[0].Name != "过期-custom-proxy" {
		t.Errorf("result[0].Name = %q，期望自定义节点被保留", result[0].Name)
	}
	if result[0].Kind != model.KindCustom {
		t.Errorf("result[0].Kind = %q，期望 custom", result[0].Kind)
	}
	if result[1].Name != "HK-01" {
		t.Errorf("result[1].Name = %q，期望 HK-01", result[1].Name)
	}
}

func TestFilter_EmptyPattern(t *testing.T) {
	proxies := []model.Proxy{
		makeProxy("HK-01", model.KindSubscription),
		makeProxy("SG-01", model.KindSubscription),
	}

	result, err := Filter(proxies, "")
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	if len(result) != 2 {
		t.Fatalf("得到 %d 个节点，期望 2 个（不做过滤）", len(result))
	}
}

func TestFilter_AllFiltered(t *testing.T) {
	proxies := []model.Proxy{
		makeProxy("test-1", model.KindSubscription),
		makeProxy("test-2", model.KindSubscription),
	}

	result, err := Filter(proxies, "test")
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	if len(result) != 0 {
		t.Fatalf("得到 %d 个节点，期望 0 个", len(result))
	}
}

func TestFilter_NoMatch(t *testing.T) {
	proxies := []model.Proxy{
		makeProxy("HK-01", model.KindSubscription),
		makeProxy("SG-01", model.KindSubscription),
	}

	result, err := Filter(proxies, "NOMATCH")
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	if len(result) != 2 {
		t.Fatalf("得到 %d 个节点，期望 2 个", len(result))
	}
}

// T-FLT-SNELL：Snell 节点与订阅节点使用相同的过滤语义。
func TestFilter_SnellNodesFiltered(t *testing.T) {
	proxies := []model.Proxy{
		makeProxy("HK-Snell", model.KindSnell),
		makeProxy("过期-Snell", model.KindSnell),
		makeProxy("HK-01", model.KindSubscription),
		makeProxy("过期-sub", model.KindSubscription),
	}

	result, err := Filter(proxies, "过期")
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	if len(result) != 2 {
		t.Fatalf("得到 %d 个节点，期望 2 个（过期-Snell 和 过期-sub 都被过滤）", len(result))
	}
	wantNames := []string{"HK-Snell", "HK-01"}
	for i, want := range wantNames {
		if result[i].Name != want {
			t.Errorf("result[%d].Name = %q，期望 %q", i, result[i].Name, want)
		}
	}
}

// T-FLT-VLESS：VLESS 节点与订阅节点使用相同的过滤语义。
// 守护 isFetchedKind 的扩展：如果有人把它改回只包含 Source/Snell，
// 这个测试会失败。
func TestFilter_ExcludesVLessNodesByName(t *testing.T) {
	proxies := []model.Proxy{
		makeProxy("HK-VL", model.KindVLess),
		makeProxy("过期-VL", model.KindVLess),
		makeProxy("HK-01", model.KindSubscription),
	}

	result, err := Filter(proxies, "过期")
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	if len(result) != 2 {
		t.Fatalf("得到 %d 个节点，期望 2 个（过期-VL 被过滤）", len(result))
	}
	wantNames := []string{"HK-VL", "HK-01"}
	for i, want := range wantNames {
		if result[i].Name != want {
			t.Errorf("result[%d].Name = %q，期望 %q", i, result[i].Name, want)
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
		t.Fatalf("意外错误：%v", err)
	}

	if len(result) != 2 {
		t.Fatalf("得到 %d 个节点，期望 2 个（链式节点不参与过滤）", len(result))
	}
	if result[0].Name != "过期-chain" {
		t.Errorf("链式节点应在过滤后保留，实际 %q", result[0].Name)
	}
}

func TestFilter_InvalidRegex(t *testing.T) {
	_, err := Filter(nil, "[invalid")
	if err == nil {
		t.Fatal("无效正则应返回错误")
	}

	var buildErr *errtype.BuildError
	if !errors.As(err, &buildErr) {
		t.Fatalf("错误类型 = %T，期望 *errtype.BuildError", err)
	}
	if buildErr.Phase != "filter" {
		t.Errorf("Phase = %q，期望 %q", buildErr.Phase, "filter")
	}
}
