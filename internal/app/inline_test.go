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

// T-APP-INLINE-001: saved inline bodies survive reload and archive import/export.
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
	cfg.Sources.Subscriptions = []config.Subscription{{Format: "loon", Content: "HK = anytls, hk.example.com, 443, \"pw\""}}
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
		t.Fatal("saved native fragment did not generate")
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
		t.Fatal("archive retained machine-specific managed file references")
	}
	if len(entries) != 1 {
		t.Fatalf("inline content should be stored only in config.yaml: entries=%d", len(entries))
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
		t.Fatal("archive changed inline bodies")
	}
	// An explicitly packaged template replaces the embedded body of that format.
	override, err := buildConfigArchive([]archiveEntry{{Name: archiveConfigEntry, Body: entries[archiveConfigEntry]}, {Name: archiveLoonTemplateEntry, Body: []byte("[General]\ndns-server = 1.1.1.1")}}, time.Now())
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
	if _, ok := back.Templates.Inline["loon"]; ok {
		t.Fatal("packaged template did not clear inline body")
	}
	if back.Templates.Loon == "" {
		t.Fatal("packaged template path missing")
	}
}

// T-APP-INLINE-002: reading produces a text copy and leaves both files untouched.
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
					t.Fatal("want resource read failure")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if result.Content != tt.body {
				t.Fatalf("got %q want %q", result.Content, tt.body)
			}
		})
	}
	raw, err := os.ReadFile(path) // #nosec G304 -- path is inside this test's t.TempDir.
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != validConfigYAML {
		t.Fatal("read changed config")
	}
}

// T-APP-REVIEW-001: resource copies report the same suffix/content precedence used by Source.
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
				t.Fatalf("resource copy format=%q want %q, body unchanged=%v", result.Format, tt.format, result.Content == tt.body)
			}
		})
	}
}
