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

// T-APP-LOON-001: archive import, config saving and export preserve Loon format and template.
func TestApp_LoonArchiveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(validConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	svc, err := New(context.Background(), Options{ConfigLocation: path, DataDir: filepath.Join(dir, "data")})
	if err != nil {
		t.Fatal(err)
	}
	const raw = "sources:\n  subscriptions:\n    - {url: https://example.com/sub.conf, format: loon}\ngroups:\n  HK: {match: HK, strategy: select}\nrouting:\n  proxy: [HK, DIRECT]\nfallback: proxy\ntemplates:\n  loon: templates/loon.lcf\n"
	const tmpl = "[General]\ndns-server = system\n"
	archive, err := buildConfigArchive([]archiveEntry{{Name: archiveConfigEntry, Body: []byte(raw)}, {Name: archiveLoonTemplateEntry, Body: []byte(tmpl)}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	imported, err := svc.ImportConfigArchive(context.Background(), archive)
	if err != nil {
		t.Fatal(err)
	}
	var cfg config.Config
	if err := json.Unmarshal(imported.Config, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Templates.Loon != "" || cfg.Templates.Inline["loon"] != tmpl || cfg.Sources.Subscriptions[0].Format != "loon" {
		t.Fatalf("import lost Loon config: %+v", cfg)
	}
	snapshot, err := svc.ConfigSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveConfig(context.Background(), &SaveConfigInput{ConfigRevision: snapshot.ConfigRevision, Config: imported.Config}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	exported, err := svc.EffectiveConfigArchive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	entries := readArchiveEntriesForTest(t, exported.Body)
	if _, ok := entries["templates/loon.conf"]; ok {
		t.Fatal("archive used .conf for Loon template")
	}
	if string(entries[archiveLoonTemplateEntry].Body) != tmpl || !strings.Contains(string(entries[archiveConfigEntry].Body), "format: loon") {
		t.Fatalf("archive lost Loon template or format: %+v", entries)
	}
	if !strings.Contains(string(entries[archiveConfigEntry].Body), "templates/loon.lcf") {
		t.Fatalf("archive config lost .lcf reference: %s", entries[archiveConfigEntry].Body)
	}
}
