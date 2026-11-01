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
	cfg.Sources.Subscriptions = []config.Subscription{{Content: "anytls://secret-value@hk.example.com:443#HK-SAVED"}}
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
	if len(after) != 8 {
		t.Fatal("each save must retain a new timestamped version")
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
	old := svc.runtimeSnapshot().SourceInput().Subscriptions[0].ContentFile
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
	svc, cfg, _ := contentStoreFixture(t)
	location := filepath.Join(filepath.Dir(svc.configLocation), "nodes.lcf")
	const body = "[Proxy]\nHK = anytls, hk.example.com, 443, \"pw\"\n"
	if err := os.WriteFile(location, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.Sources.Subscriptions = []config.Subscription{{ContentFile: location}}
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
	for _, path := range []string{stored.Sources.Subscriptions[0].ContentFile, stored.Templates.Loon} {
		if filepath.Ext(path) != ".lcf" {
			t.Fatalf("Loon body filename = %q, want .lcf", path)
		}
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
