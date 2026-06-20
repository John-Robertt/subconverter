package app

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/fetch"
	"github.com/John-Robertt/subconverter/internal/generate"
)

const validConfigYAML = `
sources: {}
filters: {}
groups:
  HK:
    match: "(HK)"
    strategy: select
routing:
  proxy:
    - HK
    - DIRECT
rulesets: {}
rules: []
fallback: proxy
`

type mapFetcher struct {
	responses map[string][]byte
	calls     map[string]int
}

func (f *mapFetcher) Fetch(_ context.Context, rawURL string) ([]byte, error) {
	if f.calls != nil {
		f.calls[rawURL]++
	}
	body, ok := f.responses[rawURL]
	if !ok {
		return nil, errors.New("missing response")
	}
	return append([]byte(nil), body...), nil
}

// T-APP-001: config snapshot and save round-trip preserves content and updates revision
func TestConfigSnapshotAndSaveRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(validConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	svc, err := New(context.Background(), Options{ConfigLocation: path})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	snapshot, err := svc.ConfigSnapshot(context.Background())
	if err != nil {
		t.Fatalf("ConfigSnapshot: %v", err)
	}
	var body struct {
		Sources struct {
			FetchOrder []string `json:"fetch_order"`
		} `json:"sources"`
		Groups []struct {
			Key string `json:"key"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(snapshot.Config, &body); err != nil {
		t.Fatalf("unmarshal config json: %v", err)
	}
	if len(body.Groups) != 1 || body.Groups[0].Key != "HK" {
		t.Fatalf("groups JSON = %+v", body.Groups)
	}
	if got := body.Sources.FetchOrder; len(got) != 3 || got[0] != "subscriptions" || got[1] != "snell" || got[2] != "vless" {
		t.Fatalf("fetch_order = %v", got)
	}

	result, err := svc.SaveConfig(context.Background(), &SaveConfigInput{
		ConfigRevision: snapshot.ConfigRevision,
		Config:         snapshot.Config,
	})
	if err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	if result.ConfigRevision == "" || result.ConfigRevision == snapshot.ConfigRevision {
		t.Fatalf("new revision = %q, old = %q", result.ConfigRevision, snapshot.ConfigRevision)
	}
}

// T-APP-002: save config with stale revision returns 409 conflict
func TestSaveConfigRevisionConflict(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(validConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	svc, err := New(context.Background(), Options{ConfigLocation: path})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	snapshot, err := svc.ConfigSnapshot(context.Background())
	if err != nil {
		t.Fatalf("ConfigSnapshot: %v", err)
	}
	if err := os.WriteFile(path, []byte(validConfigYAML+"\n# external edit\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = svc.SaveConfig(context.Background(), &SaveConfigInput{
		ConfigRevision: snapshot.ConfigRevision,
		Config:         snapshot.Config,
	})
	var conflict *errtype.RevisionConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("error = %T %[1]v, want RevisionConflictError", err)
	}
	if conflict.CurrentConfigRevision == "" {
		t.Fatal("CurrentConfigRevision should be populated")
	}
}

// T-APP-012: effective config YAML tracks the last successfully loaded runtime config
func TestEffectiveConfigYAMLTracksRuntimeConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	initial := []byte("# active config\n" + validConfigYAML)
	if err := os.WriteFile(path, initial, 0o600); err != nil {
		t.Fatal(err)
	}

	svc, err := New(context.Background(), Options{ConfigLocation: path})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, err := svc.EffectiveConfigYAML(context.Background())
	if err != nil {
		t.Fatalf("EffectiveConfigYAML: %v", err)
	}
	if !bytes.Equal(got, initial) {
		t.Fatalf("effective yaml after New differs\ngot:\n%s\nwant:\n%s", got, initial)
	}

	snapshot, err := svc.ConfigSnapshot(context.Background())
	if err != nil {
		t.Fatalf("ConfigSnapshot: %v", err)
	}
	if _, err := svc.SaveConfig(context.Background(), &SaveConfigInput{
		ConfigRevision: snapshot.ConfigRevision,
		Config:         snapshot.Config,
	}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	afterSave, err := svc.EffectiveConfigYAML(context.Background())
	if err != nil {
		t.Fatalf("EffectiveConfigYAML after save: %v", err)
	}
	if !bytes.Equal(afterSave, initial) {
		t.Fatalf("save without reload should not change effective yaml\ngot:\n%s\nwant:\n%s", afterSave, initial)
	}

	savedRaw, err := os.ReadFile(path) // #nosec G304 -- test fixture path from t.TempDir.
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(savedRaw, initial) {
		t.Fatal("fixture expected SaveConfig to rewrite YAML bytes")
	}
	if _, err := svc.Reload(context.Background()); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	afterReload, err := svc.EffectiveConfigYAML(context.Background())
	if err != nil {
		t.Fatalf("EffectiveConfigYAML after reload: %v", err)
	}
	if !bytes.Equal(afterReload, savedRaw) {
		t.Fatalf("reload should update effective yaml\ngot:\n%s\nwant:\n%s", afterReload, savedRaw)
	}
}

// T-APP-013: YAML import produces API Config JSON with ordered fields preserved
func TestImportConfigYAML(t *testing.T) {
	raw := []byte(`
sources:
  snell: []
  subscriptions: []
  vless: []
groups:
  HK:
    match: "(HK)"
    strategy: select
  SG:
    match: "(SG)"
    strategy: url-test
routing: {}
rulesets: {}
rules: []
fallback: HK
`)
	svc := NewWithRuntime("", nil, nil, generateOptions())
	result, err := svc.ImportConfigYAML(context.Background(), raw)
	if err != nil {
		t.Fatalf("ImportConfigYAML: %v", err)
	}
	var body struct {
		Sources struct {
			FetchOrder []string `json:"fetch_order"`
		} `json:"sources"`
		Groups []struct {
			Key string `json:"key"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(result.Config, &body); err != nil {
		t.Fatalf("unmarshal imported config: %v", err)
	}
	if got := body.Sources.FetchOrder; len(got) != 3 || got[0] != "snell" || got[1] != "subscriptions" || got[2] != "vless" {
		t.Fatalf("fetch_order = %v", got)
	}
	if len(body.Groups) != 2 || body.Groups[0].Key != "HK" || body.Groups[1].Key != "SG" {
		t.Fatalf("groups = %+v", body.Groups)
	}

	if _, err := svc.ImportConfigYAML(context.Background(), []byte("sources: [")); err == nil {
		t.Fatal("expected invalid YAML error")
	}
}

// T-APP-014: effective config archive exports the active config and configured templates
func TestEffectiveConfigArchiveIncludesTemplates(t *testing.T) {
	dir := t.TempDir()
	clashPath := filepath.Join(dir, "base_clash.yaml")
	surgePath := filepath.Join(dir, "base_surge.conf")
	if err := os.WriteFile(clashPath, []byte("mixed-port: 7890\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(surgePath, []byte("[General]\nloglevel = notify\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	raw := []byte("templates:\n  clash: " + clashPath + "\n  surge: " + surgePath + "\n" + validConfigYAML)
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	archiveModifiedAt := time.Date(2026, 6, 20, 20, 30, 0, 0, time.UTC)
	svc, err := New(context.Background(), Options{
		ConfigLocation: configPath,
		Now: func() time.Time {
			return archiveModifiedAt
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := svc.EffectiveConfigArchive(context.Background())
	if err != nil {
		t.Fatalf("EffectiveConfigArchive: %v", err)
	}
	if result.Filename != ConfigArchiveFilename || result.ContentType != "application/zip" {
		t.Fatalf("archive metadata = %+v", result)
	}
	entries := readArchiveEntriesForTest(t, result.Body)
	for name, entry := range entries {
		if got := entry.Modified.UTC(); !got.Equal(archiveModifiedAt) {
			t.Fatalf("archive entry %q modified time = %s, want %s", name, got, archiveModifiedAt)
		}
	}
	if !bytes.Equal(entries[archiveConfigEntry].Body, raw) {
		t.Fatalf("config archive entry differs\ngot:\n%s\nwant:\n%s", entries[archiveConfigEntry].Body, raw)
	}
	if got := string(entries[archiveClashTemplateEntry].Body); got != "mixed-port: 7890\n" {
		t.Fatalf("clash template = %q", got)
	}
	if got := string(entries[archiveSurgeTemplateEntry].Body); got != "[General]\nloglevel = notify\n" {
		t.Fatalf("surge template = %q", got)
	}
}

// T-APP-015: config archive import writes local template copies and returns a draft config JSON
func TestImportConfigArchiveWritesTemplates(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(validConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	existingTemplateDir := filepath.Join(dir, "templates")
	if err := os.Mkdir(existingTemplateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	existingClashPath := filepath.Join(existingTemplateDir, "clash.yaml")
	if err := os.WriteFile(existingClashPath, []byte("old clash\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	existingSurgePath := filepath.Join(existingTemplateDir, "surge.conf")
	if err := os.WriteFile(existingSurgePath, []byte("old surge\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc, err := New(context.Background(), Options{ConfigLocation: configPath})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	archiveBody, err := buildConfigArchive([]archiveEntry{
		{Name: archiveConfigEntry, Body: []byte(validConfigYAML)},
		{Name: archiveClashTemplateEntry, Body: []byte("mixed-port: 7890\n")},
		{Name: archiveSurgeTemplateEntry, Body: []byte("[General]\nloglevel = notify\n")},
	}, time.Now())
	if err != nil {
		t.Fatalf("build archive: %v", err)
	}

	result, err := svc.ImportConfigArchive(context.Background(), archiveBody)
	if err != nil {
		t.Fatalf("ImportConfigArchive: %v", err)
	}
	var body struct {
		Templates struct {
			Clash string `json:"clash"`
			Surge string `json:"surge"`
		} `json:"templates"`
	}
	if err := json.Unmarshal(result.Config, &body); err != nil {
		t.Fatalf("unmarshal imported config: %v", err)
	}
	if !isImportedTemplatePath(body.Templates.Clash, dir, "clash.yaml") {
		t.Fatalf("clash template path = %q", body.Templates.Clash)
	}
	if !isImportedTemplatePath(body.Templates.Surge, dir, "surge.conf") {
		t.Fatalf("surge template path = %q", body.Templates.Surge)
	}
	clash, err := os.ReadFile(body.Templates.Clash) // #nosec G304 -- path returned by service under t.TempDir.
	if err != nil {
		t.Fatal(err)
	}
	if string(clash) != "mixed-port: 7890\n" {
		t.Fatalf("written clash template = %q", clash)
	}
	surge, err := os.ReadFile(body.Templates.Surge) // #nosec G304 -- path returned by service under t.TempDir.
	if err != nil {
		t.Fatal(err)
	}
	if string(surge) != "[General]\nloglevel = notify\n" {
		t.Fatalf("written surge template = %q", surge)
	}
	if oldClash, err := os.ReadFile(existingClashPath); err != nil || string(oldClash) != "old clash\n" { // #nosec G304 -- path is built under t.TempDir in this test.
		t.Fatalf("existing clash template = %q err=%v", oldClash, err)
	}
	if oldSurge, err := os.ReadFile(existingSurgePath); err != nil || string(oldSurge) != "old surge\n" { // #nosec G304 -- path is built under t.TempDir in this test.
		t.Fatalf("existing surge template = %q err=%v", oldSurge, err)
	}
}

// T-APP-016: config archive import rejects readonly sources and malformed archives
func TestImportConfigArchiveRejectsReadonlyAndInvalidArchive(t *testing.T) {
	remoteSvc := NewWithRuntime("https://config.example.com/config.yaml", nil, nil, generateOptions())
	_, err := remoteSvc.ImportConfigArchive(context.Background(), []byte("not zip"))
	if !errors.Is(err, errtype.ErrConfigSourceReadonly) {
		t.Fatalf("remote ImportConfigArchive error = %T %[1]v", err)
	}

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(validConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	svc, err := New(context.Background(), Options{ConfigLocation: path})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = svc.ImportConfigArchive(context.Background(), []byte("not zip"))
	var badReq *BadRequestError
	if !errors.As(err, &badReq) || badReq.Code != "invalid_archive" {
		t.Fatalf("invalid zip error = %T %[1]v, want invalid_archive", err)
	}

	archiveBody, err := buildConfigArchive([]archiveEntry{{Name: archiveClashTemplateEntry, Body: []byte("x")}}, time.Now())
	if err != nil {
		t.Fatalf("build archive: %v", err)
	}
	_, err = svc.ImportConfigArchive(context.Background(), archiveBody)
	if !errors.As(err, &badReq) || badReq.Code != "invalid_archive" {
		t.Fatalf("missing config error = %T %[1]v, want invalid_archive", err)
	}

	emptyConfigArchive, err := buildConfigArchive([]archiveEntry{{Name: archiveConfigEntry, Body: []byte(" \n")}}, time.Now())
	if err != nil {
		t.Fatalf("build empty config archive: %v", err)
	}
	_, err = svc.ImportConfigArchive(context.Background(), emptyConfigArchive)
	if !errors.As(err, &badReq) || badReq.Code != "invalid_archive" {
		t.Fatalf("empty config error = %T %[1]v, want invalid_archive", err)
	}

	directoryConfigArchive := buildDirectoryArchiveForTest(t, archiveConfigEntry)
	_, err = svc.ImportConfigArchive(context.Background(), directoryConfigArchive)
	if !errors.As(err, &badReq) || badReq.Code != "invalid_archive" {
		t.Fatalf("directory config error = %T %[1]v, want invalid_archive", err)
	}
}

// T-APP-017: config archive import does not overwrite existing templates on template write failures
func TestImportConfigArchiveTemplateWriteFailureDoesNotOverwriteExistingTemplates(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(validConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	existingTemplateDir := filepath.Join(dir, "templates")
	if err := os.Mkdir(existingTemplateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	existingClashPath := filepath.Join(existingTemplateDir, "clash.yaml")
	if err := os.WriteFile(existingClashPath, []byte("old clash\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, configImportsDirName), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc, err := New(context.Background(), Options{ConfigLocation: configPath})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	archiveBody, err := buildConfigArchive([]archiveEntry{
		{Name: archiveConfigEntry, Body: []byte(validConfigYAML)},
		{Name: archiveClashTemplateEntry, Body: []byte("new clash\n")},
	}, time.Now())
	if err != nil {
		t.Fatalf("build archive: %v", err)
	}

	_, err = svc.ImportConfigArchive(context.Background(), archiveBody)
	if !errors.Is(err, errtype.ErrTemplateFileNotWritable) {
		t.Fatalf("template write error = %T %[1]v, want ErrTemplateFileNotWritable", err)
	}
	if oldClash, err := os.ReadFile(existingClashPath); err != nil || string(oldClash) != "old clash\n" { // #nosec G304 -- path is built under t.TempDir in this test.
		t.Fatalf("existing clash template = %q err=%v", oldClash, err)
	}
}

// T-APP-003: save config returns 409 for readonly source and not-writable file
func TestSaveConfigReadonlySourceAndFileNotWritable(t *testing.T) {
	remoteSvc := NewWithRuntime("https://config.example.com/config.yaml", nil, nil, generateOptions())
	_, err := remoteSvc.SaveConfig(context.Background(), &SaveConfigInput{ConfigRevision: "sha256:x", Config: json.RawMessage(`{}`)})
	if !errors.Is(err, errtype.ErrConfigSourceReadonly) {
		t.Fatalf("remote SaveConfig error = %T %[1]v", err)
	}

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(validConfigYAML), 0o400); err != nil {
		t.Fatal(err)
	}
	svc, err := New(context.Background(), Options{ConfigLocation: path})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	snapshot, err := svc.ConfigSnapshot(context.Background())
	if err != nil {
		t.Fatalf("ConfigSnapshot: %v", err)
	}
	_, err = svc.SaveConfig(context.Background(), &SaveConfigInput{ConfigRevision: snapshot.ConfigRevision, Config: snapshot.Config})
	if !errors.Is(err, errtype.ErrConfigFileNotWritable) {
		t.Fatalf("readonly SaveConfig error = %T %[1]v", err)
	}
}

func generateOptions() generate.Options {
	return generate.Options{}
}

// T-APP-004: validate draft returns structured diagnostics with code and locator
func TestValidateDraftStructuredDiagnostics(t *testing.T) {
	svc := &Service{}
	result, err := svc.ValidateDraft(context.Background(), json.RawMessage(`{
		"sources": {"fetch_order":["subscriptions","subscriptions","vless"]},
		"groups": [],
		"routing": [],
		"rulesets": [],
		"rules": [],
		"fallback": ""
	}`))
	if err != nil {
		t.Fatalf("ValidateDraft: %v", err)
	}
	if result.Valid {
		t.Fatal("result.Valid = true, want false")
	}
	var hasFetchOrder bool
	for _, item := range result.Errors {
		if item.Code == "invalid_fetch_order" && item.Locator.JSONPointer == "/config/sources/fetch_order" {
			hasFetchOrder = true
		}
	}
	if !hasFetchOrder {
		t.Fatalf("diagnostics missing invalid_fetch_order: %+v", result.Errors)
	}
}

// T-APP-005: reload invalidates remote config cache and fetches fresh content
func TestReloadInvalidatesRemoteConfigCache(t *testing.T) {
	rawURL := "https://config.example.com/config.yaml"
	inner := &mapFetcher{responses: map[string][]byte{rawURL: []byte(validConfigYAML)}}
	cached := fetch.NewCachedFetcher(inner, time.Hour)
	svc, err := New(context.Background(), Options{ConfigLocation: rawURL, Fetcher: cached})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	inner.responses[rawURL] = []byte(`sources: {}`)
	_, err = svc.Reload(context.Background())
	if err == nil {
		t.Fatal("Reload error = nil, want validation error from refreshed remote config")
	}
	if result, ok := ValidateResultFromError(err); !ok || result.Valid {
		t.Fatalf("Reload error = %v, want validation diagnostics", err)
	}
	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status after failed reload: %v", err)
	}
	if !status.ConfigDirty {
		t.Fatalf("failed remote reload should keep runtime old and mark dirty: %+v", status)
	}
	if status.ConfigRevision == status.RuntimeConfigRevision {
		t.Fatalf("status should expose last observed remote revision after failed reload: %+v", status)
	}
	if status.LastReload == nil || status.LastReload.Time == "" || status.LastReload.Success {
		t.Fatalf("failed reload should be recorded in last_reload: %+v", status.LastReload)
	}
}

// T-APP-006: preview nodes runtime vs draft are isolated
func TestPreviewNodesRuntimeAndDraftAreIsolated(t *testing.T) {
	runtimeURL := "https://sub.example.com/runtime"
	draftURL := "https://sub.example.com/draft"
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(configYAML(runtimeURL, "(过期)", "HK")), 0o600); err != nil {
		t.Fatal(err)
	}
	fetcher := &mapFetcher{responses: map[string][]byte{
		runtimeURL: subResponse(
			"ss://YWVzLTI1Ni1jZmI6cGFzcw@hk.example.com:8388#HK-01",
			"ss://YWVzLTI1Ni1jZmI6cGFzcw@expire.example.com:8388#过期提醒",
		),
		draftURL: subResponse("ss://YWVzLTI1Ni1jZmI6cGFzcw@sg.example.com:8388#SG-01"),
	}}
	svc, err := New(context.Background(), Options{ConfigLocation: path, Fetcher: fetcher})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	before, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status before: %v", err)
	}

	runtimePreview, err := svc.PreviewNodes(context.Background())
	if err != nil {
		t.Fatalf("PreviewNodes: %v", err)
	}
	if runtimePreview.Total != 2 || runtimePreview.ActiveCount != 1 || runtimePreview.FilteredCount != 1 {
		t.Fatalf("runtime preview = %+v, want total=2 active=1 filtered=1", runtimePreview)
	}

	draftPreview, err := svc.PreviewNodesFromDraft(context.Background(), json.RawMessage(configJSON(draftURL, "", "SG")))
	if err != nil {
		t.Fatalf("PreviewNodesFromDraft: %v", err)
	}
	if draftPreview.Total != 1 || draftPreview.Nodes[0].Name != "SG-01" {
		t.Fatalf("draft preview = %+v, want SG-01 only", draftPreview)
	}

	after, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status after: %v", err)
	}
	if after.RuntimeConfigRevision != before.RuntimeConfigRevision || after.ConfigDirty != before.ConfigDirty || lastReloadTimeOf(after) != lastReloadTimeOf(before) {
		t.Fatalf("draft preview mutated status: before=%+v after=%+v", before, after)
	}
}

// T-APP-007: preview groups returns separated groups with expanded_members origin
func TestPreviewGroupsReturnsSeparatedGroupsAndExpandedMembers(t *testing.T) {
	subURL := "https://sub.example.com/runtime"
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(configYAML(subURL, "", "HK")), 0o600); err != nil {
		t.Fatal(err)
	}
	fetcher := &mapFetcher{responses: map[string][]byte{
		subURL: subResponse(
			"ss://YWVzLTI1Ni1jZmI6cGFzcw@hk.example.com:8388#HK-01",
			"ss://YWVzLTI1Ni1jZmI6cGFzcw@hk2.example.com:8388#HK-02",
		),
	}}
	svc, err := New(context.Background(), Options{ConfigLocation: path, Fetcher: fetcher})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := svc.PreviewGroups(context.Background())
	if err != nil {
		t.Fatalf("PreviewGroups: %v", err)
	}
	if len(result.NodeGroups) != 1 || result.NodeGroups[0].Name != "HK" {
		t.Fatalf("NodeGroups = %+v, want HK", result.NodeGroups)
	}
	if len(result.ChainedGroups) != 1 || result.ChainedGroups[0].Name != "SS-Chain" {
		t.Fatalf("ChainedGroups = %+v, want SS-Chain", result.ChainedGroups)
	}
	if len(result.ServiceGroups) != 1 {
		t.Fatalf("ServiceGroups = %+v, want one group", result.ServiceGroups)
	}
	var hasAllExpanded bool
	for _, member := range result.ServiceGroups[0].ExpandedMembers {
		if member.Value == "HK-01" && member.Origin == "all_expanded" {
			hasAllExpanded = true
		}
	}
	if !hasAllExpanded {
		t.Fatalf("ExpandedMembers missing all_expanded HK-01: %+v", result.ServiceGroups[0].ExpandedMembers)
	}
}

// T-APP-008: generate from draft does not mutate runtime status or config_dirty
func TestGenerateFromDraftDoesNotMutateRuntimeStatus(t *testing.T) {
	runtimeURL := "https://sub.example.com/runtime"
	draftURL := "https://sub.example.com/draft"
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(configYAML(runtimeURL, "", "HK")), 0o600); err != nil {
		t.Fatal(err)
	}
	fetcher := &mapFetcher{responses: map[string][]byte{
		runtimeURL: subResponse("ss://YWVzLTI1Ni1jZmI6cGFzcw@hk.example.com:8388#HK-01"),
		draftURL:   subResponse("ss://YWVzLTI1Ni1jZmI6cGFzcw@sg.example.com:8388#SG-01"),
	}}
	svc, err := New(context.Background(), Options{ConfigLocation: path, Fetcher: fetcher})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	before, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status before: %v", err)
	}

	result, err := svc.GenerateFromDraft(context.Background(), generate.Request{Format: "clash", Filename: "draft.yaml"}, json.RawMessage(configJSON(draftURL, "", "SG")))
	if err != nil {
		t.Fatalf("GenerateFromDraft: %v", err)
	}
	if !strings.Contains(string(result.Body), "SG-01") || strings.Contains(string(result.Body), "HK-01") {
		t.Fatalf("draft generate body should use draft only: %s", result.Body)
	}

	after, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status after: %v", err)
	}
	if after.RuntimeConfigRevision != before.RuntimeConfigRevision || after.ConfigDirty != before.ConfigDirty || lastReloadTimeOf(after) != lastReloadTimeOf(before) {
		t.Fatalf("draft generate mutated status: before=%+v after=%+v", before, after)
	}
}

// T-APP-009: status rehashes local config file; remote source does not trigger fetch
func TestStatusLocalRehashesAndRemoteDoesNotFetch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(validConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	svc, err := New(context.Background(), Options{
		ConfigLocation: path,
		Version:        "2.0.0",
		Commit:         "abc123",
		BuildDate:      "2026-05-03",
		Now:            func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("New local: %v", err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(validConfigYAML, "strategy: select", "strategy: select # same size not required", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status local: %v", err)
	}
	if !status.ConfigDirty || status.ConfigRevision == status.RuntimeConfigRevision {
		t.Fatalf("local status should rehash and become dirty: %+v", status)
	}
	if status.Version != "2.0.0" || status.ConfigSource.Type != "local" || !status.Capabilities.ConfigWrite {
		t.Fatalf("local status metadata = %+v", status)
	}

	remoteURL := "https://config.example.com/config.yaml"
	remoteFetcher := &mapFetcher{
		responses: map[string][]byte{remoteURL: []byte(validConfigYAML)},
		calls:     map[string]int{},
	}
	remoteSvc, err := New(context.Background(), Options{ConfigLocation: remoteURL, Fetcher: remoteFetcher})
	if err != nil {
		t.Fatalf("New remote: %v", err)
	}
	if _, err := remoteSvc.Status(context.Background()); err != nil {
		t.Fatalf("Status remote: %v", err)
	}
	if remoteFetcher.calls[remoteURL] != 1 {
		t.Fatalf("remote status fetched config; calls = %d, want startup only", remoteFetcher.calls[remoteURL])
	}
}

// T-PRV-004: status reflects local config file and directory writability
func TestStatusConfigWriteCapabilityReflectsLocalPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(validConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	svc, err := New(context.Background(), Options{ConfigLocation: path})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status writable: %v", err)
	}
	if !status.ConfigSource.Writable || !status.Capabilities.ConfigWrite {
		t.Fatalf("writable local config reported readonly: %+v", status)
	}

	if err := os.Chmod(path, 0o400); err != nil {
		t.Fatal(err)
	}
	status, err = svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status readonly file: %v", err)
	}
	if status.ConfigSource.Writable || status.Capabilities.ConfigWrite {
		t.Fatalf("readonly local file reported writable: %+v", status)
	}

	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil { //nolint:gosec // test needs execute bit while removing write permission.
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(dir, 0o700) }() //nolint:gosec // restore temp directory permissions for cleanup.
	status, err = svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status readonly dir: %v", err)
	}
	if status.ConfigSource.Writable || status.Capabilities.ConfigWrite {
		t.Fatalf("readonly config dir reported writable: %+v", status)
	}
}

// T-APP-010: generate link uses server token and base_url
func TestGenerateLinkUsesServerTokenAndBaseURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("base_url: https://example.com\n"+validConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	svc, err := New(context.Background(), Options{
		ConfigLocation: path,
		Generate:       generate.Options{AccessToken: "server-token"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := svc.GenerateLink(context.Background(), &GenerateLinkInput{Format: "surge", Filename: "phone.conf", IncludeToken: true})
	if err != nil {
		t.Fatalf("GenerateLink: %v", err)
	}
	want := "https://example.com/generate?format=surge&token=server-token&filename=phone.conf"
	if result.URL != want || !result.TokenIncluded {
		t.Fatalf("GenerateLink = %+v, want %s with token", result, want)
	}
	publicLink, err := svc.GenerateLink(context.Background(), &GenerateLinkInput{Format: "surge", Filename: "phone.conf", IncludeToken: false})
	if err != nil {
		t.Fatalf("GenerateLink include_token=false: %v", err)
	}
	if publicLink.TokenIncluded || strings.Contains(publicLink.URL, "token=") {
		t.Fatalf("GenerateLink include_token=false = %+v, want no token", publicLink)
	}

	noTokenSvc, err := New(context.Background(), Options{ConfigLocation: path})
	if err != nil {
		t.Fatalf("New without token: %v", err)
	}
	noToken, err := noTokenSvc.GenerateLink(context.Background(), &GenerateLinkInput{Format: "clash", Filename: "clash.yaml", IncludeToken: true})
	if err != nil {
		t.Fatalf("GenerateLink without token: %v", err)
	}
	if noToken.TokenIncluded || strings.Contains(noToken.URL, "token=") {
		t.Fatalf("GenerateLink without server token = %+v, want token_included=false and no token query", noToken)
	}

	noBasePath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(noBasePath, []byte(validConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	noBaseSvc, err := New(context.Background(), Options{ConfigLocation: noBasePath})
	if err != nil {
		t.Fatalf("New without base_url: %v", err)
	}
	_, err = noBaseSvc.GenerateLink(context.Background(), &GenerateLinkInput{Format: "surge", Filename: "phone.conf", IncludeToken: true})
	var badReq *BadRequestError
	if !errors.As(err, &badReq) || badReq.Code != "base_url_required" {
		t.Fatalf("GenerateLink without base_url error = %T %[1]v, want base_url_required", err)
	}
}

type archiveEntryForTest struct {
	Body     []byte
	Modified time.Time
}

func readArchiveEntriesForTest(t *testing.T, body []byte) map[string]archiveEntryForTest {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	entries := make(map[string]archiveEntryForTest, len(zr.File))
	for _, file := range zr.File {
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open archive entry %q: %v", file.Name, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read archive entry %q: %v", file.Name, err)
		}
		entries[file.Name] = archiveEntryForTest{
			Body:     data,
			Modified: file.Modified,
		}
	}
	return entries
}

func isImportedTemplatePath(path, configDir, filename string) bool {
	if !filepath.IsAbs(path) {
		return false
	}
	prefix := filepath.Join(configDir, configImportsDirName, "import-")
	suffix := filepath.Join("templates", filename)
	return strings.HasPrefix(path, prefix) && strings.HasSuffix(path, suffix)
}

func buildDirectoryArchiveForTest(t *testing.T, name string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	header := &zip.FileHeader{Name: name}
	header.SetMode(os.ModeDir | 0o700)
	if _, err := zw.CreateHeader(header); err != nil {
		t.Fatalf("create directory archive entry %q: %v", name, err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close directory archive: %v", err)
	}
	return buf.Bytes()
}

func subResponse(uris ...string) []byte {
	return []byte(base64.StdEncoding.EncodeToString([]byte(strings.Join(uris, "\n"))))
}

func configYAML(subURL, exclude, groupName string) string {
	return `
sources:
  subscriptions:
    - url: "` + subURL + `"
  custom_proxies:
    - name: SS-Chain
      url: "ss://YWVzLTI1Ni1nY206Y2hhaW5wYXNz@1.2.3.4:8388"
      relay_through:
        type: all
        strategy: select
filters:
  exclude: "` + exclude + `"
groups:
  ` + groupName + `:
    match: "(` + groupName + `)"
    strategy: select
routing:
  proxy:
    - ` + groupName + `
    - SS-Chain
    - "@all"
    - DIRECT
rulesets: {}
rules: []
fallback: proxy
`
}

func configJSON(subURL, exclude, groupName string) string {
	return `{
  "sources": {
    "subscriptions": [{"url":"` + subURL + `"}],
    "custom_proxies": [{
      "name": "SS-Chain",
      "url": "ss://YWVzLTI1Ni1nY206Y2hhaW5wYXNz@1.2.3.4:8388",
      "relay_through": {"type":"all","strategy":"select"}
    }]
  },
  "filters": {"exclude":"` + exclude + `"},
  "groups": [{"key":"` + groupName + `","value":{"match":"(` + groupName + `)","strategy":"select"}}],
  "routing": [{"key":"proxy","value":["` + groupName + `","SS-Chain","@all","DIRECT"]}],
  "rulesets": [],
  "rules": [],
  "fallback": "proxy"
}`
}

func lastReloadTimeOf(status *StatusResult) string {
	if status == nil || status.LastReload == nil {
		return ""
	}
	return status.LastReload.Time
}
