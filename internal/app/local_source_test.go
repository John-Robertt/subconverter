package app

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/config"
)

// T-APP-LOCAL-001: node pools preserve editable names/bodies across save, restart and both archive exports.
func TestContentStore_LocalNodePools(t *testing.T) {
	ctx := context.Background()
	svc, cfg, revision := contentStoreFixture(t)
	cfg.Sources.Snell = []config.Source{{Name: "手动 Snell", Content: "HK-SN = snell, hk.example.com, 443, psk=pw, version=4\n"}}
	cfg.Sources.VLess = []config.Source{{Name: "手动 VLESS", Content: "vless://11111111-1111-1111-1111-111111111111@hk.example.com:443?type=tcp#HK-VL\n"}}
	saved, err := saveContentDraft(t, svc, cfg, revision)
	if err != nil {
		t.Fatal(err)
	}
	stored := storedContentConfig(t, svc)
	for _, list := range stored.Sources.Lists() {
		if source := list.Entries[0]; source.Content != "" || source.ContentFile == "" || source.Name == "" {
			t.Fatalf("%s did not save a named file reference: %+v", list.Kind, source)
		}
	}
	if _, err := saveContentDraft(t, svc, cfg, saved.ConfigRevision); err != nil {
		t.Fatal(err)
	}
	unchanged := storedContentConfig(t, svc)
	if unchanged.Sources.Snell[0] != stored.Sources.Snell[0] || unchanged.Sources.VLess[0] != stored.Sources.VLess[0] {
		t.Fatalf("unchanged save published new pool versions: %+v", unchanged.Sources)
	}
	restarted, err := New(ctx, Options{ConfigLocation: svc.configLocation, DataDir: svc.dataDir})
	if err != nil {
		t.Fatal(err)
	}
	editable, revision := editableContentConfig(t, restarted)
	if editable.Sources.Snell[0] != cfg.Sources.Snell[0] || editable.Sources.VLess[0] != cfg.Sources.VLess[0] {
		t.Fatalf("restart lost named editable pools: %+v", editable.Sources)
	}
	for _, format := range []string{"clash", "surge", "loon"} {
		request, err := NewGenerateInput(format, "", false)
		if err != nil {
			t.Fatal(err)
		}
		output, err := restarted.Generate(ctx, request)
		if err != nil {
			t.Fatalf("%s generation: %v", format, err)
		}
		body := string(output.Body)
		if strings.Contains(body, "HK-SN") != (format == "surge") || strings.Contains(body, "HK-VL") != (format != "surge") || !strings.Contains(body, "HK-SAVED") {
			t.Fatalf("%s changed protocol filtering: %s", format, body)
		}
	}
	for _, mode := range []string{"saved", "effective"} {
		t.Run(mode, func(t *testing.T) {
			export := restarted.SavedConfigArchive
			if mode == "effective" {
				export = restarted.EffectiveConfigArchive
			}
			archive, err := export(ctx)
			if err != nil {
				t.Fatal(err)
			}
			entries, err := readConfigArchive(archive.Body)
			if err != nil {
				t.Fatal(err)
			}
			if string(entries["snell/1.txt"]) != cfg.Sources.Snell[0].Content || string(entries["vless/1.txt"]) != cfg.Sources.VLess[0].Content {
				t.Fatal("archive lost pool files or changed their bytes")
			}
			packaged, err := parseConfigYAML(entries[archiveConfigEntry])
			if err != nil {
				t.Fatal(err)
			}
			if packaged.Sources.Snell[0].ContentFile != "snell/1.txt" || packaged.Sources.VLess[0].ContentFile != "vless/1.txt" {
				t.Fatalf("archive lost relative pool references: %+v", packaged.Sources)
			}
			back, err := restarted.ImportConfigArchive(ctx, archive.Body)
			if err != nil {
				t.Fatal(err)
			}
			imported, err := parseConfigJSON(back.Config)
			if err != nil {
				t.Fatal(err)
			}
			if imported.Sources.Snell[0] != cfg.Sources.Snell[0] || imported.Sources.VLess[0] != cfg.Sources.VLess[0] {
				t.Fatalf("archive round trip changed named pools: %+v", imported.Sources)
			}
		})
	}
	// Updating a pool publishes a version without changing the active snapshot or its file.
	oldSnell := cfg.Sources.Snell[0].Content
	cfg.Sources.Snell[0].Content = strings.ReplaceAll(oldSnell, "HK-SN", "HK-SN-NEW")
	changed, err := saveContentDraft(t, restarted, cfg, revision)
	if err != nil {
		t.Fatal(err)
	}
	request, err := NewGenerateInput("surge", "", false)
	if err != nil {
		t.Fatal(err)
	}
	output, err := restarted.Generate(ctx, request)
	if err != nil || strings.Contains(string(output.Body), "HK-SN-NEW") {
		t.Fatalf("saving changed the active pool before reload: %v", err)
	}
	previous, err := os.ReadFile(stored.Sources.Snell[0].ContentFile) // #nosec G304 -- fixture path is a managed version inside t.TempDir.
	if err != nil || string(previous) != oldSnell {
		t.Fatalf("old pool version changed: body=%q err=%v", previous, err)
	}
	if _, err := restarted.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	output, err = restarted.Generate(ctx, request)
	if err != nil || !strings.Contains(string(output.Body), "HK-SN-NEW") {
		t.Fatalf("reload did not activate the new pool: %v", err)
	}
	revision = changed.ConfigRevision
	// A missing pool file stays repairable in the editor and blocks incomplete archives.
	if err := os.Remove(stored.Sources.VLess[0].ContentFile); err != nil {
		t.Fatal(err)
	}
	broken, _ := editableContentConfig(t, restarted)
	if broken.Sources.VLess[0].ContentFile != stored.Sources.VLess[0].ContentFile || broken.Sources.Snell[0].Content != cfg.Sources.Snell[0].Content {
		t.Fatalf("missing VLESS body damaged other editor inputs: %+v", broken.Sources)
	}
	if _, err := restarted.SavedConfigArchive(ctx); err == nil {
		t.Fatal("archive silently omitted the missing VLESS file")
	}
	if _, err := saveContentDraft(t, restarted, cfg, revision); err != nil {
		t.Fatalf("repairing missing pool file: %v", err)
	}
}
