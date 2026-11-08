package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/errtype"
	"gopkg.in/yaml.v3"
)

func contentStoreFixture(t *testing.T) (*Service, *config.Config, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(validConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	svc, err := New(context.Background(), Options{ConfigLocation: path, DataDir: filepath.Join(dir, "resources"), Now: func() time.Time { return time.Date(2026, 11, 1, 12, 34, 56, 123456789, time.UTC) }})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := svc.ConfigSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var cfg config.Config
	if err := json.Unmarshal(snapshot.Config, &cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Sources.Subscriptions = []config.Source{{Name: "手动订阅", Content: "anytls://secret-value@hk.example.com:443#HK-SAVED"}}
	cfg.Templates.Inline = map[string]string{"clash": "mixed-port: 17890\n", "surge": "[General]\ndns-server = system\n", "loon": "[General]\ndns-server = system\n"}
	return svc, &cfg, snapshot.ConfigRevision
}

func saveContentDraft(t *testing.T, svc *Service, cfg *config.Config, revision string) (*SaveConfigResult, error) {
	t.Helper()
	js, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return svc.SaveConfig(context.Background(), &SaveConfigInput{ConfigRevision: revision, Config: js})
}

// T-APP-STORE-001: bodies are stored directly in the configured directory and remain editable after restart.
func TestContentStore_SaveAndRestart(t *testing.T) {
	svc, cfg, revision := contentStoreFixture(t)
	result, err := saveContentDraft(t, svc, cfg, revision)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(svc.configLocation) // #nosec G304 -- fixture location is inside t.TempDir.
	if err != nil {
		t.Fatal(err)
	}
	var stored config.Config
	if err := yaml.Unmarshal(raw, &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Sources.Subscriptions[0].Content != "" || len(stored.Templates.Inline) != 0 || strings.Contains(string(raw), "secret-value") {
		t.Fatal("config contains bodies instead of references")
	}
	if stored.Sources.Subscriptions[0].Name != cfg.Sources.Subscriptions[0].Name {
		t.Fatalf("saved name=%q want %q", stored.Sources.Subscriptions[0].Name, cfg.Sources.Subscriptions[0].Name)
	}
	paths := []string{stored.Sources.Subscriptions[0].ContentFile, stored.Templates.Clash, stored.Templates.Surge, stored.Templates.Loon}
	for _, path := range paths {
		if !strings.Contains(filepath.Base(path), "20261101T123456.123456789Z-") {
			t.Fatalf("filename lacks timestamp: %q", path)
		}
		if filepath.Dir(path) != svc.dataDir {
			t.Fatalf("body stored outside fixed directory: %q", path)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("body permissions=%o want 600", info.Mode().Perm())
		}
	}
	entries, err := os.ReadDir(svc.dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("want 4 body files and no staging files, got %d", len(entries))
	}
	restarted, err := New(context.Background(), Options{ConfigLocation: svc.configLocation, DataDir: svc.dataDir})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := restarted.ConfigSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var editable config.Config
	if err := json.Unmarshal(snapshot.Config, &editable); err != nil {
		t.Fatal(err)
	}
	if editable.Sources.Subscriptions[0] != cfg.Sources.Subscriptions[0] || editable.Templates.Inline["loon"] != cfg.Templates.Inline["loon"] || editable.Templates.Loon != "" {
		t.Fatal("restart lost editable bodies")
	}
	if snapshot.ConfigRevision != result.ConfigRevision {
		t.Fatal("snapshot revision does not represent disk config")
	}
	for _, format := range []string{"clash", "surge", "loon"} {
		request, err := NewGenerateInput(format, "", false)
		if err != nil {
			t.Fatal(err)
		}
		output, err := restarted.Generate(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(output.Body), "HK-SAVED") {
			t.Fatalf("%s lost saved node", format)
		}
	}
	if _, err := saveContentDraft(t, svc, cfg, result.ConfigRevision); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadDir(svc.dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 4 {
		t.Fatalf("unchanged save created extra versions: files=%d want 4", len(after))
	}
}

// T-APP-STORE-002: directory failures preserve the saved config and can be retried after fixing permissions.
func TestContentStore_DirectoryFailureAndRetry(t *testing.T) {
	for _, mode := range []string{"readonly", "file", "missing-parent-permission"} {
		t.Run(mode, func(t *testing.T) {
			svc, cfg, revision := contentStoreFixture(t)
			switch mode {
			case "file":
				if err := os.WriteFile(svc.dataDir, []byte("occupied"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "readonly":
				if err := os.MkdirAll(svc.dataDir, 0o500); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					_ = os.Chmod(svc.dataDir, 0o700) // #nosec G302 -- Owner execute permission is required for temporary directories.
				})
			default:
				if err := os.MkdirAll(svc.dataDir, 0o500); err != nil {
					t.Fatal(err)
				}
				parent := svc.dataDir
				svc.dataDir = filepath.Join(parent, "nested")
				t.Cleanup(func() {
					_ = os.Chmod(parent, 0o700) // #nosec G302 -- Owner execute permission is required for temporary directories.
				})
			}
			_, err := saveContentDraft(t, svc, cfg, revision)
			var writeErr *errtype.ResourceError
			if !errors.As(err, &writeErr) || writeErr.Code != errtype.CodeResourceContentWriteFailed || !strings.Contains(writeErr.Message, svc.dataDir) || strings.Contains(writeErr.Message, "secret-value") {
				t.Fatalf("want actionable sanitized write error, got %v", err)
			}
			snapshot, err := svc.ConfigSnapshot(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			status, err := svc.Status(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if snapshot.ConfigRevision != revision || status.ConfigDirty || !status.Capabilities.ConfigWrite || !status.ConfigSource.Writable {
				t.Fatal("failed body save changed config or readonly contract")
			}
			switch mode {
			case "file":
				if err := os.Remove(svc.dataDir); err != nil {
					t.Fatal(err)
				}
			case "readonly":
				if err := os.Chmod(svc.dataDir, 0o700); err != nil { // #nosec G302 -- Restores owner access to a temporary directory.
					t.Fatal(err)
				}
			default:
				if err := os.Chmod(filepath.Dir(svc.dataDir), 0o700); err != nil { // #nosec G302 -- Restores owner access to a temporary directory.
					t.Fatal(err)
				}
			}
			if _, err := saveContentDraft(t, svc, cfg, revision); err != nil {
				t.Fatalf("retry failed: %v", err)
			}
		})
	}
}

// T-APP-STORE-003: changing a body cannot change an existing runtime snapshot before reload.
func TestContentStore_SnapshotAndConcurrentSave(t *testing.T) {
	svc, cfg, revision := contentStoreFixture(t)
	saved, err := saveContentDraft(t, svc, cfg, revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	active, err := svc.runtimeSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	old := active.SourceInput().Subscriptions[0].ContentFile
	cfg.Sources.Subscriptions[0].Content = "anytls://new-secret@hk.example.com:443#HK-NEW"
	js, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 4)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.SaveConfig(context.Background(), &SaveConfigInput{ConfigRevision: saved.ConfigRevision, Config: js})
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	success, conflicts := 0, 0
	for err := range results {
		var conflict *errtype.RevisionConflictError
		if err == nil {
			success++
		} else if errors.As(err, &conflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflicts != 3 {
		t.Fatalf("success=%d conflicts=%d want 1/3", success, conflicts)
	}
	body, err := os.ReadFile(old) // #nosec G304 -- path is generated within the test's managed directory.
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "HK-SAVED") {
		t.Fatal("old snapshot body was overwritten")
	}
	request, err := NewGenerateInput("loon", "", false)
	if err != nil {
		t.Fatal(err)
	}
	output, err := svc.Generate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output.Body), "HK-SAVED") {
		t.Fatal("save mutated active runtime before reload")
	}
	if _, err := svc.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	output, err = svc.Generate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output.Body), "HK-NEW") {
		t.Fatal("reload did not activate new body")
	}
}

// T-APP-STORE-004: editing a readonly source writes only a new version in the managed directory.
func TestContentStore_ReadonlyOriginal(t *testing.T) {
	svc, cfg, revision := contentStoreFixture(t)
	original := filepath.Join(filepath.Dir(svc.configLocation), "original.conf")
	const body = "[General]\ndns-server = system\n"
	if err := os.WriteFile(original, []byte(body), 0o400); err != nil {
		t.Fatal(err)
	}
	resource, err := svc.ReadResource(context.Background(), original)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Templates.Inline["loon"] = resource.Content + "loglevel = notify\n"
	if _, err := saveContentDraft(t, svc, cfg, revision); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(original) // #nosec G304 -- original is inside t.TempDir.
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != body {
		t.Fatal("readonly original was overwritten")
	}
}

// T-APP-STORE-005: the production default has no extra content subdirectory.
func TestContentStore_DefaultDirectory(t *testing.T) {
	if got := (&Service{}).contentDirectory(); got != "/config" {
		t.Fatalf("default directory=%q want /config", got)
	}
}

// T-APP-STORE-009: copying a suffix-detected document retains its format after saving and restarting.
func TestContentStore_SourceFormatSurvivesCopy(t *testing.T) {
	for _, origin := range []string{"ordinary file", "managed file"} {
		t.Run(origin, func(t *testing.T) {
			svc, cfg, _ := contentStoreFixture(t)
			location := filepath.Join(filepath.Dir(svc.configLocation), "nodes.lcf")
			if origin == "managed file" {
				if err := os.MkdirAll(svc.dataDir, 0o700); err != nil {
					t.Fatal(err)
				}
				location = filepath.Join(svc.dataDir, "subscription-20261101T123456.123456789Z-12345.lcf")
			}
			const body = "[Proxy]\nHK = anytls, hk.example.com, 443, \"pw\"\n"
			if err := os.WriteFile(location, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg.Sources.Subscriptions = []config.Source{{ContentFile: location}}
			raw, err := yaml.Marshal(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(svc.configLocation, raw, 0o600); err != nil {
				t.Fatal(err)
			}
			snapshot, err := svc.ConfigSnapshot(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			var editable config.Config
			if err := json.Unmarshal(snapshot.Config, &editable); err != nil {
				t.Fatal(err)
			}
			if editable.Sources.Subscriptions[0].Format != "loon" {
				t.Fatalf("copy lost .lcf format: %+v", editable.Sources.Subscriptions[0])
			}
			if _, err := svc.SaveConfig(context.Background(), &SaveConfigInput{ConfigRevision: snapshot.ConfigRevision, Config: snapshot.Config}); err != nil {
				t.Fatal(err)
			}
			savedRaw, err := os.ReadFile(svc.configLocation) // #nosec G304 -- fixture config is inside t.TempDir.
			if err != nil {
				t.Fatal(err)
			}
			var stored config.Config
			if err := yaml.Unmarshal(savedRaw, &stored); err != nil {
				t.Fatal(err)
			}
			if path := stored.Sources.Subscriptions[0].ContentFile; filepath.Ext(path) != ".txt" {
				t.Fatalf("Loon source filename=%q want .txt", path)
			}
			if filepath.Ext(stored.Templates.Loon) != ".lcf" {
				t.Fatalf("Loon template filename=%q want .lcf", stored.Templates.Loon)
			}
			if stored.Sources.Subscriptions[0].Format != "loon" {
				t.Fatalf("saved source lost explicit format: %+v", stored.Sources.Subscriptions[0])
			}
			original, err := os.ReadFile(location) // #nosec G304 -- fixture source is inside t.TempDir.
			if err != nil || string(original) != body {
				t.Fatalf("saving modified original source: body=%q err=%v", original, err)
			}
			savedBody, err := os.ReadFile(stored.Sources.Subscriptions[0].ContentFile) // #nosec G304 -- service-managed body is inside t.TempDir.
			if err != nil || string(savedBody) != body {
				t.Fatalf("saved Loon body = %q err=%v, want %q", savedBody, err, body)
			}
			restarted, err := New(context.Background(), Options{ConfigLocation: svc.configLocation, DataDir: svc.dataDir})
			if err != nil {
				t.Fatal(err)
			}
			req, err := NewGenerateInput("loon", "", false)
			if err != nil {
				t.Fatal(err)
			}
			output, err := restarted.Generate(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(output.Body), `anytls, hk.example.com, 443, "pw"`) {
				t.Fatal("Loon node disappeared after copy/save/restart")
			}
			edited, revision := editableContentConfig(t, restarted)
			if _, err := saveContentDraft(t, restarted, &edited, revision); err != nil {
				t.Fatal(err)
			}
			after := storedContentConfig(t, restarted)
			if after.Sources.Subscriptions[0] != stored.Sources.Subscriptions[0] || after.Templates.Loon != stored.Templates.Loon {
				t.Fatalf("unchanged save replaced source or template: before=%+v after=%+v", stored, after)
			}
		})
	}
}

// T-APP-STORE-008: one missing body does not prevent repairing the rest of the config.
func TestContentStore_MissingBodyRemainsEditable(t *testing.T) {
	svc, cfg, revision := contentStoreFixture(t)
	if _, err := saveContentDraft(t, svc, cfg, revision); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(svc.configLocation) // #nosec G304 -- fixture config is inside t.TempDir.
	if err != nil {
		t.Fatal(err)
	}
	var stored config.Config
	if err := yaml.Unmarshal(raw, &stored); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(stored.Sources.Subscriptions[0].ContentFile); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(stored.Templates.Loon); err != nil {
		t.Fatal(err)
	}
	snapshot, err := svc.ConfigSnapshot(context.Background())
	if err != nil {
		t.Fatalf("missing body blocked config editor: %v", err)
	}
	var got config.Config
	if err := json.Unmarshal(snapshot.Config, &got); err != nil {
		t.Fatal(err)
	}
	if got.Sources.Subscriptions[0].ContentFile != stored.Sources.Subscriptions[0].ContentFile || got.Templates.Loon != stored.Templates.Loon {
		t.Fatal("unreadable body references were lost")
	}
	if got.Templates.Inline["clash"] != cfg.Templates.Inline["clash"] {
		t.Fatal("a failed body prevented restoring other readable bodies")
	}
	if _, err := svc.ImportConfigYAML(context.Background(), raw); err != nil {
		t.Fatalf("missing local body blocked draft import: %v", err)
	}
	if _, err := svc.EffectiveConfigArchive(context.Background()); err == nil {
		t.Fatal("archive exported incomplete bodies")
	}
}

// T-APP-STORE-007: ordinary template filenames never turn a live reference into a snapshot.
func TestContentStore_ManagedTemplateNames(t *testing.T) {
	for _, tt := range []struct {
		name    string
		managed bool
	}{
		{"loon.lcf", false},
		{"template-loon-base.lcf", false},
		{"template-loon-20261101T123456.123456789Z-12345.lcf", true},
		{"template-loon-99999999T123456.123456789Z-12345.lcf", false},
		{"template-loon-20261101T123456.123456789Z-12345.yaml", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, cfg, _ := contentStoreFixture(t)
			if err := os.MkdirAll(svc.dataDir, 0o700); err != nil {
				t.Fatal(err)
			}
			location := filepath.Join(svc.dataDir, tt.name)
			const body = "[General]\ndns-server = system\n"
			if err := os.WriteFile(location, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg.Templates = config.Templates{Loon: location}
			raw, err := yaml.Marshal(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(svc.configLocation, raw, 0o600); err != nil {
				t.Fatal(err)
			}
			snapshot, err := svc.ConfigSnapshot(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			var got config.Config
			if err := json.Unmarshal(snapshot.Config, &got); err != nil {
				t.Fatal(err)
			}
			if tt.managed {
				if got.Templates.Loon != "" || got.Templates.Inline["loon"] != body {
					t.Fatal("managed body was not restored")
				}
			} else if got.Templates.Loon != location || len(got.Templates.Inline) != 0 {
				t.Fatalf("ordinary template reference changed: %+v", got.Templates)
			}
		})
	}
}

// T-APP-STORE-006: a config commit failure removes only the versions created by that save.
func TestContentStore_ConfigCommitRollback(t *testing.T) {
	svc, cfg, revision := contentStoreFixture(t)
	original := svc.configLocation
	blocked := filepath.Join(filepath.Dir(original), "blocked-config")
	if err := os.Mkdir(blocked, 0o700); err != nil {
		t.Fatal(err)
	}
	now := svc.now
	svc.now = func() time.Time { svc.configLocation = blocked; return now() }
	_, err := saveContentDraft(t, svc, cfg, revision)
	if !errors.Is(err, errtype.ErrConfigFileNotWritable) {
		t.Fatalf("want config commit failure, got %v", err)
	}
	svc.configLocation = original
	svc.now = now
	entries, err := os.ReadDir(svc.dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("uncommitted versions remain: %d", len(entries))
	}
	snapshot, err := svc.ConfigSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ConfigRevision != revision {
		t.Fatal("failed config commit changed revision")
	}
}

// T-APP-STORE-010: SaveConfig also rolls back files when a later body cannot be written.
func TestContentStore_PartialBodyRollback(t *testing.T) {
	svc, cfg, revision := contentStoreFixture(t)
	root := svc.dataDir
	blocked := filepath.Join(filepath.Dir(svc.configLocation), "blocked-directory")
	if err := os.WriteFile(blocked, []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}
	clock := svc.now
	writes := 0
	svc.now = func() time.Time {
		writes++
		if writes == 2 {
			svc.dataDir = blocked
		}
		return clock()
	}
	_, err := saveContentDraft(t, svc, cfg, revision)
	var writeErr *errtype.ResourceError
	if !errors.As(err, &writeErr) || writeErr.Code != errtype.CodeResourceContentWriteFailed {
		t.Fatalf("want later-body write failure, got %v", err)
	}
	svc.dataDir, svc.now = root, clock
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("partial save left %d uncommitted files", len(entries))
	}
	snapshot, err := svc.ConfigSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ConfigRevision != revision {
		t.Fatal("partial body save changed config")
	}
}

func storedContentConfig(t *testing.T, svc *Service) config.Config {
	t.Helper()
	raw, err := os.ReadFile(svc.configLocation) // #nosec G304 -- Fixture config is under t.TempDir.
	if err != nil {
		t.Fatal(err)
	}
	var cfg config.Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func editableContentConfig(t *testing.T, svc *Service) (config.Config, string) {
	t.Helper()
	snapshot, err := svc.ConfigSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var cfg config.Config
	if err := json.Unmarshal(snapshot.Config, &cfg); err != nil {
		t.Fatal(err)
	}
	return cfg, snapshot.ConfigRevision
}

// T-APP-STORE-011: only changed bodies create versions; metadata and identical saves reuse references.
func TestContentStore_ReuseUnchangedBodies(t *testing.T) {
	for _, tt := range []struct {
		name                             string
		patch                            func(*config.Config)
		changed                          string
		addLoon, missingSource, readonly bool
	}{
		{name: "unchanged"},
		{name: "metadata only", patch: func(c *config.Config) { c.BaseURL = "https://example.com" }},
		{name: "readonly content directory", readonly: true, patch: func(c *config.Config) { c.BaseURL = "https://example.com" }},
		{name: "add Loon only", addLoon: true, changed: "loon", patch: func(c *config.Config) { c.Templates.Inline["loon"] = "[General]\ndns-server = system\n" }},
		{name: "subscription", changed: "subscription", patch: func(c *config.Config) {
			c.Sources.Subscriptions[0].Content = "anytls://changed@hk.example.com:443#HK-CHANGED"
		}},
		{name: "clash", changed: "clash", patch: func(c *config.Config) { c.Templates.Inline["clash"] = "mixed-port: 7891\n" }},
		{name: "surge", changed: "surge", patch: func(c *config.Config) { c.Templates.Inline["surge"] = "[General]\ndns-server = 1.1.1.1\n" }},
		{name: "loon", changed: "loon", patch: func(c *config.Config) { c.Templates.Inline["loon"] = "[General]\ndns-server = 1.1.1.1\n" }},
		{name: "repair missing source", changed: "subscription", missingSource: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, cfg, revision := contentStoreFixture(t)
			if tt.addLoon {
				delete(cfg.Templates.Inline, "loon")
			}
			if _, err := saveContentDraft(t, svc, cfg, revision); err != nil {
				t.Fatal(err)
			}
			before := storedContentConfig(t, svc)
			edited, savedRevision := editableContentConfig(t, svc)
			if tt.patch != nil {
				tt.patch(&edited)
			}
			if tt.missingSource {
				if err := os.Remove(before.Sources.Subscriptions[0].ContentFile); err != nil {
					t.Fatal(err)
				}
			}
			if tt.readonly {
				if err := os.Chmod(svc.dataDir, 0o500); err != nil { // #nosec G302 -- Directory read/execute access is required to reuse existing versions without write permission.
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(svc.dataDir, 0o700) }) // #nosec G302 -- Restores owner access to a temporary directory.
			}
			info, err := os.Stat(svc.configLocation)
			if err != nil {
				t.Fatal(err)
			}
			result, err := saveContentDraft(t, svc, &edited, savedRevision)
			if err != nil {
				t.Fatal(err)
			}
			after := storedContentConfig(t, svc)
			oldPaths := map[string]string{"subscription": before.Sources.Subscriptions[0].ContentFile, "clash": before.Templates.Clash, "surge": before.Templates.Surge, "loon": before.Templates.Loon}
			newPaths := map[string]string{"subscription": after.Sources.Subscriptions[0].ContentFile, "clash": after.Templates.Clash, "surge": after.Templates.Surge, "loon": after.Templates.Loon}
			for kind, old := range oldPaths {
				changed := newPaths[kind] != old
				if changed != (kind == tt.changed) {
					t.Fatalf("%s reference changed=%v want %v: old=%q new=%q", kind, changed, kind == tt.changed, old, newPaths[kind])
				}
			}
			entries, err := os.ReadDir(svc.dataDir)
			if err != nil {
				t.Fatal(err)
			}
			wantFiles := 4
			if tt.addLoon {
				wantFiles--
			}
			if tt.changed != "" {
				wantFiles++
			}
			if tt.missingSource {
				wantFiles--
			}
			if len(entries) != wantFiles {
				t.Fatalf("body files=%d want %d", len(entries), wantFiles)
			}
			if tt.patch == nil && !tt.missingSource {
				afterInfo, err := os.Stat(svc.configLocation)
				if err != nil {
					t.Fatal(err)
				}
				if result.ConfigRevision != savedRevision || !os.SameFile(info, afterInfo) {
					t.Fatalf("identical save rewrote config: revision=%s want %s", result.ConfigRevision, savedRevision)
				}
			}
		})
	}
}

// T-APP-STORE-012: subscription reordering reuses each body; a new duplicate remains a separate resource.
func TestContentStore_ReorderSubscriptions(t *testing.T) {
	svc, cfg, revision := contentStoreFixture(t)
	cfg.Sources.Subscriptions = append(cfg.Sources.Subscriptions, config.Source{Content: "anytls://pw@sg.example.com:443#SG"})
	if _, err := saveContentDraft(t, svc, cfg, revision); err != nil {
		t.Fatal(err)
	}
	before := storedContentConfig(t, svc)
	edited, revision := editableContentConfig(t, svc)
	edited.Sources.Subscriptions[0], edited.Sources.Subscriptions[1] = edited.Sources.Subscriptions[1], edited.Sources.Subscriptions[0]
	if _, err := saveContentDraft(t, svc, &edited, revision); err != nil {
		t.Fatal(err)
	}
	after := storedContentConfig(t, svc)
	for i, j := range []int{1, 0} {
		if after.Sources.Subscriptions[i].ContentFile != before.Sources.Subscriptions[j].ContentFile {
			t.Fatalf("source %d lost reference after reorder: %+v", i, after.Sources.Subscriptions)
		}
	}
	entries, err := os.ReadDir(svc.dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 5 {
		t.Fatalf("reorder created versions: files=%d want 5", len(entries))
	}
	edited, revision = editableContentConfig(t, svc)
	edited.Sources.Subscriptions = append(edited.Sources.Subscriptions, edited.Sources.Subscriptions[0])
	if _, err := saveContentDraft(t, svc, &edited, revision); err != nil {
		t.Fatal(err)
	}
	after = storedContentConfig(t, svc)
	if len(after.Sources.Subscriptions) != 3 || after.Sources.Subscriptions[0].ContentFile == after.Sources.Subscriptions[2].ContentFile {
		t.Fatalf("new subscription lost independent resource: %+v", after.Sources.Subscriptions)
	}
}

// T-APP-STORE-013: explicitly editing an ordinary reference creates a managed copy even when text is identical.
func TestContentStore_CopyOrdinaryTemplate(t *testing.T) {
	svc, cfg, revision := contentStoreFixture(t)
	path := filepath.Join(filepath.Dir(svc.configLocation), "original.lcf")
	const body = "[General]\ndns-server = system\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	delete(cfg.Templates.Inline, "loon")
	cfg.Templates.Loon = path
	if _, err := saveContentDraft(t, svc, cfg, revision); err != nil {
		t.Fatal(err)
	}
	edited, revision := editableContentConfig(t, svc)
	edited.Templates.Loon = ""
	edited.Templates.Inline["loon"] = body
	if _, err := saveContentDraft(t, svc, &edited, revision); err != nil {
		t.Fatal(err)
	}
	after := storedContentConfig(t, svc)
	if after.Templates.Loon == path || !svc.isManagedTemplate(after.Templates.Loon, "loon") {
		t.Fatalf("ordinary reference reused instead of copying: %q", after.Templates.Loon)
	}
}

// T-APP-STORE-014: a failed config commit removes only new bodies and keeps reused versions readable.
func TestContentStore_ReuseRollback(t *testing.T) {
	svc, cfg, revision := contentStoreFixture(t)
	if _, err := saveContentDraft(t, svc, cfg, revision); err != nil {
		t.Fatal(err)
	}
	before := storedContentConfig(t, svc)
	edited, revision := editableContentConfig(t, svc)
	edited.Templates.Inline["loon"] += "loglevel = notify\n"
	original := svc.configLocation
	blocked := filepath.Join(filepath.Dir(original), "blocked")
	if err := os.Mkdir(blocked, 0o700); err != nil {
		t.Fatal(err)
	}
	clock := svc.now
	svc.now = func() time.Time { svc.configLocation = blocked; return clock() }
	_, err := saveContentDraft(t, svc, &edited, revision)
	svc.configLocation, svc.now = original, clock
	if !errors.Is(err, errtype.ErrConfigFileNotWritable) {
		t.Fatalf("save error=%v want config commit failure", err)
	}
	after := storedContentConfig(t, svc)
	if after.Templates.Loon != before.Templates.Loon {
		t.Fatal("failed save changed config reference")
	}
	entries, err := os.ReadDir(svc.dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("failed save retained new versions or removed old ones: %d", len(entries))
	}
	if _, err := svc.ReadResource(context.Background(), before.Templates.Clash); err != nil {
		t.Fatalf("reused file was removed: %v", err)
	}
}

// T-APP-STORE-015: malformed saved YAML does not prevent replacing it with a valid draft.
func TestContentStore_RepairInvalidSavedConfig(t *testing.T) {
	svc, cfg, _ := contentStoreFixture(t)
	raw := []byte("sources: [invalid yaml\n")
	if err := os.WriteFile(svc.configLocation, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := saveContentDraft(t, svc, cfg, configRevision(raw)); err != nil {
		t.Fatalf("valid replacement failed: %v", err)
	}
	if stored := storedContentConfig(t, svc); stored.Sources.Subscriptions[0].ContentFile == "" {
		t.Fatal("replacement did not save body")
	}
}

// T-APP-STORE-016: keeping one explicit reference must not consume another subscription's reusable version.
func TestContentStore_MixedReferencesAndBodies(t *testing.T) {
	svc, cfg, revision := contentStoreFixture(t)
	cfg.Sources.Subscriptions = append(cfg.Sources.Subscriptions, cfg.Sources.Subscriptions[0])
	if _, err := saveContentDraft(t, svc, cfg, revision); err != nil {
		t.Fatal(err)
	}
	before := storedContentConfig(t, svc)
	edited, revision := editableContentConfig(t, svc)
	edited.Sources.Subscriptions[0].Content = ""
	edited.Sources.Subscriptions[0].ContentFile = before.Sources.Subscriptions[0].ContentFile
	if _, err := saveContentDraft(t, svc, &edited, revision); err != nil {
		t.Fatal(err)
	}
	after := storedContentConfig(t, svc)
	for i := range before.Sources.Subscriptions {
		if after.Sources.Subscriptions[i].ContentFile != before.Sources.Subscriptions[i].ContentFile {
			t.Fatalf("source %d reference changed: old=%q new=%q", i, before.Sources.Subscriptions[i].ContentFile, after.Sources.Subscriptions[i].ContentFile)
		}
	}
}
