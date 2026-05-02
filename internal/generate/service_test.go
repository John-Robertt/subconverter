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
		return nil, errors.New("missing response for " + rawURL)
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
			Subscriptions: []config.Subscription{{URL: testSubURL}},
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
		t.Fatalf("Prepare() error = %v", err)
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

func TestServiceGenerateClash(t *testing.T) {
	rt := mustRuntimeConfig(t, validConfig(t))
	svc := New(validFetcher(), Options{})

	result, err := svc.Generate(context.Background(), rt, Request{
		Format:   "clash",
		Filename: "clash.yaml",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
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
			t.Errorf("body missing %q", want)
		}
	}
}

func TestServiceGenerateSurgeManagedURL(t *testing.T) {
	rt := mustRuntimeConfig(t, validConfig(t))
	svc := New(validFetcher(), Options{AccessToken: "secret-token"})

	result, err := svc.Generate(context.Background(), rt, Request{
		Format:   "surge",
		Filename: "my-profile.conf",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if result.ContentType != "text/plain; charset=utf-8" {
		t.Fatalf("ContentType = %q", result.ContentType)
	}
	if result.Filename != "my-profile.conf" {
		t.Fatalf("Filename = %q", result.Filename)
	}
	body := string(result.Body)
	if !strings.Contains(body, "#!MANAGED-CONFIG https://my-server.com/generate?format=surge&token=secret-token&filename=my-profile.conf interval=86400 strict=false") {
		t.Fatalf("managed header missing or incorrect: %s", body)
	}
}

func TestServiceGenerateUnsupportedFormat(t *testing.T) {
	rt := mustRuntimeConfig(t, validConfig(t))
	svc := New(validFetcher(), Options{})

	_, err := svc.Generate(context.Background(), rt, Request{
		Format:   "quantumultx",
		Filename: "qx.conf",
	})
	if err == nil {
		t.Fatal("expected unsupported format error")
	}
	if !strings.Contains(err.Error(), `unsupported format "quantumultx"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
