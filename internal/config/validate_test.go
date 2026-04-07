package config

import (
	"errors"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"gopkg.in/yaml.v3"
)

// validBase returns a minimal valid Config for mutation in tests.
func validBase() Config {
	var cfg Config
	cfg.Sources.Subscriptions = []Subscription{{URL: "https://example.com/sub"}}
	cfg.Groups = mustOrderedMap[Group](`HK: { match: "(HK)", strategy: select }`)
	cfg.Routing = mustOrderedMap[[]string](`proxy: [HK, DIRECT]`)
	cfg.Rulesets = mustOrderedMap[[]string](`proxy: ["https://example.com/rules.list"]`)
	cfg.Rules = []string{"GEOIP,CN,proxy"}
	cfg.Fallback = "proxy"
	return cfg
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := validBase()
	if err := Validate(&cfg); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidate_MissingSubscriptionURL(t *testing.T) {
	cfg := validBase()
	cfg.Sources.Subscriptions = []Subscription{{URL: ""}}
	assertFieldError(t, Validate(&cfg), "sources.subscriptions[0].url")
}

func TestValidate_MissingCustomProxyName(t *testing.T) {
	cfg := validBase()
	cfg.Sources.CustomProxies = []CustomProxy{{
		Type: "socks5", Server: "1.2.3.4", Port: 1080,
	}}
	assertFieldError(t, Validate(&cfg), "sources.custom_proxies[0].name")
}

func TestValidate_MissingCustomProxyType(t *testing.T) {
	cfg := validBase()
	cfg.Sources.CustomProxies = []CustomProxy{{
		Name: "p1", Server: "1.2.3.4", Port: 1080,
	}}
	assertFieldError(t, Validate(&cfg), "sources.custom_proxies[0].type")
}

func TestValidate_InvalidCustomProxyType(t *testing.T) {
	cfg := validBase()
	cfg.Sources.CustomProxies = []CustomProxy{{
		Name: "p1", Type: "vmess", Server: "1.2.3.4", Port: 1080,
	}}
	assertFieldError(t, Validate(&cfg), "sources.custom_proxies[0].type")
}

func TestValidate_MissingCustomProxyServer(t *testing.T) {
	cfg := validBase()
	cfg.Sources.CustomProxies = []CustomProxy{{
		Name: "p1", Type: "socks5", Port: 1080,
	}}
	assertFieldError(t, Validate(&cfg), "sources.custom_proxies[0].server")
}

func TestValidate_InvalidCustomProxyPort(t *testing.T) {
	cfg := validBase()
	cfg.Sources.CustomProxies = []CustomProxy{{
		Name: "p1", Type: "socks5", Server: "1.2.3.4", Port: 0,
	}}
	assertFieldError(t, Validate(&cfg), "sources.custom_proxies[0].port")
}

func TestValidate_DuplicateCustomProxyNames(t *testing.T) {
	cfg := validBase()
	cfg.Sources.CustomProxies = []CustomProxy{
		{Name: "dup", Type: "socks5", Server: "1.2.3.4", Port: 1080},
		{Name: "dup", Type: "http", Server: "5.6.7.8", Port: 8080},
	}
	assertFieldError(t, Validate(&cfg), "sources.custom_proxies[1].name")
}

func TestValidate_RelayThroughMissingStrategy(t *testing.T) {
	// T-CFG-004
	cfg := validBase()
	cfg.Sources.CustomProxies = []CustomProxy{{
		Name: "p1", Type: "socks5", Server: "1.2.3.4", Port: 1080,
		RelayThrough: &RelayThrough{Type: "all"},
	}}
	assertFieldError(t, Validate(&cfg), "relay_through.strategy")
}

func TestValidate_RelayThroughInvalidStrategy(t *testing.T) {
	cfg := validBase()
	cfg.Sources.CustomProxies = []CustomProxy{{
		Name: "p1", Type: "socks5", Server: "1.2.3.4", Port: 1080,
		RelayThrough: &RelayThrough{Type: "all", Strategy: "random"},
	}}
	assertFieldError(t, Validate(&cfg), "relay_through.strategy")
}

func TestValidate_RelayThroughMissingType(t *testing.T) {
	cfg := validBase()
	cfg.Sources.CustomProxies = []CustomProxy{{
		Name: "p1", Type: "socks5", Server: "1.2.3.4", Port: 1080,
		RelayThrough: &RelayThrough{Strategy: "select"},
	}}
	assertFieldError(t, Validate(&cfg), "relay_through.type")
}

func TestValidate_RelayThroughInvalidType(t *testing.T) {
	cfg := validBase()
	cfg.Sources.CustomProxies = []CustomProxy{{
		Name: "p1", Type: "socks5", Server: "1.2.3.4", Port: 1080,
		RelayThrough: &RelayThrough{Type: "invalid", Strategy: "select"},
	}}
	assertFieldError(t, Validate(&cfg), "relay_through.type")
}

func TestValidate_RelayThroughGroupMissingName(t *testing.T) {
	cfg := validBase()
	cfg.Sources.CustomProxies = []CustomProxy{{
		Name: "p1", Type: "socks5", Server: "1.2.3.4", Port: 1080,
		RelayThrough: &RelayThrough{Type: "group", Strategy: "select"},
	}}
	assertFieldError(t, Validate(&cfg), "relay_through.name")
}

func TestValidate_RelayThroughSelectMissingMatch(t *testing.T) {
	cfg := validBase()
	cfg.Sources.CustomProxies = []CustomProxy{{
		Name: "p1", Type: "socks5", Server: "1.2.3.4", Port: 1080,
		RelayThrough: &RelayThrough{Type: "select", Strategy: "select"},
	}}
	assertFieldError(t, Validate(&cfg), "relay_through.match")
}

func TestValidate_RelayThroughSelectInvalidRegex(t *testing.T) {
	cfg := validBase()
	cfg.Sources.CustomProxies = []CustomProxy{{
		Name: "p1", Type: "socks5", Server: "1.2.3.4", Port: 1080,
		RelayThrough: &RelayThrough{Type: "select", Strategy: "select", Match: "[invalid"},
	}}
	assertFieldError(t, Validate(&cfg), "relay_through.match")
}

func TestValidate_FiltersInvalidRegex(t *testing.T) {
	cfg := validBase()
	cfg.Filters.Exclude = "[broken"
	assertFieldError(t, Validate(&cfg), "filters.exclude")
}

func TestValidate_GroupMissingMatch(t *testing.T) {
	cfg := validBase()
	cfg.Groups = mustOrderedMap[Group](`HK: { match: "", strategy: select }`)
	assertFieldError(t, Validate(&cfg), "groups.HK.match")
}

func TestValidate_GroupInvalidRegex(t *testing.T) {
	cfg := validBase()
	cfg.Groups = mustOrderedMap[Group](`HK: { match: "[bad", strategy: select }`)
	assertFieldError(t, Validate(&cfg), "groups.HK.match")
}

func TestValidate_GroupInvalidStrategy(t *testing.T) {
	// T-CFG-005
	cfg := validBase()
	cfg.Groups = mustOrderedMap[Group](`HK: { match: "(HK)", strategy: random }`)
	assertFieldError(t, Validate(&cfg), "groups.HK.strategy")
}

func TestValidate_GroupMissingStrategy(t *testing.T) {
	cfg := validBase()
	cfg.Groups = mustOrderedMap[Group](`HK: { match: "(HK)" }`)
	assertFieldError(t, Validate(&cfg), "groups.HK.strategy")
}

func TestValidate_MissingFallback(t *testing.T) {
	cfg := validBase()
	cfg.Fallback = ""
	assertFieldError(t, Validate(&cfg), "fallback")
}

func TestValidate_MultipleErrors(t *testing.T) {
	cfg := validBase()
	cfg.Sources.Subscriptions = []Subscription{{URL: ""}}
	cfg.Fallback = ""

	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected error")
	}

	// Should contain at least 2 errors
	msg := err.Error()
	if !strings.Contains(msg, "sources.subscriptions[0].url") {
		t.Errorf("missing subscription error in: %s", msg)
	}
	if !strings.Contains(msg, "fallback") {
		t.Errorf("missing fallback error in: %s", msg)
	}
}

// --- helpers ---

// assertFieldError checks that err is non-nil and contains a ConfigError
// whose Field contains the given substring.
func assertFieldError(t *testing.T, err error, fieldSubstr string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing field %q, got nil", fieldSubstr)
	}

	// Walk joined errors to find a matching ConfigError
	for _, e := range unwrapAll(err) {
		var ce *errtype.ConfigError
		if errors.As(e, &ce) && strings.Contains(ce.Field, fieldSubstr) {
			return
		}
	}
	t.Errorf("no ConfigError with field containing %q in: %v", fieldSubstr, err)
}

// unwrapAll extracts individual errors from errors.Join results.
func unwrapAll(err error) []error {
	type joinedError interface{ Unwrap() []error }
	if je, ok := err.(joinedError); ok {
		return je.Unwrap()
	}
	return []error{err}
}

// mustOrderedMap unmarshals a YAML snippet into an OrderedMap.
func mustOrderedMap[V any](yamlStr string) OrderedMap[V] {
	var m OrderedMap[V]
	if err := yaml.Unmarshal([]byte(yamlStr), &m); err != nil {
		panic("mustOrderedMap: " + err.Error())
	}
	return m
}
