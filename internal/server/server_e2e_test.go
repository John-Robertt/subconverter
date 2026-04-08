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
		return nil, &errtype.FetchError{URL: rawURL, Message: f.err.Error(), Cause: f.err}
	}
	body, ok := f.responses[rawURL]
	if !ok {
		return nil, &errtype.FetchError{URL: rawURL, Message: "not found"}
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
	srv := server.New(cfg, f)
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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200; body: %s", resp.StatusCode, body)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "text/yaml; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", ct, "text/yaml; charset=utf-8")
	}

	body, _ := io.ReadAll(resp.Body)
	content := string(body)
	for _, want := range []string{"proxies:", "proxy-groups:", "rules:"} {
		if !strings.Contains(content, want) {
			t.Errorf("response body missing %q", want)
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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200; body: %s", resp.StatusCode, body)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", ct, "text/plain; charset=utf-8")
	}

	body, _ := io.ReadAll(resp.Body)
	content := string(body)
	for _, want := range []string{"[Proxy]", "[Proxy Group]", "[Rule]", "#!MANAGED-CONFIG"} {
		if !strings.Contains(content, want) {
			t.Errorf("response body missing %q", want)
		}
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
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", resp.StatusCode)
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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", resp.StatusCode)
	}
}

// T-E2E-005: Build error returns 500
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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("status = %d, want 500; body: %s", resp.StatusCode, body)
	}
}

// T-E2E-006: /healthz returns 200
func TestE2E_Healthz(t *testing.T) {
	ts := startTestServer(t, validConfig(t), validFetcher())

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("status = %d, want 500; body: %s", resp.StatusCode, body)
	}
}
