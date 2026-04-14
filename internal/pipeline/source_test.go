package pipeline

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
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

// T-SRC-SNELL-001: Snell source yields Kind=KindSnell proxies.
func TestSource_SnellSource(t *testing.T) {
	body := []byte("HK-Snell = snell, 1.2.3.4, 57891, psk=xxx, version=4, reuse=true\n" +
		"# comment line\n" +
		"\n" +
		"SG-Snell = snell, 5.6.7.8, 8989, psk=yyy, version=4\n")
	cfg := baseCfg()
	cfg.Sources.Snell = []config.SnellSource{
		{URL: "https://example.com/snell-nodes.txt"},
	}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://example.com/snell-nodes.txt": body,
	}}

	proxies, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(proxies) != 2 {
		t.Fatalf("got %d proxies, want 2", len(proxies))
	}
	if proxies[0].Name != "HK-Snell" || proxies[0].Kind != model.KindSnell {
		t.Errorf("proxies[0] = {Name: %q, Kind: %q}, want {HK-Snell, snell}", proxies[0].Name, proxies[0].Kind)
	}
	if proxies[1].Name != "SG-Snell" || proxies[1].Type != "snell" {
		t.Errorf("proxies[1] = {Name: %q, Type: %q}, want {SG-Snell, snell}", proxies[1].Name, proxies[1].Type)
	}
	if proxies[0].Params["psk"] != "xxx" {
		t.Errorf("proxies[0].Params[psk] = %q, want xxx", proxies[0].Params["psk"])
	}
}

// T-SRC-SNELL-002: Subscriptions and Snell sources share the name dedup pool.
func TestSource_SnellAndSubscriptionDeduped(t *testing.T) {
	// SS subscription with name "HK-01".
	subBody := makeSubResponse("ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:8388#HK-01")
	// Snell source with the same name.
	snellBody := []byte("HK-01 = snell, 1.2.3.4, 57891, psk=xxx, version=4\n")

	cfg := baseCfg()
	cfg.Sources.Subscriptions = []config.Subscription{{URL: "https://sub.example.com/api"}}
	cfg.Sources.Snell = []config.SnellSource{{URL: "https://example.com/snell.txt"}}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://sub.example.com/api":   subBody,
		"https://example.com/snell.txt": snellBody,
	}}

	proxies, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(proxies) != 2 {
		t.Fatalf("got %d proxies, want 2", len(proxies))
	}
	// Subscription is fetched first, so it keeps the original name.
	if proxies[0].Name != "HK-01" {
		t.Errorf("proxies[0].Name = %q, want HK-01", proxies[0].Name)
	}
	// Snell source gets the circled-2 suffix.
	if proxies[1].Name != "HK-01②" {
		t.Errorf("proxies[1].Name = %q, want HK-01②", proxies[1].Name)
	}
}

// T-SRC-SNELL-MULTI: Multiple Snell sources share the name-dedup pool;
// duplicate names across sources get incrementing circled suffixes in
// declaration order.
func TestSource_MultiSnellSourcesDeduped(t *testing.T) {
	url1 := "https://snell.example.com/a.txt"
	url2 := "https://snell.example.com/b.txt"
	body1 := []byte("SG-01 = snell, 1.2.3.4, 57891, psk=xxx, version=4\n")
	body2 := []byte("SG-01 = snell, 5.6.7.8, 8989, psk=yyy, version=4\n")

	cfg := baseCfg()
	cfg.Sources.Snell = []config.SnellSource{{URL: url1}, {URL: url2}}

	f := &fakeFetcher{responses: map[string][]byte{
		url1: body1,
		url2: body2,
	}}

	proxies, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(proxies) != 2 {
		t.Fatalf("got %d proxies, want 2", len(proxies))
	}
	// First Snell source keeps original name.
	if proxies[0].Name != "SG-01" {
		t.Errorf("proxies[0].Name = %q, want SG-01", proxies[0].Name)
	}
	// Second Snell source gets circled-2 suffix.
	if proxies[1].Name != "SG-01②" {
		t.Errorf("proxies[1].Name = %q, want SG-01②", proxies[1].Name)
	}
	// Both are KindSnell.
	for i, px := range proxies {
		if px.Kind != model.KindSnell {
			t.Errorf("proxies[%d].Kind = %q, want snell", i, px.Kind)
		}
	}
}

// T-SRC-SNELL-003: Malformed Snell line fails fast (no silent skip).
func TestSource_SnellSource_MalformedLineFailsFast(t *testing.T) {
	body := []byte("# comment\n" +
		"\n" +
		"HK = snell, 1.2.3.4, 57891, psk=xxx, version=4\n" +
		"INVALID = snell, 1.2.3.4, no-port-here\n") // malformed

	cfg := baseCfg()
	cfg.Sources.Snell = []config.SnellSource{{URL: "https://example.com/snell.txt?token=secret"}}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://example.com/snell.txt?token=secret": body,
	}}

	_, err := Source(context.Background(), cfg, f)
	if err == nil {
		t.Fatal("expected error from malformed Snell line")
	}
	var buildErr *errtype.BuildError
	if !errors.As(err, &buildErr) {
		t.Fatalf("err type = %T, want *errtype.BuildError", err)
	}
	if buildErr.Code != errtype.CodeBuildSnellLineInvalid {
		t.Errorf("Code = %q, want %q", buildErr.Code, errtype.CodeBuildSnellLineInvalid)
	}
	if !strings.Contains(buildErr.Message, `Snell 来源 "https://example.com/snell.txt" 第 4 行解析失败`) {
		t.Errorf("message should include sanitized URL and physical line number, got: %s", buildErr.Message)
	}
	if strings.Contains(buildErr.Message, "secret") {
		t.Errorf("message leaked secret token: %s", buildErr.Message)
	}
	if strings.Contains(buildErr.Message, "build error [source]") {
		t.Errorf("message should embed inner detail without build error prefix: %s", buildErr.Message)
	}

	inner := errors.Unwrap(buildErr)
	var innerBuildErr *errtype.BuildError
	if !errors.As(inner, &innerBuildErr) {
		t.Fatalf("inner err type = %T, want *errtype.BuildError", inner)
	}
	if innerBuildErr == buildErr {
		t.Fatal("outer BuildError should wrap the inner parse error, not itself")
	}
	if innerBuildErr.Code != errtype.CodeBuildSnellLineInvalid {
		t.Errorf("inner Code = %q, want %q", innerBuildErr.Code, errtype.CodeBuildSnellLineInvalid)
	}
	if !strings.Contains(innerBuildErr.Message, `port "no-port-here" 不是整数`) {
		t.Errorf("inner message should preserve parser detail, got: %s", innerBuildErr.Message)
	}
}

// T-SRC-SNELL-004: Empty Snell source (all lines commented out) returns FetchError.
func TestSource_SnellSource_EmptyReported(t *testing.T) {
	body := []byte("# only comments here\n\n// nothing else\n")

	cfg := baseCfg()
	cfg.Sources.Snell = []config.SnellSource{{URL: "https://example.com/snell.txt?token=secret"}}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://example.com/snell.txt?token=secret": body,
	}}

	_, err := Source(context.Background(), cfg, f)
	if err == nil {
		t.Fatal("expected error for empty Snell source")
	}
	var fe *errtype.FetchError
	if !errors.As(err, &fe) {
		t.Fatalf("err type = %T, want *errtype.FetchError", err)
	}
	if fe.Code != errtype.CodeFetchSubscriptionEmpty {
		t.Errorf("Code = %q, want %q", fe.Code, errtype.CodeFetchSubscriptionEmpty)
	}
	// URL in error message must be sanitized (no raw token).
	if strings.Contains(fe.URL, "secret") {
		t.Errorf("FetchError.URL leaked secret: %q", fe.URL)
	}
}

// --- VLESS source tests ---

// vlessURIFor builds a minimal valid vless:// URI for test fixtures.
func vlessURIFor(name, server string, port int) string {
	return fmt.Sprintf("vless://11111111-2222-3333-4444-555555555555@%s:%d?security=tls&sni=%s&type=tcp#%s",
		server, port, server, name)
}

// T-SRC-VLESS-SRC-001: Basic VLESS source yields correct proxies, skipping
// comments and blank lines along the way.
func TestSource_VLessSourceBasic(t *testing.T) {
	body := []byte("# vless nodes\n" +
		"\n" +
		vlessURIFor("HK-VL", "hk.example.com", 443) + "\n" +
		"// second block\n" +
		vlessURIFor("SG-VL", "sg.example.com", 443) + "\n")

	cfg := baseCfg()
	cfg.Sources.VLess = []config.VLessSource{{URL: "https://example.com/vless.txt"}}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://example.com/vless.txt": body,
	}}

	proxies, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(proxies) != 2 {
		t.Fatalf("got %d proxies, want 2", len(proxies))
	}
	if proxies[0].Name != "HK-VL" || proxies[0].Kind != model.KindVLess {
		t.Errorf("proxies[0] = {%q, %q}, want {HK-VL, vless}", proxies[0].Name, proxies[0].Kind)
	}
	if proxies[1].Name != "SG-VL" || proxies[1].Type != "vless" {
		t.Errorf("proxies[1] = {%q, %q}, want {SG-VL, vless}", proxies[1].Name, proxies[1].Type)
	}
}

// T-SRC-VLESS-SRC-002: Unknown transport falls back to tcp and non-none
// encryption survives source parsing.
func TestSource_VLessUnknownTypeFallsBackAndEncryptionPassesThrough(t *testing.T) {
	body := []byte(
		"vless://11111111-2222-3333-4444-555555555555@hk.example.com:443?security=tls&sni=hk.example.com&type=quic&encryption=mlkem768x25519plus.native#HK-VL\n",
	)

	cfg := baseCfg()
	cfg.Sources.VLess = []config.VLessSource{{URL: "https://example.com/vless.txt"}}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://example.com/vless.txt": body,
	}}

	proxies, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(proxies) != 1 {
		t.Fatalf("got %d proxies, want 1", len(proxies))
	}
	if proxies[0].Params["network"] != "tcp" {
		t.Errorf("Params[network] = %q, want tcp fallback", proxies[0].Params["network"])
	}
	if proxies[0].Params["encryption"] != "mlkem768x25519plus.native" {
		t.Errorf("Params[encryption] = %q, want passthrough", proxies[0].Params["encryption"])
	}
}

// T-SRC-VLESS-SRC-003: Malformed VLESS line fails fast (strict mode like Snell).
func TestSource_VLessBadLineAbortsSource(t *testing.T) {
	body := []byte("# header\n" +
		vlessURIFor("HK-VL", "hk.example.com", 443) + "\n" +
		"vless://not-a-uuid@example.com:443?type=tcp#bad\n") // bad uuid

	cfg := baseCfg()
	cfg.Sources.VLess = []config.VLessSource{{URL: "https://example.com/vless.txt?token=secret"}}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://example.com/vless.txt?token=secret": body,
	}}

	_, err := Source(context.Background(), cfg, f)
	if err == nil {
		t.Fatal("expected error from malformed VLESS line")
	}
	var be *errtype.BuildError
	if !errors.As(err, &be) {
		t.Fatalf("err type = %T, want *errtype.BuildError", err)
	}
	if be.Code != errtype.CodeBuildVLessSourceLineInvalid {
		t.Errorf("Code = %q, want %q", be.Code, errtype.CodeBuildVLessSourceLineInvalid)
	}
	// Unwrap: inner err is the per-URI parse error with CodeBuildVLessURIInvalid.
	var inner *errtype.BuildError
	if !errors.As(errors.Unwrap(be), &inner) {
		t.Fatalf("inner err not a BuildError")
	}
	if inner.Code != errtype.CodeBuildVLessURIInvalid {
		t.Errorf("inner Code = %q, want %q", inner.Code, errtype.CodeBuildVLessURIInvalid)
	}
	if !strings.Contains(be.Message, `VLESS 来源 "https://example.com/vless.txt" 第 3 行解析失败`) {
		t.Errorf("message should include sanitized URL and line number, got: %s", be.Message)
	}
	if strings.Contains(be.Message, "secret") {
		t.Errorf("message leaked secret token: %s", be.Message)
	}
}

// T-SRC-VLESS-SRC-004: All-comments VLESS source reports empty.
func TestSource_VLessSource_EmptyReported(t *testing.T) {
	body := []byte("# only comments\n\n// nothing else\n")

	cfg := baseCfg()
	cfg.Sources.VLess = []config.VLessSource{{URL: "https://example.com/vless.txt?token=secret"}}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://example.com/vless.txt?token=secret": body,
	}}

	_, err := Source(context.Background(), cfg, f)
	if err == nil {
		t.Fatal("expected error for empty VLESS source")
	}
	var fe *errtype.FetchError
	if !errors.As(err, &fe) {
		t.Fatalf("err type = %T, want *errtype.FetchError", err)
	}
	if fe.Code != errtype.CodeFetchSubscriptionEmpty {
		t.Errorf("Code = %q, want %q", fe.Code, errtype.CodeFetchSubscriptionEmpty)
	}
	if !strings.Contains(fe.Message, "VLESS") {
		t.Errorf("message should mention VLESS, got: %s", fe.Message)
	}
	if strings.Contains(fe.URL, "secret") {
		t.Errorf("FetchError.URL leaked secret: %q", fe.URL)
	}
}

// T-SRC-VLESS-SRC-005: Subscription, Snell and VLESS sources share a single
// dedup pool.
func TestSource_VLessSharesDedupPoolWithSSAndSnell(t *testing.T) {
	subBody := makeSubResponse("ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:8388#HK-01")
	snellBody := []byte("HK-01 = snell, 1.2.3.4, 57891, psk=xxx, version=4\n")
	vlessBody := []byte(vlessURIFor("HK-01", "hk.example.com", 443) + "\n")

	cfg := baseCfg()
	cfg.Sources.Subscriptions = []config.Subscription{{URL: "https://sub.example.com/api"}}
	cfg.Sources.Snell = []config.SnellSource{{URL: "https://snell.example.com/s.txt"}}
	cfg.Sources.VLess = []config.VLessSource{{URL: "https://vless.example.com/v.txt"}}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://sub.example.com/api":     subBody,
		"https://snell.example.com/s.txt": snellBody,
		"https://vless.example.com/v.txt": vlessBody,
	}}

	proxies, err := Source(context.Background(), cfg, f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(proxies) != 3 {
		t.Fatalf("got %d proxies, want 3", len(proxies))
	}
	// FetchOrder fallback: subscriptions, snell, vless (alphabetical-ish default).
	// So SS "HK-01" first, Snell "HK-01②" second, VLESS "HK-01③" third.
	wantNames := []string{"HK-01", "HK-01②", "HK-01③"}
	for i, want := range wantNames {
		if proxies[i].Name != want {
			t.Errorf("proxies[%d].Name = %q, want %q", i, proxies[i].Name, want)
		}
	}
}

// T-SRC-VLESS-SRC-006: A custom proxy with the same name as a VLESS source
// node is rejected via checkNameConflicts; the error message identifies
// the conflict source as "VLESS 来源" (describeFetchedKind).
func TestSource_VLessCustomNameConflictError(t *testing.T) {
	vlessBody := []byte(vlessURIFor("HK-01", "hk.example.com", 443) + "\n")

	cfg := baseCfg()
	cfg.Sources.VLess = []config.VLessSource{{URL: "https://vless.example.com/v.txt"}}
	cfg.Sources.CustomProxies = []config.CustomProxy{{
		Name:   "HK-01",
		Type:   "socks5",
		Server: "1.2.3.4",
		Port:   1080,
	}}

	f := &fakeFetcher{responses: map[string][]byte{
		"https://vless.example.com/v.txt": vlessBody,
	}}

	_, err := Source(context.Background(), cfg, f)
	if err == nil {
		t.Fatal("expected conflict error")
	}
	if !strings.Contains(err.Error(), "VLESS 来源") {
		t.Errorf("error message should identify VLESS source, got: %s", err.Error())
	}
}

// TestSource_UnknownFetchKindErrors guards the Source() switch default
// branch: if Sources.FetchOrder contains a typo/unsupported kind, Source()
// surfaces a BuildError instead of silently skipping. Production YAML paths
// cannot hit this (UnmarshalYAML rejects unknown keys upfront), but in-
// memory Config construction in tests or future extensions can.
func TestSource_UnknownFetchKindErrors(t *testing.T) {
	cfg := baseCfg()
	cfg.Sources.FetchOrder = []string{"hysteria"} // typo / future kind

	f := &fakeFetcher{}

	_, err := Source(context.Background(), cfg, f)
	if err == nil {
		t.Fatal("expected error for unknown fetch-kind, got nil")
	}
	var be *errtype.BuildError
	if !errors.As(err, &be) {
		t.Fatalf("err type = %T, want *errtype.BuildError", err)
	}
	if be.Code != errtype.CodeBuildValidationFailed {
		t.Errorf("Code = %q, want %q", be.Code, errtype.CodeBuildValidationFailed)
	}
	if !strings.Contains(be.Message, `"hysteria"`) {
		t.Errorf("message should quote the bad kind, got: %s", be.Message)
	}
}

// T-SRC-VLESS-SRC-007: Source traversal follows Sources.FetchOrder.
//
// Declaring order snell→vless→subscriptions should yield the node slice in
// the same category order. The test inspects the observed order against
// different FetchOrder permutations, proving the switch-dispatch honors
// the user's YAML layout.
func TestSource_RespectsFetchOrder(t *testing.T) {
	subBody := makeSubResponse("ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@sub.example.com:8388#SS-01")
	snellBody := []byte("SNELL-01 = snell, 1.2.3.4, 57891, psk=xxx, version=4\n")
	vlessBody := []byte(vlessURIFor("VL-01", "v.example.com", 443) + "\n")

	cases := []struct {
		name  string
		order []string
		want  []string
	}{
		{"snell first", []string{"snell", "vless", "subscriptions"}, []string{"SNELL-01", "VL-01", "SS-01"}},
		{"vless first", []string{"vless", "subscriptions", "snell"}, []string{"VL-01", "SS-01", "SNELL-01"}},
		{"subs first", []string{"subscriptions", "snell", "vless"}, []string{"SS-01", "SNELL-01", "VL-01"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := baseCfg()
			cfg.Sources.Subscriptions = []config.Subscription{{URL: "https://sub.example.com/api"}}
			cfg.Sources.Snell = []config.SnellSource{{URL: "https://snell.example.com/s.txt"}}
			cfg.Sources.VLess = []config.VLessSource{{URL: "https://vless.example.com/v.txt"}}
			cfg.Sources.FetchOrder = tc.order

			f := &fakeFetcher{responses: map[string][]byte{
				"https://sub.example.com/api":     subBody,
				"https://snell.example.com/s.txt": snellBody,
				"https://vless.example.com/v.txt": vlessBody,
			}}

			proxies, err := Source(context.Background(), cfg, f)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(proxies) != 3 {
				t.Fatalf("got %d proxies, want 3", len(proxies))
			}
			for i, want := range tc.want {
				if proxies[i].Name != want {
					t.Errorf("proxies[%d].Name = %q, want %q", i, proxies[i].Name, want)
				}
			}
		})
	}
}
