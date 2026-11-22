package config

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// T-CFG-LOON-001：订阅格式与 Loon 模板经过 YAML/JSON 往返和运行时预处理后保持不变。
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
		t.Fatalf("运行时配置丢失了 Loon 设置：%+v %+v", rt.SourceInput(), rt.Templates())
	}
	back, err := yaml.Marshal(roundTrip)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(back), "format: loon") || !strings.Contains(string(back), "loon: configs/base_loon.lcf") {
		t.Fatalf("YAML 丢失了 Loon 设置：\n%s", back)
	}
	for _, format := range []string{"", "auto", "text", "clash", "surge", "loon", "invalid"} {
		t.Run(format, func(t *testing.T) {
			cfg.Sources.Subscriptions[0].Format = format
			_, err := Prepare(&cfg)
			if (err != nil) != (format == "invalid") {
				t.Fatalf("format=%q，错误=%v", format, err)
			}
			if format == "invalid" && !strings.Contains(err.Error(), "subscriptions[0].format") {
				t.Fatalf("错误信息缺少字段路径：%v", err)
			}
		})
	}
}
