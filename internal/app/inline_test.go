package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/John-Robertt/subconverter/internal/config"
)

// T-APP-INLINE-001：已保存的内联正文在热重载和配置包导入导出后仍然保留。
func TestApp_InlineSaveReloadArchive(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(validConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	svc, err := New(ctx, Options{ConfigLocation: path, DataDir: filepath.Join(filepath.Dir(path), "data")})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := svc.ConfigSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var cfg config.Config
	if err := json.Unmarshal(snapshot.Config, &cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Sources.Subscriptions = []config.Source{{Name: "本地 Loon", Format: "loon", Content: "HK = anytls, hk.example.com, 443, \"pw\""}}
	cfg.Templates = config.Templates{Inline: map[string]string{"loon": "[General]\ndns-server = system\n"}}
	js, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveConfig(ctx, &SaveConfigInput{ConfigRevision: snapshot.ConfigRevision, Config: js}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	request, err := NewGenerateInput("loon", "", false)
	if err != nil {
		t.Fatal(err)
	}
	output, err := svc.Generate(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output.Body), `anytls, hk.example.com, 443, "pw"`) {
		t.Fatal("已保存的原生片段没有出现在生成结果中")
	}
	archive, err := svc.EffectiveConfigArchive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := readConfigArchive(archive.Body)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(entries[archiveConfigEntry]), svc.dataDir) {
		t.Fatal("配置包残留了与本机相关的托管文件引用")
	}
	if len(entries) != 3 || string(entries[archiveLoonTemplateEntry]) != cfg.Templates.Inline["loon"] || string(entries["subscriptions/1.txt"]) != cfg.Sources.Subscriptions[0].Content {
		t.Fatalf("配置包应包含订阅文件和模板文件：entries=%v", entries)
	}
	exported, err := parseConfigYAML(entries[archiveConfigEntry])
	if err != nil {
		t.Fatal(err)
	}
	if sub := exported.Sources.Subscriptions[0]; sub.Content != "" || sub.ContentFile != "subscriptions/1.txt" || sub.Name != cfg.Sources.Subscriptions[0].Name {
		t.Fatalf("配置包中的来源应保留名称并引用对应文件：%+v", sub)
	}
	imported, err := svc.ImportConfigArchive(ctx, archive.Body)
	if err != nil {
		t.Fatal(err)
	}
	var back config.Config
	if err := json.Unmarshal(imported.Config, &back); err != nil {
		t.Fatal(err)
	}
	if back.Sources.Subscriptions[0] != cfg.Sources.Subscriptions[0] || back.Templates.Inline["loon"] != cfg.Templates.Inline["loon"] {
		t.Fatal("配置包改动了内联正文")
	}
	// 替换包内模板会改变导入的草稿，但不影响其中的订阅。
	override, err := buildConfigArchive([]archiveEntry{{Name: archiveConfigEntry, Body: entries[archiveConfigEntry]}, {Name: "subscriptions/1.txt", Body: entries["subscriptions/1.txt"]}, {Name: archiveLoonTemplateEntry, Body: []byte("[General]\ndns-server = 1.1.1.1")}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	imported, err = svc.ImportConfigArchive(ctx, override)
	if err != nil {
		t.Fatal(err)
	}
	back = config.Config{}
	if err := json.Unmarshal(imported.Config, &back); err != nil {
		t.Fatal(err)
	}
	if back.Templates.Inline["loon"] != "[General]\ndns-server = 1.1.1.1" || back.Templates.Loon != "" {
		t.Fatalf("包内模板没有替换草稿中的正文：%+v", back.Templates)
	}
}

// T-APP-INLINE-002：读取会得到一份文本副本，两个文件都保持不变。
func TestApp_ReadResource(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(validConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	svc, err := New(ctx, Options{ConfigLocation: path, Fetcher: &mapFetcher{responses: map[string][]byte{"https://example.com/template": []byte("[General]\n")}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name, location, body string
		invalid              bool
	}{
		{"remote", "https://example.com/template", "[General]\n", false},
		{"local", filepath.Join(dir, "template.conf"), "[General]\ndns-server = system\n", false},
		{"empty", "", "", true},
		{"missing", filepath.Join(dir, "missing.conf"), "", true},
		{"binary", filepath.Join(dir, "binary"), "\x00\xff", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.body != "" && !strings.HasPrefix(tt.location, "https:") {
				if err := os.WriteFile(tt.location, []byte(tt.body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			result, err := svc.ReadResource(ctx, tt.location)
			if tt.invalid {
				if err == nil {
					t.Fatal("期望读取资源失败")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if result.Content != tt.body {
				t.Fatalf("实际 %q，期望 %q", result.Content, tt.body)
			}
		})
	}
	raw, err := os.ReadFile(path) // #nosec G304 -- 路径位于本测试的 t.TempDir 内。
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != validConfigYAML {
		t.Fatal("读取操作改动了配置")
	}
}

// T-APP-REVIEW-001：资源副本报告的格式遵循与 Source 相同的后缀/内容优先级。
func TestApp_ReadResourceFormatHint(t *testing.T) {
	svc, _, _ := contentStoreFixture(t)
	for _, tt := range []struct{ name, body, format string }{
		{"nodes.lcf", "[Proxy]\nHK = anytls, hk.example.com, 443, \"pw\"\n", "loon"},
		{"nodes.conf", "[Proxy]\nHK = anytls, hk.example.com, 443, password=pw\n[Remote Rule]\n", "surge"},
		{"nodes.yaml", "proxies: []\n", "clash"},
		{"nodes.txt", "anytls://pw@hk.example.com:443#HK", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			location := filepath.Join(filepath.Dir(svc.configLocation), tt.name)
			if err := os.WriteFile(location, []byte(tt.body), 0o600); err != nil {
				t.Fatal(err)
			}
			result, err := svc.ReadResource(context.Background(), location)
			if err != nil {
				t.Fatal(err)
			}
			if result.Format != tt.format || result.Content != tt.body {
				t.Fatalf("资源副本 format=%q，期望 %q，正文未变=%v", result.Format, tt.format, result.Content == tt.body)
			}
		})
	}
}
