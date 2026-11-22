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

	"github.com/John-Robertt/subconverter/internal/app"
	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/generate"
	"github.com/John-Robertt/subconverter/internal/server"
	"gopkg.in/yaml.v3"
)

// --- 测试辅助函数 ---

// fakeFetcher 把 URL 映射到预设的响应。
type fakeFetcher struct {
	responses map[string][]byte
	err       error // 设置后，所有拉取都会失败
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

// validConfig 返回一份能生成有效 Pipeline 的最小配置。
func validConfig(t *testing.T) *config.Config {
	t.Helper()
	return &config.Config{
		Sources: config.Sources{
			Subscriptions: []config.Source{{URL: subURL}},
		},
		Groups:   mustParseOrderedMapGroups(t, `"HK": { match: "(HK)", strategy: select }`),
		Routing:  mustParseOrderedMapStrings(t, `"proxy": ["HK", "DIRECT"]`),
		Rulesets: mustParseOrderedMapStrings(t, "\"proxy\":\n  - \"https://example.com/rules.list\""),
		Rules:    []string{"GEOIP,CN,proxy"},
		Fallback: "proxy",
		BaseURL:  "https://my-server.com",
	}
}

func mustRuntimeConfig(t *testing.T, cfg *config.Config) *config.RuntimeConfig {
	t.Helper()
	rt, err := config.Prepare(cfg)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	return rt
}

// validFetcher 返回一个提供有效订阅数据的 fetcher。
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

// startTestServer 创建并启动一个测试用 HTTP 服务。
func startTestServer(t *testing.T, cfg *config.Config, f *fakeFetcher) *httptest.Server {
	t.Helper()
	appSvc := app.NewWithRuntime("", mustRuntimeConfig(t, cfg), f, generate.Options{})
	srv := server.New(appSvc, server.Options{})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func startTestServerWithOptions(t *testing.T, cfg *config.Config, f *fakeFetcher, opts server.Options) *httptest.Server {
	t.Helper()
	appSvc := app.NewWithRuntime("", mustRuntimeConfig(t, cfg), f, generate.Options{AccessToken: opts.AccessToken})
	srv := server.New(appSvc, opts)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts
}

// --- 端到端测试 ---

// T-E2E-LOON-001：HTTP 生成接口接受 Loon，应用配置中的分组，并使用 Loon 的默认文件名。
func TestE2E_GenerateLoon(t *testing.T) {
	cfg := validConfig(t)
	cfg.Sources.Subscriptions[0].Format = "loon"
	f := &fakeFetcher{responses: map[string][]byte{
		subURL: []byte("[Proxy]\nHK-LOON = anytls, node.example.com, 443, \"pw\", tls-name=edge.example.com\n[Proxy Group]\nPROVIDER = select, MISSING\n"),
	}}
	ts := startTestServer(t, cfg, f)
	resp, err := ts.Client().Get(ts.URL + "/generate?format=loon")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || !strings.Contains(resp.Header.Get("Content-Disposition"), "loon.lcf") {
		t.Fatalf("status=%d headers=%v body=%s", resp.StatusCode, resp.Header, body)
	}
	for _, want := range []string{"HK = select, HK-LOON", "[Remote Rule]", "https://example.com/rules.list,proxy", "FINAL,proxy"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("Loon 输出缺少 %q：\n%s", want, body)
		}
	}
	if strings.Contains(string(body), "PROVIDER") || strings.Contains(string(body), "MANAGED-CONFIG") {
		t.Fatalf("Loon 输出包含了订阅提供方的分组或 Surge 文件头：\n%s", body)
	}
}

// T-E2E-001：HTTP 成功生成 Clash 配置
func TestE2E_GenerateClash(t *testing.T) {
	ts := startTestServer(t, validConfig(t), validFetcher())

	resp, err := http.Get(ts.URL + "/generate?format=clash")
	if err != nil {
		t.Fatalf("请求失败：%v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d，期望 200；body: %s", resp.StatusCode, body)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "text/yaml; charset=utf-8" {
		t.Errorf("Content-Type = %q，期望 %q", ct, "text/yaml; charset=utf-8")
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
		t.Fatalf("解析 Clash 响应 YAML 失败：%v", err)
	}
	if len(doc.Proxies) == 0 {
		t.Error("响应应包含 proxies")
	}
	if len(doc.ProxyGroups) == 0 {
		t.Error("响应应包含 proxy-groups")
	}
	if len(doc.Rules) == 0 {
		t.Error("响应应包含 rules")
	}
	if len(doc.RuleProviders) == 0 {
		t.Fatal("响应应包含 rule-providers")
	}
	for name, provider := range doc.RuleProviders {
		if provider.Format != "text" {
			t.Errorf("rule-provider %q 的 format = %q，期望 %q", name, provider.Format, "text")
		}
	}
}

// T-E2E-002：HTTP 成功生成 Surge 配置
func TestE2E_GenerateSurge(t *testing.T) {
	ts := startTestServer(t, validConfig(t), validFetcher())

	resp, err := http.Get(ts.URL + "/generate?format=surge")
	if err != nil {
		t.Fatalf("请求失败：%v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d，期望 200；body: %s", resp.StatusCode, body)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q，期望 %q", ct, "text/plain; charset=utf-8")
	}
	if cd := resp.Header.Get("Content-Disposition"); cd != `attachment; filename="surge.conf"; filename*=UTF-8''surge.conf` {
		t.Errorf("Content-Disposition = %q", cd)
	}

	body, _ := io.ReadAll(resp.Body)
	content := string(body)
	for _, want := range []string{"[Proxy]", "[Proxy Group]", "[Rule]", "#!MANAGED-CONFIG"} {
		if !strings.Contains(content, want) {
			t.Errorf("响应 body 缺少 %q", want)
		}
	}
	if !strings.Contains(content, "https://my-server.com/generate?format=surge&filename=surge.conf") {
		t.Errorf("managed config 头缺少默认文件名：%s", content)
	}
}

func TestE2E_GenerateWithCustomFilename(t *testing.T) {
	ts := startTestServer(t, validConfig(t), validFetcher())

	resp, err := http.Get(ts.URL + "/generate?format=clash&filename=my-profile")
	if err != nil {
		t.Fatalf("请求失败：%v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d，期望 200；body: %s", resp.StatusCode, body)
	}
	if cd := resp.Header.Get("Content-Disposition"); cd != `attachment; filename="my-profile.yaml"; filename*=UTF-8''my-profile.yaml` {
		t.Errorf("Content-Disposition = %q", cd)
	}
}

func TestE2E_GenerateSurgeWithTokenAndCustomFilename(t *testing.T) {
	ts := startTestServerWithOptions(t, validConfig(t), validFetcher(), server.Options{AccessToken: accessToken})

	resp, err := http.Get(ts.URL + "/generate?format=surge&token=" + accessToken + "&filename=my-profile")
	if err != nil {
		t.Fatalf("请求失败：%v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d，期望 200；body: %s", resp.StatusCode, body)
	}
	if cd := resp.Header.Get("Content-Disposition"); cd != `attachment; filename="my-profile.conf"; filename*=UTF-8''my-profile.conf` {
		t.Errorf("Content-Disposition = %q", cd)
	}
	body, _ := io.ReadAll(resp.Body)
	content := string(body)
	if !strings.Contains(content, "https://my-server.com/generate?format=surge&token=secret-token&filename=my-profile.conf") {
		t.Errorf("managed config 头缺少 token 或文件名：%s", content)
	}
}

func TestE2E_GenerateSurgeWithAdminSessionUsesServerTokenInManagedURL(t *testing.T) {
	ts := startTestServerWithOptions(t, validConfig(t), validFetcher(), server.Options{
		AccessToken: accessToken,
		AdminSessionValidator: func(*http.Request) bool {
			return true
		},
	})

	resp, err := http.Get(ts.URL + "/generate?format=surge&filename=admin-profile")
	if err != nil {
		t.Fatalf("请求失败：%v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d，期望 200；body: %s", resp.StatusCode, body)
	}
	body, _ := io.ReadAll(resp.Body)
	content := string(body)
	if !strings.Contains(content, "https://my-server.com/generate?format=surge&token=secret-token&filename=admin-profile.conf") {
		t.Errorf("管理后台会话下的 managed config 应使用服务端 token 和文件名：%s", content)
	}
}

// T-E2E-003：无效的 format 返回 400
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
				t.Fatalf("请求失败：%v", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("status = %d，期望 400", resp.StatusCode)
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
				t.Fatalf("请求失败：%v", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusBadRequest {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("status = %d，期望 400；body: %s", resp.StatusCode, body)
			}
			body, _ := io.ReadAll(resp.Body)
			if !strings.Contains(string(body), "filename 参数无效") {
				t.Errorf("body = %q，期望包含 filename 参数无效", body)
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
				t.Fatalf("请求失败：%v", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusUnauthorized {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("status = %d，期望 401；body: %s", resp.StatusCode, body)
			}
			body, _ := io.ReadAll(resp.Body)
			if !strings.Contains(string(body), "访问令牌缺失或无效") {
				t.Errorf("body = %q，期望包含 访问令牌缺失或无效", body)
			}
		})
	}
}

// T-E2E-004：拉取来源失败时返回 502
func TestE2E_FetchFailure(t *testing.T) {
	f := &fakeFetcher{err: errors.New("connection refused")}
	ts := startTestServer(t, validConfig(t), f)

	resp, err := http.Get(ts.URL + "/generate?format=clash")
	if err != nil {
		t.Fatalf("请求失败：%v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d，期望 502", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "拉取失败") {
		t.Errorf("body = %q，期望包含 拉取失败", body)
	}
}

// T-E2E-005：动态关系图校验失败时返回 400
func TestE2E_BuildError(t *testing.T) {
	// 配置能通过启动时的预处理，但在请求时的关系图校验中失败：
	// HK 节点组在当前远程结果中为空。
	cfg := validConfig(t)
	cfg.Sources.Subscriptions = []config.Source{{URL: subURL}}
	cfg.Groups = mustParseOrderedMapGroups(t, `"HK": { match: "(HK)", strategy: select }`)
	cfg.Routing = mustParseOrderedMapStrings(t, `"proxy": ["HK", "DIRECT"]`)
	cfg.Fallback = "proxy"

	f := &fakeFetcher{
		responses: map[string][]byte{
			subURL: makeSubResponse("ss://YWVzLTI1Ni1jZmI6cGFzcw@sg.example.com:8388#SG-01"),
		},
	}
	ts := startTestServer(t, cfg, f)

	resp, err := http.Get(ts.URL + "/generate?format=clash")
	if err != nil {
		t.Fatalf("请求失败：%v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("status = %d，期望 400；body: %s", resp.StatusCode, body)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "构建错误") {
		t.Errorf("body = %q，期望包含 构建错误", body)
	}
}

// T-E2E-005b：订阅内容无效时返回 502
func TestE2E_InvalidSubscriptionContent(t *testing.T) {
	f := &fakeFetcher{responses: map[string][]byte{subURL: []byte("")}}
	ts := startTestServer(t, validConfig(t), f)

	resp, err := http.Get(ts.URL + "/generate?format=clash")
	if err != nil {
		t.Fatalf("请求失败：%v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadGateway {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("status = %d，期望 502；body: %s", resp.StatusCode, body)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "拉取失败") {
		t.Errorf("body = %q，期望包含 拉取失败", body)
	}
}

// T-E2E-006：/healthz 返回 200
func TestE2E_Healthz(t *testing.T) {
	ts := startTestServer(t, validConfig(t), validFetcher())

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("请求失败：%v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d，期望 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Errorf("body = %q，期望 %q", body, "ok")
	}
}

// T-E2E-007：本地模板缺失应按服务端错误处理。
func TestE2E_LocalTemplateReadError(t *testing.T) {
	cfg := validConfig(t)
	cfg.Templates.Clash = "/nonexistent/path/base_clash.yaml"

	ts := startTestServer(t, cfg, validFetcher())

	resp, err := http.Get(ts.URL + "/generate?format=clash")
	if err != nil {
		t.Fatalf("请求失败：%v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusInternalServerError {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("status = %d，期望 500；body: %s", resp.StatusCode, body)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "资源读取失败") {
		t.Errorf("body = %q，期望包含 %q", body, "资源读取失败")
	}
	if !strings.Contains(string(body), cfg.Templates.Clash) {
		t.Errorf("body = %q，期望包含 %q", body, cfg.Templates.Clash)
	}
}
