package config

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// T-CFG-INLINE-001：内联正文经过两种传输格式后保持不变，且快照相互隔离。
func TestConfig_InlineRoundTrip(t *testing.T) {
	cfg := validBase()
	cfg.Sources.Subscriptions = []Source{{Name: "手动 Loon 订阅", Content: "[Proxy]\nHK = anytls, hk.example.com, 443, \"pw\"\n", Format: "loon"}}
	cfg.Sources.Snell = []Source{{Name: "本地 Snell", Content: "HK = snell, hk.example.com, 443, psk=pw, version=4"}}
	cfg.Sources.VLess = []Source{{Name: "本地 VLESS", ContentFile: "nodes/vless.txt"}}
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
		t.Fatal("内联内容在往返中被改变，或对副本的修改影响到了快照")
	}
	if rt.SourceInput().Snell[0] != cfg.Sources.Snell[0] || rt.SourceInput().VLess[0] != cfg.Sources.VLess[0] {
		t.Fatalf("节点池在 YAML/JSON 往返中丢失了字段：%+v", rt.SourceInput())
	}
}

// T-CFG-LOCAL-001：每个来源只能选择一种输入；节点池的解析器是固定的。
func TestPrepare_LocalNodePoolInputs(t *testing.T) {
	for _, kind := range []string{"snell", "vless"} {
		for _, tt := range []struct {
			name   string
			source Source
			field  string
		}{
			{"body", Source{Name: "手动节点池", Content: "node body"}, ""},
			{"file", Source{ContentFile: "nodes.txt"}, ""},
			{"URL", Source{URL: "https://example.com/nodes"}, ""},
			{"missing", Source{}, ".url"},
			{"conflict", Source{URL: "https://example.com/nodes", Content: "body"}, ".url"},
			{"whitespace", Source{Content: " \n"}, ".content"},
			{"remote file", Source{ContentFile: "https://example.com/nodes"}, ".content_file"},
			{"format", Source{Content: "body", Format: "surge"}, ".format"},
		} {
			t.Run(kind+"/"+tt.name, func(t *testing.T) {
				cfg := validBase()
				if kind == "snell" {
					cfg.Sources.Snell = []Source{tt.source}
				} else {
					cfg.Sources.VLess = []Source{tt.source}
				}
				_, err := Prepare(&cfg)
				if tt.field == "" {
					if err != nil {
						t.Fatalf("合法来源被拒绝：%v", err)
					}
				} else if err == nil || !strings.Contains(err.Error(), "sources."+kind+"[0]"+tt.field) {
					t.Fatalf("实际=%v，期望字段 sources.%s[0]%s", err, kind, tt.field)
				}
			})
		}
	}
}

// T-CFG-INLINE-002：每个来源和模板都只有一份权威正文。
func TestPrepare_InlineValidation(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
		field  string
	}{
		{"file conflict", func(c *Config) { c.Sources.Subscriptions[0].ContentFile = "/config/subscription.txt" }, "subscriptions[0].url"},
		{"file URL", func(c *Config) { c.Sources.Subscriptions = []Source{{ContentFile: "https://example.com/sub"}} }, "subscriptions[0].content_file"},
		{"source conflict", func(c *Config) { c.Sources.Subscriptions[0].Content = "ss://value" }, "subscriptions[0].url"},
		{"source blank", func(c *Config) { c.Sources.Subscriptions = []Source{{Content: " \n"}} }, "subscriptions[0].content"},
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
				t.Fatalf("期望在 %s 处报错，实际为 %v", tt.field, err)
			}
		})
	}
}
