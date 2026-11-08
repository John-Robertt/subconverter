package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/John-Robertt/subconverter/internal/errtype"
)

const bootstrapBody = "anytls://test-password@hk.example.com:443#HK_BOOT"
const bootstrapYAML = `sources:
  subscriptions:
    - format: text
      content: "` + bootstrapBody + `"
groups:
  HK: {match: HK, strategy: select}
routing:
  proxy: [HK, DIRECT]
fallback: proxy
`

func bootstrapService(t *testing.T) *Service {
	t.Helper()
	dir := t.TempDir()
	svc, err := New(context.Background(), Options{ConfigLocation: filepath.Join(dir, "config.yaml"), DataDir: filepath.Join(dir, "data")})
	if err != nil {
		t.Fatalf("missing local config should start setup mode: %v", err)
	}
	return svc
}

// T-APP-BOOT-001: an empty deployment can import either package type, create a config and explicitly load it.
func TestBootstrap_ImportSaveReload(t *testing.T) {
	for _, kind := range []string{"yaml", "zip"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			svc := bootstrapService(t)
			status, err := svc.Status(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if status.ConfigSource.Exists || !status.ConfigSource.Writable || !status.Capabilities.ConfigWrite || status.Capabilities.Reload || status.ConfigDirty || status.ConfigLoadedAt != "" || status.RuntimeConfigRevision != "" || status.ConfigRevision != missingConfigRevision {
				t.Fatalf("invalid setup status: %+v", status)
			}
			snapshot, err := svc.ConfigSnapshot(ctx)
			if err != nil || snapshot.ConfigRevision != status.ConfigRevision {
				t.Fatalf("empty draft/revision: snapshot=%+v err=%v", snapshot, err)
			}
			if _, err := svc.SaveConfig(ctx, &SaveConfigInput{ConfigRevision: snapshot.ConfigRevision, Config: snapshot.Config}); err == nil {
				t.Fatal("an invalid empty draft was saved")
			}
			var imported *ImportConfigYAMLResult
			if kind == "yaml" {
				imported, err = svc.ImportConfigYAML(ctx, []byte(bootstrapYAML))
			} else {
				raw := strings.Replace(bootstrapYAML, `content: "`+bootstrapBody+`"`, "content_file: subscriptions/1.txt", 1)
				archive, buildErr := buildConfigArchive([]archiveEntry{{Name: "config.yaml", Body: []byte(raw)}, {Name: "subscriptions/1.txt", Body: []byte(bootstrapBody)}}, time.Now())
				if buildErr != nil {
					t.Fatal(buildErr)
				}
				imported, err = svc.ImportConfigArchive(ctx, archive)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(svc.configLocation); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("import created a main config: %v", err)
			}
			if _, err := os.Stat(svc.dataDir); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("import published body files: %v", err)
			}
			if _, err := svc.PreviewNodesFromDraft(ctx, imported.Config); err != nil {
				t.Fatalf("draft preview needs no active config: %v", err)
			}
			saved, err := svc.SaveConfig(ctx, &SaveConfigInput{ConfigRevision: snapshot.ConfigRevision, Config: imported.Config})
			if err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(svc.configLocation)
			if err != nil || info.Mode().Perm() != 0o600 {
				t.Fatalf("created main config permissions: info=%v err=%v", info, err)
			}
			status, err = svc.Status(ctx)
			if err != nil || !status.ConfigSource.Exists || !status.Capabilities.Reload || !status.ConfigDirty || status.RuntimeConfigRevision != "" || saved.ConfigRevision == missingConfigRevision {
				t.Fatalf("save must await reload: status=%+v saved=%+v err=%v", status, saved, err)
			}
			if _, err := svc.SavedConfigArchive(ctx); err != nil {
				t.Fatalf("saved export before reload: %v", err)
			}
			if _, err := svc.EffectiveConfigYAML(ctx); err == nil {
				t.Fatal("save activated a config without reload")
			}
			if _, err := svc.Reload(ctx); err != nil {
				t.Fatal(err)
			}
			status, err = svc.Status(ctx)
			if err != nil || status.ConfigDirty || status.RuntimeConfigRevision != saved.ConfigRevision || status.ConfigLoadedAt == "" {
				t.Fatalf("loaded status=%+v err=%v", status, err)
			}
			for _, format := range []string{"clash", "surge", "loon"} {
				output, err := svc.Generate(ctx, GenerateInput{Format: format, Filename: "config"})
				if err != nil {
					t.Fatalf("%s generation: %v", format, err)
				}
				if !strings.Contains(string(output.Body), "HK_BOOT") {
					t.Fatalf("%s lost imported node", format)
				}
			}
			if _, err := New(ctx, Options{ConfigLocation: svc.configLocation, DataDir: svc.dataDir}); err != nil {
				t.Fatalf("restart after first save: %v", err)
			}
		})
	}
}

// T-APP-BOOT-002: runtime-only operations fail explicitly while editing and draft checks remain available.
func TestBootstrap_RuntimeUnavailable(t *testing.T) {
	svc := bootstrapService(t)
	ctx := context.Background()
	for _, tt := range []struct {
		name string
		run  func() error
	}{
		{"generate", func() error { _, err := svc.Generate(ctx, GenerateInput{Format: "clash"}); return err }},
		{"nodes", func() error { _, err := svc.PreviewNodes(ctx); return err }},
		{"groups", func() error { _, err := svc.PreviewGroups(ctx); return err }},
		{"link", func() error { _, err := svc.GenerateLink(ctx, &GenerateLinkInput{Format: "clash"}); return err }},
		{"effective YAML", func() error { _, err := svc.EffectiveConfigYAML(ctx); return err }},
		{"saved ZIP", func() error { _, err := svc.SavedConfigArchive(ctx); return err }},
		{"effective ZIP", func() error { _, err := svc.EffectiveConfigArchive(ctx); return err }},
		{"reload", func() error { _, err := svc.Reload(ctx); return err }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var configErr *errtype.ConfigError
			err := tt.run()
			if !errors.As(err, &configErr) || configErr.Code != errtype.CodeConfigNotInitialized {
				t.Fatalf("got=%v want config_not_initialized", err)
			}
		})
	}
}

// T-APP-BOOT-003: first publication cannot overwrite an externally created file and rolls back new body versions.
func TestBootstrap_CreateConflict(t *testing.T) {
	for _, timing := range []string{"before save", "during body publication"} {
		t.Run(timing, func(t *testing.T) {
			ctx := context.Background()
			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")
			external := []byte(bootstrapYAML + "# external writer\n")
			armed, published := false, false
			svc, err := New(ctx, Options{ConfigLocation: path, DataDir: filepath.Join(dir, "data"), Now: func() time.Time {
				if armed && !published {
					if err := os.WriteFile(path, external, 0o600); err != nil {
						t.Fatal(err)
					}
					published = true
				}
				return time.Now()
			}})
			if err != nil {
				t.Fatal(err)
			}
			imported, err := svc.ImportConfigYAML(ctx, []byte(bootstrapYAML))
			if err != nil {
				t.Fatal(err)
			}
			if timing == "before save" {
				if err := os.WriteFile(path, external, 0o600); err != nil {
					t.Fatal(err)
				}
			} else {
				armed = true
			}
			_, err = svc.SaveConfig(ctx, &SaveConfigInput{ConfigRevision: missingConfigRevision, Config: imported.Config})
			var conflict *errtype.RevisionConflictError
			if !errors.As(err, &conflict) || conflict.CurrentConfigRevision != configRevision(external) {
				t.Fatalf("got=%v want first-create revision conflict", err)
			}
			actual, err := os.ReadFile(path) // #nosec G304 -- fixture path is inside t.TempDir.
			if err != nil || string(actual) != string(external) {
				t.Fatalf("external file changed: body=%q err=%v", actual, err)
			}
			entries, err := os.ReadDir(svc.dataDir)
			if err != nil && !errors.Is(err, os.ErrNotExist) || len(entries) != 0 {
				t.Fatalf("failed save retained body versions: entries=%v err=%v", entries, err)
			}
		})
	}
}

// T-APP-BOOT-004: directory permissions and existing invalid files do not become writable setup defaults.
func TestBootstrap_PermissionsAndInvalidFiles(t *testing.T) {
	t.Run("broken existing symlink", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		if err := os.Symlink(filepath.Join(dir, "missing-target.yaml"), path); err != nil {
			t.Fatal(err)
		}
		if _, err := New(context.Background(), Options{ConfigLocation: path}); err == nil {
			t.Fatal("an existing unreadable symlink silently entered setup mode")
		}
	})
	t.Run("readonly directory", func(t *testing.T) {
		svc := bootstrapService(t)
		dir := filepath.Dir(svc.configLocation)
		if err := os.Chmod(dir, 0o500); err != nil { // #nosec G302 -- Owner search/read access keeps the missing config inspectable.
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) // #nosec G302 -- Restores temporary directory access for cleanup.
		status, err := svc.Status(context.Background())
		if err != nil || status.ConfigSource.Exists || status.ConfigSource.Writable || status.Capabilities.ConfigWrite {
			t.Fatalf("readonly setup status=%+v err=%v", status, err)
		}
		snapshot, err := svc.ConfigSnapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		_, err = svc.SaveConfig(context.Background(), &SaveConfigInput{ConfigRevision: snapshot.ConfigRevision, Config: json.RawMessage(`{}`)})
		if !errors.Is(err, errtype.ErrConfigFileNotWritable) {
			t.Fatalf("readonly first save: %v", err)
		}
	})
	for _, body := range []string{"", "sources: [", "{}"} {
		t.Run("invalid "+body, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := New(context.Background(), Options{ConfigLocation: path}); err == nil {
				t.Fatal("an existing invalid file silently entered setup mode")
			}
		})
	}
}

// T-APP-BOOT-005: independent service instances cannot both publish the first main config.
func TestBootstrap_ConcurrentCreators(t *testing.T) {
	ctx := context.Background()
	first := bootstrapService(t)
	second, err := New(ctx, Options{ConfigLocation: first.configLocation, DataDir: first.dataDir})
	if err != nil {
		t.Fatal(err)
	}
	imported, err := first.ImportConfigYAML(ctx, []byte(bootstrapYAML))
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errorsFound := make(chan error, 2)
	for _, svc := range []*Service{first, second} {
		go func() {
			<-start
			_, err := svc.SaveConfig(ctx, &SaveConfigInput{ConfigRevision: missingConfigRevision, Config: imported.Config})
			errorsFound <- err
		}()
	}
	close(start)
	successes, conflicts := 0, 0
	for range 2 {
		err := <-errorsFound
		var conflict *errtype.RevisionConflictError
		switch {
		case err == nil:
			successes++
		case errors.As(err, &conflict):
			conflicts++
		default:
			t.Fatalf("unexpected create failure: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successful creators=%d conflicting creators=%d want 1/1", successes, conflicts)
	}
	entries, err := os.ReadDir(first.dataDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("losing creator retained body versions: entries=%v err=%v", entries, err)
	}
}
