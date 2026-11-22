package generate

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/config"
	"gopkg.in/yaml.v3"
)

const testSubURL = "https://sub.example.com/api"

type fakeFetcher struct {
	responses map[string][]byte
}

func (f *fakeFetcher) Fetch(_ context.Context, rawURL string) ([]byte, error) {
	body, ok := f.responses[rawURL]
	if !ok {
		return nil, errors.New("缺少该 URL 的响应：" + rawURL)
	}
	return body, nil
}

func makeSubResponse(uris ...string) []byte {
	return []byte(base64.StdEncoding.EncodeToString([]byte(strings.Join(uris, "\n"))))
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

func validConfig(t *testing.T) *config.Config {
	t.Helper()
	return &config.Config{
		Sources: config.Sources{
			Subscriptions: []config.Source{{URL: testSubURL}},
		},
		Groups:   mustGroupsMap(t, `"HK": { match: "(HK)", strategy: select }`),
		Routing:  mustRoutingMap(t, `"proxy": ["HK", "DIRECT"]`),
		Rulesets: mustRoutingMap(t, "\"proxy\":\n  - \"https://example.com/rules.list\""),
		Rules:    []string{"GEOIP,CN,proxy"},
		Fallback: "proxy",
		BaseURL:  "https://my-server.com",
	}
}

func mustRuntimeConfig(t *testing.T, cfg *config.Config) *config.RuntimeConfig {
	t.Helper()
	rt, err := config.Prepare(cfg)
	if err != nil {
		t.Fatalf("Prepare() 出错：%v", err)
	}
	return rt
}

func validFetcher() *fakeFetcher {
	return &fakeFetcher{
		responses: map[string][]byte{
			testSubURL: makeSubResponse(
				"ss://YWVzLTI1Ni1jZmI6cGFzcw@hk.example.com:8388#HK-01",
				"ss://YWVzLTI1Ni1jZmI6cGFzcw@sg.example.com:8388#SG-01",
			),
		},
	}
}

// T-GEN-001：服务生成内容正确的 Clash 输出
func TestServiceGenerateClash(t *testing.T) {
	rt := mustRuntimeConfig(t, validConfig(t))
	svc := New(validFetcher(), Options{})

	result, err := svc.Generate(context.Background(), rt, Request{
		Format:   "clash",
		Filename: "clash.yaml",
	})
	if err != nil {
		t.Fatalf("Generate() 出错：%v", err)
	}
	if result.ContentType != "text/yaml; charset=utf-8" {
		t.Fatalf("ContentType = %q", result.ContentType)
	}
	if result.Filename != "clash.yaml" {
		t.Fatalf("Filename = %q", result.Filename)
	}
	body := string(result.Body)
	for _, want := range []string{"proxies:", "proxy-groups:", "rule-providers:", "rules:"} {
		if !strings.Contains(body, want) {
			t.Errorf("body 缺少 %q", want)
		}
	}
}

// T-GEN-002：服务生成带托管 URL 头的 Surge 输出
func TestServiceGenerateSurgeManagedURL(t *testing.T) {
	rt := mustRuntimeConfig(t, validConfig(t))
	svc := New(validFetcher(), Options{AccessToken: "secret-token"})

	result, err := svc.Generate(context.Background(), rt, Request{
		Format:   "surge",
		Filename: "my-profile.conf",
	})
	if err != nil {
		t.Fatalf("Generate() 出错：%v", err)
	}
	if result.ContentType != "text/plain; charset=utf-8" {
		t.Fatalf("ContentType = %q", result.ContentType)
	}
	if result.Filename != "my-profile.conf" {
		t.Fatalf("Filename = %q", result.Filename)
	}
	body := string(result.Body)
	if !strings.Contains(body, "#!MANAGED-CONFIG https://my-server.com/generate?format=surge&token=secret-token&filename=my-profile.conf interval=86400 strict=false") {
		t.Fatalf("托管头缺失或不正确：%s", body)
	}
}

// T-GEN-003：格式不受支持时服务返回错误
func TestServiceGenerateUnsupportedFormat(t *testing.T) {
	rt := mustRuntimeConfig(t, validConfig(t))
	svc := New(validFetcher(), Options{})

	_, err := svc.Generate(context.Background(), rt, Request{
		Format:   "quantumultx",
		Filename: "qx.conf",
	})
	if err == nil {
		t.Fatal("应返回格式不受支持的错误")
	}
	if !strings.Contains(err.Error(), `不支持的格式 "quantumultx"`) {
		t.Fatalf("意外错误：%v", err)
	}
}

// T-GEN-DOC-001：候选名相同的 Clash YAML 订阅和 Surge CONF 订阅走完
// build/target/render 全流程。名字分配发生在按格式过滤之后，
// 所以每个目标都保留原始名字。
func TestServiceGenerateStructuredSubscriptionsByTarget(t *testing.T) {
	const clashSubURL = "https://sub.example.com/nodes.yaml?token=clash-secret"
	const surgeSubURL = "https://sub.example.com/nodes.conf?token=surge-secret"

	cfg := &config.Config{
		Sources: config.Sources{
			Subscriptions: []config.Source{
				{URL: clashSubURL},
				{URL: surgeSubURL},
			},
		},
		Groups:   mustGroupsMap(t, `"ALL": { match: "(SHARED)", strategy: select }`),
		Routing:  mustRoutingMap(t, `"proxy": ["ALL", "DIRECT"]`),
		Fallback: "proxy",
	}
	rt := mustRuntimeConfig(t, cfg)
	svc := New(&fakeFetcher{responses: map[string][]byte{
		clashSubURL: []byte(`proxies:
  - name: SHARED
    type: trojan
    server: hk.example.com
    port: 443
    password: clash-password
    future-option: enabled
`),
		surgeSubURL: []byte(`[Proxy]
SHARED = trojan, sg.example.com, 443, password=surge-password, future-option=enabled
`),
	}}, Options{})

	clashResult, err := svc.Generate(context.Background(), rt, Request{
		Format:   "clash",
		Filename: "clash.yaml",
	})
	if err != nil {
		t.Fatalf("Generate(clash) 出错：%v", err)
	}
	clashBody := string(clashResult.Body)
	for _, want := range []string{"name: SHARED", "future-option: enabled", "ALL"} {
		if !strings.Contains(clashBody, want) {
			t.Errorf("Clash body 缺少 %q：\n%s", want, clashBody)
		}
	}
	if strings.Contains(clashBody, "SHARED②") || strings.Contains(clashBody, "surge-password") {
		t.Errorf("Clash body 泄露了 Surge CONF 节点：\n%s", clashBody)
	}

	surgeResult, err := svc.Generate(context.Background(), rt, Request{
		Format:   "surge",
		Filename: "surge.conf",
	})
	if err != nil {
		t.Fatalf("Generate(surge) 出错：%v", err)
	}
	surgeBody := string(surgeResult.Body)
	wantSurgeProxy := "SHARED = trojan, sg.example.com, 443, password=surge-password, future-option=enabled"
	if !strings.Contains(surgeBody, wantSurgeProxy) {
		t.Errorf("Surge body 缺少原样复制的节点声明：\n%s", surgeBody)
	}
	if strings.Contains(surgeBody, "SHARED②") || strings.Contains(surgeBody, "clash-password") {
		t.Errorf("Surge body 泄露了 Clash YAML 节点：\n%s", surgeBody)
	}
}
