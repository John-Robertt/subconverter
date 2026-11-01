package config

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// T-CFG-INLINE-001: inline bodies survive both wire formats and snapshot isolation.
func TestConfig_InlineRoundTrip(t *testing.T) {
	cfg := validBase()
	cfg.Sources.Subscriptions = []Subscription{{Content: "[Proxy]\nHK = anytls, hk.example.com, 443, \"pw\"\n", Format: "loon"}}
	cfg.Templates.Inline = map[string]string{"loon": "[General]\ndns-server = system\n"}
	raw, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Config
	if err := yaml.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	js, err := json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(js, &decoded); err != nil {
		t.Fatal(err)
	}
	rt, err := Prepare(&decoded)
	if err != nil {
		t.Fatal(err)
	}
	decoded.Templates.Inline["loon"] = "changed"
	copy := rt.Templates()
	copy.Inline["loon"] = "also changed"
	if rt.Templates().Inline["loon"] != cfg.Templates.Inline["loon"] || rt.SourceInput().Subscriptions[0] != cfg.Sources.Subscriptions[0] {
		t.Fatal("inline content changed during round trip or escaped snapshot")
	}
}

// T-CFG-INLINE-002: each source and template has exactly one authoritative body.
func TestPrepare_InlineValidation(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
		field  string
	}{
		{"file conflict", func(c *Config) { c.Sources.Subscriptions[0].ContentFile = "/config/subscription.txt" }, "subscriptions[0].url"},
		{"file URL", func(c *Config) { c.Sources.Subscriptions = []Subscription{{ContentFile: "https://example.com/sub"}} }, "subscriptions[0].content_file"},
		{"source conflict", func(c *Config) { c.Sources.Subscriptions[0].Content = "ss://value" }, "subscriptions[0].url"},
		{"source blank", func(c *Config) { c.Sources.Subscriptions = []Subscription{{Content: " \n"}} }, "subscriptions[0].content"},
		{"template conflict", func(c *Config) {
			c.Templates = Templates{Loon: "base.conf", Inline: map[string]string{"loon": "[General]"}}
		}, "templates.inline.loon"},
		{"template blank", func(c *Config) { c.Templates.Inline = map[string]string{"loon": " \n"} }, "templates.inline.loon"},
		{"template format", func(c *Config) { c.Templates.Inline = map[string]string{"unknown": "body"} }, "templates.inline.unknown"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validBase()
			tt.mutate(&cfg)
			_, err := Prepare(&cfg)
			if err == nil || !strings.Contains(err.Error(), tt.field) {
				t.Fatalf("want error at %s, got %v", tt.field, err)
			}
		})
	}
}
