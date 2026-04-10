package pipeline

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// --- test helpers ---

type fakeFetcher struct {
	responses map[string][]byte
	err       error
}

func (f *fakeFetcher) Fetch(_ context.Context, url string) ([]byte, error) {
	if f.err != nil {
		return nil, &errtype.FetchError{Code: errtype.CodeFetchRequestFailed, URL: url, Message: "请求上游失败：" + f.err.Error(), Cause: f.err}
	}
	body, ok := f.responses[url]
	if !ok {
		return nil, &errtype.FetchError{Code: errtype.CodeFetchStatusInvalid, URL: url, Message: "上游返回 HTTP 404"}
	}
	return body, nil
}

func makeSubResponse(uris ...string) []byte {
	joined := strings.Join(uris, "\n")
	return []byte(base64.StdEncoding.EncodeToString([]byte(joined)))
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return data
}

func baseCfg() *config.Config {
	return &config.Config{
		Sources: config.Sources{},
	}
}

// --- Source tests ---

func TestSource_SingleSubscription(t *testing.T) {
	body := mustReadFile(t, "../../testdata/subscriptions/sample.txt")
	cfg := baseCfg()
	cfg.Sources.Subscriptions = []config.Subscription{
		{URL: "https://sub1.example.com/api?token=secret"},
	}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://sub1.example.com/api?token=secret": body,
	}}

	proxies, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(proxies) != 5 {
		t.Fatalf("got %d proxies, want 5", len(proxies))
	}

	// Verify order matches subscription order.
	wantNames := []string{"HK-01", "HK-02", "SG-01", "JP-东京-01", "US-洛杉矶-01"}
	for i, want := range wantNames {
		if proxies[i].Name != want {
			t.Errorf("proxy[%d].Name = %q, want %q", i, proxies[i].Name, want)
		}
		if proxies[i].Kind != model.KindSubscription {
			t.Errorf("proxy[%d].Kind = %q, want %q", i, proxies[i].Kind, model.KindSubscription)
		}
	}
}

// T-SRC-003: Multi-subscription merge
func TestSource_MultiSubscriptionMerge(t *testing.T) {
	body1 := mustReadFile(t, "../../testdata/subscriptions/sample.txt")
	body2 := mustReadFile(t, "../../testdata/subscriptions/sample_sub2.txt")

	cfg := baseCfg()
	cfg.Sources.Subscriptions = []config.Subscription{
		{URL: "https://sub1.example.com"},
		{URL: "https://sub2.example.com"},
	}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://sub1.example.com": body1,
		"https://sub2.example.com": body2,
	}}

	proxies, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// sample.txt has 5 nodes, sample_sub2.txt has 2 nodes = 7 total.
	if len(proxies) != 7 {
		t.Fatalf("got %d proxies, want 7", len(proxies))
	}

	// sub1's HK-01 keeps original name, sub2's HK-01 becomes HK-01②.
	if proxies[0].Name != "HK-01" {
		t.Errorf("first HK-01 should keep name, got %q", proxies[0].Name)
	}
	if proxies[5].Name != "HK-01②" {
		t.Errorf("duplicate HK-01 should become HK-01②, got %q", proxies[5].Name)
	}
	if proxies[6].Name != "KR-01" {
		t.Errorf("KR-01 should be unchanged, got %q", proxies[6].Name)
	}
}

func TestSource_CrossSubscriptionDedup(t *testing.T) {
	// Three subscriptions, all containing "NODE-A".
	sub := makeSubResponse("ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@a.example.com:8388#NODE-A")

	cfg := baseCfg()
	cfg.Sources.Subscriptions = []config.Subscription{
		{URL: "https://sub1.example.com"},
		{URL: "https://sub2.example.com"},
		{URL: "https://sub3.example.com"},
	}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://sub1.example.com": sub,
		"https://sub2.example.com": sub,
		"https://sub3.example.com": sub,
	}}

	proxies, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantNames := []string{"NODE-A", "NODE-A②", "NODE-A③"}
	if len(proxies) != len(wantNames) {
		t.Fatalf("got %d proxies, want %d", len(proxies), len(wantNames))
	}
	for i, want := range wantNames {
		if proxies[i].Name != want {
			t.Errorf("proxy[%d].Name = %q, want %q", i, proxies[i].Name, want)
		}
	}
}

func TestSource_CustomProxyConversion(t *testing.T) {
	cfg := baseCfg()
	cfg.Sources.CustomProxies = []config.CustomProxy{
		{
			Name:     "HK-ISP",
			Type:     "socks5",
			Server:   "154.197.1.1",
			Port:     45002,
			Username: "user1",
			Password: "pass1",
		},
	}

	f := &fakeFetcher{responses: map[string][]byte{}}

	proxies, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(proxies) != 1 {
		t.Fatalf("got %d proxies, want 1", len(proxies))
	}

	p := proxies[0]
	if p.Name != "HK-ISP" {
		t.Errorf("Name = %q, want %q", p.Name, "HK-ISP")
	}
	if p.Type != "socks5" {
		t.Errorf("Type = %q, want %q", p.Type, "socks5")
	}
	if p.Kind != model.KindCustom {
		t.Errorf("Kind = %q, want %q", p.Kind, model.KindCustom)
	}
	if p.Params["username"] != "user1" {
		t.Errorf("username = %q, want %q", p.Params["username"], "user1")
	}
	if p.Params["password"] != "pass1" {
		t.Errorf("password = %q, want %q", p.Params["password"], "pass1")
	}
}

func TestSource_CustomVsSubscriptionConflict(t *testing.T) {
	sub := makeSubResponse("ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:8388#HK-ISP")

	cfg := baseCfg()
	cfg.Sources.Subscriptions = []config.Subscription{
		{URL: "https://sub.example.com"},
	}
	cfg.Sources.CustomProxies = []config.CustomProxy{
		{Name: "HK-ISP", Type: "socks5", Server: "1.2.3.4", Port: 1080},
	}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://sub.example.com": sub,
	}}

	_, err := Source(context.Background(), cfg, f)
	if err == nil {
		t.Fatal("expected error for name conflict")
	}

	var buildErr *errtype.BuildError
	if !errors.As(err, &buildErr) {
		t.Fatalf("error type = %T, want *errtype.BuildError", err)
	}
	if buildErr.Phase != "source" {
		t.Errorf("Phase = %q, want %q", buildErr.Phase, "source")
	}
	if !strings.Contains(buildErr.Message, "HK-ISP") {
		t.Errorf("error should mention conflicting name, got: %s", buildErr.Message)
	}
}

func TestSource_FetchError(t *testing.T) {
	cfg := baseCfg()
	cfg.Sources.Subscriptions = []config.Subscription{
		{URL: "https://sub.example.com/api?token=secret"},
	}

	f := &fakeFetcher{err: errors.New("connection refused")}

	_, err := Source(context.Background(), cfg, f)
	if err == nil {
		t.Fatal("expected error")
	}

	var fetchErr *errtype.FetchError
	if !errors.As(err, &fetchErr) {
		t.Fatalf("error type = %T, want *errtype.FetchError", err)
	}
}

func TestSource_InvalidBase64Response(t *testing.T) {
	cfg := baseCfg()
	cfg.Sources.Subscriptions = []config.Subscription{
		{URL: "https://sub.example.com"},
	}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://sub.example.com": []byte("this is not base64!!!@@@"),
	}}

	_, err := Source(context.Background(), cfg, f)
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}

	var fetchErr *errtype.FetchError
	if !errors.As(err, &fetchErr) {
		t.Fatalf("error type = %T, want *errtype.FetchError", err)
	}
	if !strings.Contains(fetchErr.Message, "Base64") {
		t.Errorf("error should mention Base64, got: %s", fetchErr.Message)
	}
}

func TestSource_EmptySubscription(t *testing.T) {
	cfg := baseCfg()
	cfg.Sources.Subscriptions = []config.Subscription{
		{URL: "https://sub.example.com"},
	}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://sub.example.com": []byte(""),
	}}

	_, err := Source(context.Background(), cfg, f)
	if err == nil {
		t.Fatal("expected error for empty subscription")
	}

	var fetchErr *errtype.FetchError
	if !errors.As(err, &fetchErr) {
		t.Fatalf("error type = %T, want *errtype.FetchError", err)
	}
	if fetchErr.URL != "https://sub.example.com" {
		t.Errorf("URL = %q, want %q", fetchErr.URL, "https://sub.example.com")
	}
	if !strings.Contains(fetchErr.Message, "任何有效节点") {
		t.Errorf("error should mention empty subscription, got: %s", fetchErr.Message)
	}
}

func TestSource_MixedValidInvalidURIs(t *testing.T) {
	sub := makeSubResponse(
		"ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:8388#HK-01",
		"not-a-valid-ss-uri",
		"ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@sg.example.com:8388#SG-01",
	)

	cfg := baseCfg()
	cfg.Sources.Subscriptions = []config.Subscription{
		{URL: "https://sub.example.com"},
	}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://sub.example.com": sub,
	}}

	proxies, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Invalid line should be skipped, 2 valid nodes remain.
	if len(proxies) != 2 {
		t.Fatalf("got %d proxies, want 2", len(proxies))
	}
	if proxies[0].Name != "HK-01" {
		t.Errorf("proxy[0].Name = %q, want %q", proxies[0].Name, "HK-01")
	}
	if proxies[1].Name != "SG-01" {
		t.Errorf("proxy[1].Name = %q, want %q", proxies[1].Name, "SG-01")
	}
}

func TestSource_SubscriptionAndCustomCombined(t *testing.T) {
	sub := makeSubResponse("ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:8388#HK-01")

	cfg := baseCfg()
	cfg.Sources.Subscriptions = []config.Subscription{
		{URL: "https://sub.example.com"},
	}
	cfg.Sources.CustomProxies = []config.CustomProxy{
		{Name: "MY-PROXY", Type: "http", Server: "10.0.0.1", Port: 8080},
	}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://sub.example.com": sub,
	}}

	proxies, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1 subscription + 1 custom = 2 total.
	if len(proxies) != 2 {
		t.Fatalf("got %d proxies, want 2", len(proxies))
	}

	// Subscription nodes come first, then custom.
	if proxies[0].Kind != model.KindSubscription {
		t.Errorf("proxy[0].Kind = %q, want subscription", proxies[0].Kind)
	}
	if proxies[1].Kind != model.KindCustom {
		t.Errorf("proxy[1].Kind = %q, want custom", proxies[1].Kind)
	}
}

func TestSource_DedupSuffixCollision(t *testing.T) {
	// Subscription already contains "NODE-A②", and "NODE-A" appears twice.
	// The dedup suffix "NODE-A②" should not collide with the original "NODE-A②".
	sub := makeSubResponse(
		"ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@a.example.com:8388#NODE-A",
		"ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@b.example.com:8388#NODE-A%E2%91%A1", // NODE-A②
		"ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@c.example.com:8388#NODE-A",
	)

	cfg := baseCfg()
	cfg.Sources.Subscriptions = []config.Subscription{
		{URL: "https://sub.example.com"},
	}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://sub.example.com": sub,
	}}

	proxies, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(proxies) != 3 {
		t.Fatalf("got %d proxies, want 3", len(proxies))
	}

	// All names must be unique.
	names := make(map[string]struct{})
	for _, p := range proxies {
		if _, exists := names[p.Name]; exists {
			t.Errorf("duplicate name %q after dedup", p.Name)
		}
		names[p.Name] = struct{}{}
	}

	// First NODE-A keeps original name.
	if proxies[0].Name != "NODE-A" {
		t.Errorf("proxy[0].Name = %q, want %q", proxies[0].Name, "NODE-A")
	}
	// Original NODE-A② keeps its name.
	if proxies[1].Name != "NODE-A②" {
		t.Errorf("proxy[1].Name = %q, want %q", proxies[1].Name, "NODE-A②")
	}
	// Second NODE-A would normally become NODE-A②, but that collides with the
	// natural NODE-A②, so it should advance to NODE-A③.
	if proxies[2].Name != "NODE-A③" {
		t.Errorf("proxy[2].Name = %q, want %q", proxies[2].Name, "NODE-A③")
	}
}

func TestSource_NoSubscriptionsOnlyCustom(t *testing.T) {
	cfg := baseCfg()
	cfg.Sources.CustomProxies = []config.CustomProxy{
		{Name: "PROXY-1", Type: "socks5", Server: "1.1.1.1", Port: 1080},
	}

	f := &fakeFetcher{responses: map[string][]byte{}}

	proxies, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(proxies) != 1 {
		t.Fatalf("got %d proxies, want 1", len(proxies))
	}
	if proxies[0].Name != "PROXY-1" {
		t.Errorf("Name = %q, want PROXY-1", proxies[0].Name)
	}
}
