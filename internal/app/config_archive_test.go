package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/John-Robertt/subconverter/internal/config"
	"gopkg.in/yaml.v3"
)

// T-APP-ARCHIVE-001: both exports package exact local bytes, preserve names and leave remote sources unchanged.
func TestConfigArchive_SubscriptionsRoundTrip(t *testing.T) {
	ctx := context.Background()
	svc, cfg, _ := contentStoreFixture(t)
	dir := filepath.Dir(svc.configLocation)
	const surgeBody = "[Proxy]\nHK = ss, hk.example.com, 443, encrypt-method=aes-128-gcm, password=pw\n"
	const loonBody = "[Proxy]\nSG = anytls, sg.example.com, 443, \"pw\"\n"
	for name, body := range map[string]string{"nodes.conf": surgeBody, "nodes.lcf": loonBody} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cfg.Templates = config.Templates{}
	cfg.Sources.Subscriptions = []config.Source{
		{Name: "远程", URL: "https://example.com/sub", Format: "auto"},
		{Name: "同名/../订阅", ContentFile: filepath.Join(dir, "nodes.conf"), Format: "auto"},
		{Name: "同名/../订阅", ContentFile: filepath.Join(dir, "nodes.lcf")},
	}
	raw, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(svc.configLocation, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"saved", "effective"} {
		t.Run(mode, func(t *testing.T) {
			export := svc.SavedConfigArchive
			if mode == "effective" {
				export = svc.EffectiveConfigArchive
			}
			archive, err := export(ctx)
			if err != nil {
				t.Fatal(err)
			}
			entries, err := readConfigArchive(archive.Body)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 3 || string(entries["subscriptions/2.txt"]) != surgeBody || string(entries["subscriptions/3.txt"]) != loonBody {
				t.Fatalf("archive did not preserve separate subscription files: entries=%v", entries)
			}
			if strings.Contains(string(entries[archiveConfigEntry]), dir) || strings.Contains(string(entries[archiveConfigEntry]), "password=pw") {
				t.Fatal("archive config retained local paths or embedded subscription bytes")
			}
			exported, err := parseConfigYAML(entries[archiveConfigEntry])
			if err != nil {
				t.Fatal(err)
			}
			if exported.Sources.Subscriptions[0] != cfg.Sources.Subscriptions[0] {
				t.Fatalf("remote subscription changed: got=%+v want=%+v", exported.Sources.Subscriptions[0], cfg.Sources.Subscriptions[0])
			}
			for i, want := range []config.Source{
				{Name: "同名/../订阅", ContentFile: "subscriptions/2.txt", Format: "surge"},
				{Name: "同名/../订阅", ContentFile: "subscriptions/3.txt", Format: "loon"},
			} {
				if got := exported.Sources.Subscriptions[i+1]; got != want {
					t.Fatalf("source %d got=%+v want=%+v", i+1, got, want)
				}
			}
			before, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			imported, err := svc.ImportConfigArchive(ctx, archive.Body)
			if err != nil {
				t.Fatal(err)
			}
			back, err := parseConfigJSON(imported.Config)
			if err != nil {
				t.Fatal(err)
			}
			for i, body := range []string{surgeBody, loonBody} {
				got := back.Sources.Subscriptions[i+1]
				if got.Content != body || got.ContentFile != "" || got.Name != cfg.Sources.Subscriptions[i+1].Name {
					t.Fatalf("imported source %d lost name or content: %+v", i+1, got)
				}
			}
			after, err := os.ReadDir(dir)
			if err != nil || len(after) != len(before) {
				t.Fatalf("import wrote files: before=%d after=%d err=%v", len(before), len(after), err)
			}
		})
	}
}

// T-APP-ARCHIVE-002: imported sources must be complete package files within the shared size limit.
func TestConfigArchive_InvalidSourceFiles(t *testing.T) {
	svc, cfg, _ := contentStoreFixture(t)
	cfg.Templates = config.Templates{}
	for _, kind := range []string{"subscriptions", "snell", "vless"} {
		for _, tt := range []struct {
			name    string
			source  config.Source
			entries []archiveEntry
			message string
		}{
			{"other source kind", config.Source{ContentFile: "other-kind"}, nil, "必须引用包内"},
			{"missing", config.Source{ContentFile: "subscriptions/1.txt"}, nil, "缺少来源文件"},
			{"absolute", config.Source{ContentFile: filepath.Join(filepath.Dir(svc.configLocation), "private.txt")}, nil, "必须引用包内"},
			{"traversal", config.Source{ContentFile: "subscriptions/../private.txt"}, nil, "必须引用包内"},
			{"remote file", config.Source{ContentFile: "https://example.com/private"}, nil, "必须引用包内"},
			{"inline", config.Source{Content: "anytls://pw@hk.example.com:443#HK"}, nil, "必须通过 content_file"},
			{"empty", config.Source{ContentFile: "subscriptions/1.txt"}, []archiveEntry{{Name: "subscriptions/1.txt", Body: []byte(" \n")}}, "正文不能为空"},
			{"invalid UTF-8", config.Source{ContentFile: "subscriptions/1.txt"}, []archiveEntry{{Name: "subscriptions/1.txt", Body: []byte{0xff}}}, "必须为 UTF-8 文本"},
			{"NUL", config.Source{ContentFile: "subscriptions/1.txt"}, []archiveEntry{{Name: "subscriptions/1.txt", Body: []byte("a\x00b")}}, "必须为 UTF-8 文本"},
			{"duplicate", config.Source{ContentFile: "subscriptions/1.txt"}, []archiveEntry{{Name: "subscriptions/1.txt", Body: []byte("a")}, {Name: "subscriptions/1.txt", Body: []byte("b")}}, "重复文件"},
			{"size limit", config.Source{ContentFile: "subscriptions/1.txt"}, []archiveEntry{{Name: "subscriptions/1.txt", Body: []byte(strings.Repeat("x", MaxConfigImportArchiveBytes))}}, "解压后不能超过"},
		} {
			t.Run(kind+"/"+tt.name, func(t *testing.T) {
				source := tt.source
				source.ContentFile = strings.Replace(source.ContentFile, "subscriptions/", kind+"/", 1)
				if tt.name == "other source kind" {
					source.ContentFile = "subscriptions/1.txt"
					if kind == "subscriptions" {
						source.ContentFile = "snell/1.txt"
					}
				}
				cfg.Sources = config.Sources{}
				switch kind {
				case "subscriptions":
					cfg.Sources.Subscriptions = []config.Source{source}
				case "snell":
					cfg.Sources.Snell = []config.Source{source}
				case "vless":
					cfg.Sources.VLess = []config.Source{source}
				}
				entries := []archiveEntry{}
				for _, entry := range tt.entries {
					entries = append(entries, archiveEntry{Name: strings.Replace(entry.Name, "subscriptions/", kind+"/", 1), Body: entry.Body})
				}
				raw, err := yaml.Marshal(cfg)
				if err != nil {
					t.Fatal(err)
				}
				archive, err := buildConfigArchive(append([]archiveEntry{{Name: archiveConfigEntry, Body: raw}}, entries...), time.Now())
				if err != nil {
					t.Fatal(err)
				}
				_, err = svc.ImportConfigArchive(context.Background(), archive)
				var badReq *BadRequestError
				if !errors.As(err, &badReq) || badReq.Code != "invalid_archive" || !strings.Contains(badReq.Message, tt.message) {
					t.Fatalf("got=%v want invalid_archive containing %q", err, tt.message)
				}
			})
		}
	}
}

// T-APP-ARCHIVE-003: an unpacked config generates all three formats from its relative file references.
func TestConfigArchive_UnpackedConfig(t *testing.T) {
	ctx := context.Background()
	svc, cfg, revision := contentStoreFixture(t)
	if _, err := saveContentDraft(t, svc, cfg, revision); err != nil {
		t.Fatal(err)
	}
	archive, err := svc.SavedConfigArchive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := readConfigArchive(archive.Body)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, body := range entries {
		location := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(location), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(location, body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
	unpacked, err := New(ctx, Options{ConfigLocation: archiveConfigEntry, DataDir: "data"})
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"clash", "surge", "loon"} {
		t.Run(format, func(t *testing.T) {
			request, err := NewGenerateInput(format, "", false)
			if err != nil {
				t.Fatal(err)
			}
			output, err := unpacked.Generate(ctx, request)
			if err != nil {
				t.Fatalf("unpacked %s generation failed: %v", format, err)
			}
			if !strings.Contains(string(output.Body), "HK-SAVED") {
				t.Fatalf("unpacked %s generation lost the saved node", format)
			}
		})
	}
}
