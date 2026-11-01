package config

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// T-CFG-LOON-001: subscription format and Loon template survive YAML/JSON and runtime preparation.
func TestConfig_LoonRoundTrip(t *testing.T) {
	const raw = `sources:
  subscriptions:
    - {url: https://example.com/sub.conf, format: loon}
groups:
  HK: {match: HK, strategy: select}
routing:
  OUT: [HK, DIRECT]
fallback: OUT
templates:
  loon: configs/base_loon.lcf
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip Config
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatal(err)
	}
	rt, err := Prepare(&roundTrip)
	if err != nil {
		t.Fatal(err)
	}
	if rt.SourceInput().Subscriptions[0].Format != "loon" || rt.Templates().Loon != "configs/base_loon.lcf" {
		t.Fatalf("runtime lost Loon config: %+v %+v", rt.SourceInput(), rt.Templates())
	}
	back, err := yaml.Marshal(roundTrip)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(back), "format: loon") || !strings.Contains(string(back), "loon: configs/base_loon.lcf") {
		t.Fatalf("YAML lost Loon config:\n%s", back)
	}
	for _, format := range []string{"", "auto", "text", "clash", "surge", "loon", "invalid"} {
		t.Run(format, func(t *testing.T) {
			cfg.Sources.Subscriptions[0].Format = format
			_, err := Prepare(&cfg)
			if (err != nil) != (format == "invalid") {
				t.Fatalf("format=%q error=%v", format, err)
			}
			if format == "invalid" && !strings.Contains(err.Error(), "subscriptions[0].format") {
				t.Fatalf("error lacks field path: %v", err)
			}
		})
	}
}
