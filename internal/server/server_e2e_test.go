package server_test

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/server"
	"gopkg.in/yaml.v3"
)

// --- test helpers ---

// fakeFetcher maps URLs to canned responses.
type fakeFetcher struct {
	responses map[string][]byte
	err       error // if set, all fetches fail
}

func (f *fakeFetcher) Fetch(_ context.Context, rawURL string) ([]byte, error) {
	if f.err != nil {
		return nil, &errtype.FetchError{Code: errtype.CodeFetchRequestFailed, URL: rawURL, Message: "请求上游失败：" + f.err.Error(), Cause: f.err}
	}
	body, ok := f.responses[rawURL]
	if !ok {
		return nil, &errtype.FetchError{Code: errtype.CodeFetchStatusInvalid, URL: rawURL, Message: "上游返回 HTTP 404"}
	}
	return body, nil
}

func makeSubResponse(uris ...string) []byte {
	joined := strings.Join(uris, "\n")
	return []byte(base64.StdEncoding.EncodeToString([]byte(joined)))
}

func mustParseOrderedMapGroups(t *testing.T, yamlStr string) config.OrderedMap[config.Group] {
	t.Helper()
	var m config.OrderedMap[config.Group]
	if err := yaml.Unmarshal([]byte(yamlStr), &m); err != nil {
		t.Fatalf("mustParseOrderedMapGroups: %v", err)
	}
	return m
}

func mustParseOrderedMapStrings(t *testing.T, yamlStr string) config.OrderedMap[[]string] {
	t.Helper()
	var m config.OrderedMap[[]string]
	if err := yaml.Unmarshal([]byte(yamlStr), &m); err != nil {
		t.Fatalf("mustParseOrderedMapStrings: %v", err)
	}
	return m
}

const subURL = "https://sub.example.com/api"
const accessToken = "secret-token"

// validConfig returns a minimal config that produces a valid Pipeline.
func validConfig(t *testing.T) *config.Config {
	t.Helper()
	return &config.Config{
		Sources: config.Sources{
			Subscriptions: []config.Subscription{{URL: subURL}},
		},
		Groups:   mustParseOrderedMapGroups(t, `"HK": { match: "(HK)", strategy: select }`),
		Routing:  mustParseOrderedMapStrings(t, `"proxy": ["HK", "DIRECT"]`),
		Rulesets: mustParseOrderedMapStrings(t, "\"proxy\":\n  - \"https://example.com/rules.list\""),
		Rules:    []string{"GEOIP,CN,proxy"},
		Fallback: "proxy",
		BaseURL:  "https://my-server.com",
	}
}

// validFetcher returns a fetcher that serves valid subscription data.
func validFetcher() *fakeFetcher {
	return &fakeFetcher{
		responses: map[string][]byte{
			subURL: makeSubResponse(
				"ss://YWVzLTI1Ni1jZmI6cGFzcw@hk.example.com:8388#HK-01",
				"ss://YWVzLTI1Ni1jZmI6cGFzcw@sg.example.com:8388#SG-01",
			),
		},
	}
}

// startTestServer creates and starts a test HTTP server.
func startTestServer(t *testing.T, cfg *config.Config, f *fakeFetcher) *httptest.Server {
	t.Helper()
	srv := server.New(cfg, f, server.Options{})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func startTestServerWithOptions(t *testing.T, cfg *config.Config, f *fakeFetcher, opts server.Options) *httptest.Server {
	t.Helper()
	srv := server.New(cfg, f, opts)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts
}

// --- E2E tests ---

// T-E2E-001: HTTP generate Clash success
func TestE2E_GenerateClash(t *testing.T) {
	ts := startTestServer(t, validConfig(t), validFetcher())

	resp, err := http.Get(ts.URL + "/generate?format=clash")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200; body: %s", resp.StatusCode, body)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "text/yaml; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", ct, "text/yaml; charset=utf-8")
	}
	if cd := resp.Header.Get("Content-Disposition"); cd != `attachment; filename="clash.yaml"; filename*=UTF-8''clash.yaml` {
		t.Errorf("Content-Disposition = %q", cd)
	}

	body, _ := io.ReadAll(resp.Body)
	var doc struct {
		Proxies       []any    `yaml:"proxies"`
		ProxyGroups   []any    `yaml:"proxy-groups"`
		Rules         []string `yaml:"rules"`
		RuleProviders map[string]struct {
			Format string `yaml:"format"`
		} `yaml:"rule-providers"`
	}
	if err := yaml.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse clash response yaml: %v", err)
	}
	if len(doc.Proxies) == 0 {
		t.Error("response should contain proxies")
	}
	if len(doc.ProxyGroups) == 0 {
		t.Error("response should contain proxy-groups")
	}
	if len(doc.Rules) == 0 {
		t.Error("response should contain rules")
	}
	if len(doc.RuleProviders) == 0 {
		t.Fatal("response should contain rule-providers")
	}
	for name, provider := range doc.RuleProviders {
		if provider.Format != "text" {
			t.Errorf("rule-provider %q format = %q, want %q", name, provider.Format, "text")
		}
	}
}

// T-E2E-002: HTTP generate Surge success
func TestE2E_GenerateSurge(t *testing.T) {
	ts := startTestServer(t, validConfig(t), validFetcher())

	resp, err := http.Get(ts.URL + "/generate?format=surge")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200; body: %s", resp.StatusCode, body)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", ct, "text/plain; charset=utf-8")
	}
	if cd := resp.Header.Get("Content-Disposition"); cd != `attachment; filename="surge.conf"; filename*=UTF-8''surge.conf` {
		t.Errorf("Content-Disposition = %q", cd)
	}

	body, _ := io.ReadAll(resp.Body)
	content := string(body)
	for _, want := range []string{"[Proxy]", "[Proxy Group]", "[Rule]", "#!MANAGED-CONFIG"} {
		if !strings.Contains(content, want) {
			t.Errorf("response body missing %q", want)
		}
	}
	if !strings.Contains(content, "https://my-server.com/generate?format=surge&filename=surge.conf") {
		t.Errorf("managed config header missing default filename: %s", content)
	}
}

func TestE2E_GenerateWithCustomFilename(t *testing.T) {
	ts := startTestServer(t, validConfig(t), validFetcher())

	resp, err := http.Get(ts.URL + "/generate?format=clash&filename=my-profile")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200; body: %s", resp.StatusCode, body)
	}
	if cd := resp.Header.Get("Content-Disposition"); cd != `attachment; filename="my-profile.yaml"; filename*=UTF-8''my-profile.yaml` {
		t.Errorf("Content-Disposition = %q", cd)
	}
}

func TestE2E_GenerateSurgeWithTokenAndCustomFilename(t *testing.T) {
	ts := startTestServerWithOptions(t, validConfig(t), validFetcher(), server.Options{AccessToken: accessToken})

	resp, err := http.Get(ts.URL + "/generate?format=surge&token=" + accessToken + "&filename=my-profile")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200; body: %s", resp.StatusCode, body)
	}
	if cd := resp.Header.Get("Content-Disposition"); cd != `attachment; filename="my-profile.conf"; filename*=UTF-8''my-profile.conf` {
		t.Errorf("Content-Disposition = %q", cd)
	}
	body, _ := io.ReadAll(resp.Body)
	content := string(body)
	if !strings.Contains(content, "https://my-server.com/generate?format=surge&token=secret-token&filename=my-profile.conf") {
		t.Errorf("managed config header missing token or filename: %s", content)
	}
}

// T-E2E-003: Invalid format returns 400
func TestE2E_InvalidFormat(t *testing.T) {
	ts := startTestServer(t, validConfig(t), validFetcher())

	cases := []struct {
		name string
		url  string
	}{
		{"bad format", ts.URL + "/generate?format=v2ray"},
		{"missing format", ts.URL + "/generate"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Get(tc.url)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", resp.StatusCode)
			}
		})
	}
}

func TestE2E_InvalidFilename(t *testing.T) {
	ts := startTestServer(t, validConfig(t), validFetcher())

	cases := []struct {
		name string
		url  string
	}{
		{"empty", ts.URL + "/generate?format=clash&filename="},
		{"wrong extension", ts.URL + "/generate?format=surge&filename=profile.yaml"},
		{"path separator", ts.URL + "/generate?format=clash&filename=../secret"},
		{"unicode", ts.URL + "/generate?format=clash&filename=%E9%85%8D%E7%BD%AE"},
		{"space", ts.URL + "/generate?format=clash&filename=my%20profile"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Get(tc.url)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusBadRequest {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("status = %d, want 400; body: %s", resp.StatusCode, body)
			}
			body, _ := io.ReadAll(resp.Body)
			if !strings.Contains(string(body), "filename 参数无效") {
				t.Errorf("body = %q, want filename 参数无效", body)
			}
		})
	}
}

func TestE2E_TokenRequired(t *testing.T) {
	ts := startTestServerWithOptions(t, validConfig(t), validFetcher(), server.Options{AccessToken: accessToken})

	cases := []struct {
		name string
		url  string
	}{
		{"missing token", ts.URL + "/generate?format=clash"},
		{"wrong token", ts.URL + "/generate?format=clash&token=wrong-token"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Get(tc.url)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusUnauthorized {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("status = %d, want 401; body: %s", resp.StatusCode, body)
			}
			body, _ := io.ReadAll(resp.Body)
			if !strings.Contains(string(body), "访问令牌缺失或无效") {
				t.Errorf("body = %q, want 访问令牌缺失或无效", body)
			}
		})
	}
}

// T-E2E-004: Subscription fetch failure returns 502
func TestE2E_FetchFailure(t *testing.T) {
	f := &fakeFetcher{err: errors.New("connection refused")}
	ts := startTestServer(t, validConfig(t), f)

	resp, err := http.Get(ts.URL + "/generate?format=clash")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "拉取失败") {
		t.Errorf("body = %q, want 拉取失败", body)
	}
}

// T-E2E-005: Graph validation error returns 400
func TestE2E_BuildError(t *testing.T) {
	// Config passes static validation but fails graph validation:
	// routing references "NONEXISTENT" which is not a node group, route group, or reserved policy.
	cfg := validConfig(t)
	cfg.Routing = mustParseOrderedMapStrings(t, `"proxy": ["NONEXISTENT", "DIRECT"]`)

	ts := startTestServer(t, cfg, validFetcher())

	resp, err := http.Get(ts.URL + "/generate?format=clash")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("status = %d, want 400; body: %s", resp.StatusCode, body)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "构建错误") {
		t.Errorf("body = %q, want 构建错误", body)
	}
}

// T-E2E-005b: Invalid subscription content returns 502
func TestE2E_InvalidSubscriptionContent(t *testing.T) {
	f := &fakeFetcher{responses: map[string][]byte{subURL: []byte("")}}
	ts := startTestServer(t, validConfig(t), f)

	resp, err := http.Get(ts.URL + "/generate?format=clash")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadGateway {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("status = %d, want 502; body: %s", resp.StatusCode, body)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "拉取失败") {
		t.Errorf("body = %q, want 拉取失败", body)
	}
}

// T-E2E-006: /healthz returns 200
func TestE2E_Healthz(t *testing.T) {
	ts := startTestServer(t, validConfig(t), validFetcher())

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Errorf("body = %q, want %q", body, "ok")
	}
}

// T-E2E-007: Missing local template should be treated as server error.
func TestE2E_LocalTemplateReadError(t *testing.T) {
	cfg := validConfig(t)
	cfg.Templates.Clash = "/nonexistent/path/base_clash.yaml"

	ts := startTestServer(t, cfg, validFetcher())

	resp, err := http.Get(ts.URL + "/generate?format=clash")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusInternalServerError {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("status = %d, want 500; body: %s", resp.StatusCode, body)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "资源读取失败") {
		t.Errorf("body = %q, want containing %q", body, "资源读取失败")
	}
	if !strings.Contains(string(body), cfg.Templates.Clash) {
		t.Errorf("body = %q, want containing %q", body, cfg.Templates.Clash)
	}
}
