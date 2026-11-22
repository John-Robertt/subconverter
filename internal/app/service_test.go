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
	"sync"
	"testing"
	"time"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/fetch"
	"github.com/John-Robertt/subconverter/internal/generate"
	"github.com/John-Robertt/subconverter/internal/model"
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
		return nil, errors.New("缺少响应")
	}
	return append([]byte(nil), body...), nil
}

// T-APP-001：配置快照与保存的往返过程保留内容并更新 revision
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
		t.Fatalf("反序列化配置 JSON：%v", err)
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
		t.Fatalf("新 revision = %q，旧 revision = %q", result.ConfigRevision, snapshot.ConfigRevision)
	}
}

// T-APP-002：用过期 revision 保存配置时返回 409 冲突
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
		t.Fatalf("错误 = %T %[1]v，期望 RevisionConflictError", err)
	}
	if conflict.CurrentConfigRevision == "" {
		t.Fatal("CurrentConfigRevision 应有值")
	}
}

// T-APP-012：生效配置 YAML 跟随最近一次成功加载的运行时配置
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
		t.Fatalf("New 之后的生效 YAML 不一致\n实际：\n%s\n期望：\n%s", got, initial)
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
		t.Fatalf("保存后调用 EffectiveConfigYAML：%v", err)
	}
	if !bytes.Equal(afterSave, initial) {
		t.Fatalf("未热重载时保存不应改变生效 YAML\n实际：\n%s\n期望：\n%s", afterSave, initial)
	}

	savedRaw, err := os.ReadFile(path) // #nosec G304 -- 测试 fixture 路径来自 t.TempDir。
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(savedRaw, initial) {
		t.Fatal("该 fixture 预期 SaveConfig 会重写 YAML 字节")
	}
	if _, err := svc.Reload(context.Background()); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	afterReload, err := svc.EffectiveConfigYAML(context.Background())
	if err != nil {
		t.Fatalf("热重载后调用 EffectiveConfigYAML：%v", err)
	}
	if !bytes.Equal(afterReload, savedRaw) {
		t.Fatalf("热重载应更新生效 YAML\n实际：\n%s\n期望：\n%s", afterReload, savedRaw)
	}
}

// T-APP-013：导入 YAML 生成 API 使用的 Config JSON，并保留有序字段的顺序
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
		t.Fatalf("反序列化导入的配置：%v", err)
	}
	if got := body.Sources.FetchOrder; len(got) != 3 || got[0] != "snell" || got[1] != "subscriptions" || got[2] != "vless" {
		t.Fatalf("fetch_order = %v", got)
	}
	if len(body.Groups) != 2 || body.Groups[0].Key != "HK" || body.Groups[1].Key != "SG" {
		t.Fatalf("groups = %+v", body.Groups)
	}

	if _, err := svc.ImportConfigYAML(context.Background(), []byte("sources: [")); err == nil {
		t.Fatal("期望得到 YAML 无效错误")
	}
}

// T-APP-014：生效配置包导出当前生效的配置和已配置的模板
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
		t.Fatalf("配置包元数据 = %+v", result)
	}
	entries := readArchiveEntriesForTest(t, result.Body)
	for name, entry := range entries {
		if got := entry.Modified.UTC(); !got.Equal(archiveModifiedAt) {
			t.Fatalf("配置包条目 %q 的修改时间 = %s，期望 %s", name, got, archiveModifiedAt)
		}
	}
	exported, err := parseConfigYAML(entries[archiveConfigEntry].Body)
	if err != nil {
		t.Fatal(err)
	}
	if exported.Templates.Clash != archiveClashTemplateEntry || exported.Templates.Surge != archiveSurgeTemplateEntry {
		t.Fatalf("配置包中的模板引用不可移植：%+v", exported.Templates)
	}
	if got := string(entries[archiveClashTemplateEntry].Body); got != "mixed-port: 7890\n" {
		t.Fatalf("clash 模板 = %q", got)
	}
	if got := string(entries[archiveSurgeTemplateEntry].Body); got != "[General]\nloglevel = notify\n" {
		t.Fatalf("surge 模板 = %q", got)
	}
}

// T-APP-015：导入配置包只停留在内存中；由保存负责发布，两种导出都能往返。
func TestImportConfigArchiveDraftSaveExport(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(validConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	svc, err := New(ctx, Options{ConfigLocation: path, DataDir: filepath.Join(dir, "data")})
	if err != nil {
		t.Fatal(err)
	}
	archive, err := buildConfigArchive([]archiveEntry{
		{Name: archiveConfigEntry, Body: []byte(validConfigYAML)},
		{Name: archiveClashTemplateEntry, Body: []byte("mixed-port: 7890\n")},
		{Name: archiveSurgeTemplateEntry, Body: []byte("[General]\nloglevel = notify\n")},
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	imported, err := svc.ImportConfigArchive(ctx, archive)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := parseConfigJSON(imported.Config)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Templates.Clash != "" || cfg.Templates.Inline["clash"] != "mixed-port: 7890\n" {
		t.Fatalf("导入的模板 = %+v", cfg.Templates)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 1 {
		t.Fatalf("导入时写入了文件：%v err=%v", files, err)
	}
	// 编辑导入返回的同一份正文，再通过 SaveConfig 发布。
	cfg.Templates.Inline["clash"] = "mixed-port: 17890\n"
	cfg.Sources.Subscriptions = []config.Source{{Format: "text", Content: "anytls://pw@hk.example.com:443#HK"}}
	edited, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveConfig(ctx, &SaveConfigInput{ConfigRevision: configRevision([]byte(validConfigYAML)), Config: edited}); err != nil {
		t.Fatal(err)
	}
	saved, err := svc.SavedConfigArchive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	entries := readArchiveEntriesForTest(t, saved.Body)
	if string(entries[archiveClashTemplateEntry].Body) != cfg.Templates.Inline["clash"] || strings.Contains(string(entries[archiveConfigEntry].Body), dir) {
		t.Fatalf("已保存配置包丢失了内容或可移植性：%+v", entries)
	}
	effective, err := svc.EffectiveConfigArchive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := readArchiveEntriesForTest(t, effective.Body)[archiveClashTemplateEntry]; ok {
		t.Fatal("保存在热重载前就改变了生效快照")
	}
	back, err := svc.ImportConfigArchive(ctx, saved.Body)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := parseConfigJSON(back.Config)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Templates.Inline["clash"] != cfg.Templates.Inline["clash"] || restored.Sources.Subscriptions[0] != cfg.Sources.Subscriptions[0] {
		t.Fatalf("往返后草稿被改动：%+v", restored)
	}
}

// T-APP-016：导入配置包时拒绝只读来源和格式错误的配置包
func TestImportConfigArchiveRejectsReadonlyAndInvalidArchive(t *testing.T) {
	remoteSvc := NewWithRuntime("https://config.example.com/config.yaml", nil, nil, generateOptions())
	_, err := remoteSvc.ImportConfigArchive(context.Background(), []byte("not zip"))
	if !errors.Is(err, errtype.ErrConfigSourceReadonly) {
		t.Fatalf("远程配置调用 ImportConfigArchive 的错误 = %T %[1]v", err)
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
		t.Fatalf("无效 ZIP 的错误 = %T %[1]v，期望 invalid_archive", err)
	}

	archiveBody, err := buildConfigArchive([]archiveEntry{{Name: archiveClashTemplateEntry, Body: []byte("x")}}, time.Now())
	if err != nil {
		t.Fatalf("构建配置包：%v", err)
	}
	_, err = svc.ImportConfigArchive(context.Background(), archiveBody)
	if !errors.As(err, &badReq) || badReq.Code != "invalid_archive" {
		t.Fatalf("缺少配置时的错误 = %T %[1]v，期望 invalid_archive", err)
	}

	emptyConfigArchive, err := buildConfigArchive([]archiveEntry{{Name: archiveConfigEntry, Body: []byte(" \n")}}, time.Now())
	if err != nil {
		t.Fatalf("构建空配置的配置包：%v", err)
	}
	_, err = svc.ImportConfigArchive(context.Background(), emptyConfigArchive)
	if !errors.As(err, &badReq) || badReq.Code != "invalid_archive" {
		t.Fatalf("空配置的错误 = %T %[1]v，期望 invalid_archive", err)
	}

	directoryConfigArchive := buildDirectoryArchiveForTest(t, archiveConfigEntry)
	_, err = svc.ImportConfigArchive(context.Background(), directoryConfigArchive)
	if !errors.As(err, &badReq) || badReq.Code != "invalid_archive" {
		t.Fatalf("配置为目录时的错误 = %T %[1]v，期望 invalid_archive", err)
	}
}

// T-APP-017：废弃的导入目录不会影响草稿导入或已有文件。
func TestImportConfigArchiveDoesNotUseImportDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(validConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	blocked := filepath.Join(dir, ".imports")
	if err := os.WriteFile(blocked, []byte("legacy sentinel"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc, err := New(context.Background(), Options{ConfigLocation: path})
	if err != nil {
		t.Fatal(err)
	}
	archive, err := buildConfigArchive([]archiveEntry{{Name: archiveConfigEntry, Body: []byte(validConfigYAML)}, {Name: archiveClashTemplateEntry, Body: []byte("new clash\n")}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ImportConfigArchive(context.Background(), archive); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(blocked); err != nil || string(got) != "legacy sentinel" { // #nosec G304 -- 路径由本测试在 t.TempDir 下创建。
		t.Fatalf("旧文件被改动：%q err=%v", got, err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != validConfigYAML { // #nosec G304 -- 路径由本测试在 t.TempDir 下创建。
		t.Fatalf("导入过程中配置被改动：%q err=%v", got, err)
	}
}

// T-APP-003：来源只读或文件不可写时，保存配置返回 409
func TestSaveConfigReadonlySourceAndFileNotWritable(t *testing.T) {
	remoteSvc := NewWithRuntime("https://config.example.com/config.yaml", nil, nil, generateOptions())
	_, err := remoteSvc.SaveConfig(context.Background(), &SaveConfigInput{ConfigRevision: "sha256:x", Config: json.RawMessage(`{}`)})
	if !errors.Is(err, errtype.ErrConfigSourceReadonly) {
		t.Fatalf("远程配置调用 SaveConfig 的错误 = %T %[1]v", err)
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
		t.Fatalf("只读文件调用 SaveConfig 的错误 = %T %[1]v", err)
	}
}

func generateOptions() generate.Options {
	return generate.Options{}
}

// T-APP-004：校验草稿返回带错误码和定位信息的结构化诊断
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
		t.Fatal("result.Valid = true，期望 false")
	}
	var hasFetchOrder bool
	for _, item := range result.Errors {
		if item.Code == "invalid_fetch_order" && item.Locator.JSONPointer == "/config/sources/fetch_order" {
			hasFetchOrder = true
		}
	}
	if !hasFetchOrder {
		t.Fatalf("诊断结果缺少 invalid_fetch_order：%+v", result.Errors)
	}
}

// T-APP-005：热重载使远程配置缓存失效并拉取最新内容
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
		t.Fatal("Reload 错误 = nil，期望刷新后的远程配置产生校验错误")
	}
	if result, ok := ValidateResultFromError(err); !ok || result.Valid {
		t.Fatalf("Reload 错误 = %v，期望得到校验诊断", err)
	}
	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("热重载失败后调用 Status：%v", err)
	}
	if !status.ConfigDirty {
		t.Fatalf("远程热重载失败时应保留旧的运行时配置并标记为 dirty：%+v", status)
	}
	if status.ConfigRevision == status.RuntimeConfigRevision {
		t.Fatalf("热重载失败后，status 应给出最近观察到的远程 revision：%+v", status)
	}
	if status.LastReload == nil || status.LastReload.Time == "" || status.LastReload.Success {
		t.Fatalf("热重载失败应记录在 last_reload 中：%+v", status.LastReload)
	}
}

// T-APP-006：运行时与草稿的节点预览相互隔离
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
		t.Fatalf("操作前调用 Status：%v", err)
	}

	runtimePreview, err := svc.PreviewNodes(context.Background())
	if err != nil {
		t.Fatalf("PreviewNodes: %v", err)
	}
	if runtimePreview.Total != 2 || runtimePreview.ActiveCount != 1 || runtimePreview.FilteredCount != 1 {
		t.Fatalf("运行时预览 = %+v，期望 total=2 active=1 filtered=1", runtimePreview)
	}

	draftPreview, err := svc.PreviewNodesFromDraft(context.Background(), json.RawMessage(configJSON(draftURL, "", "SG")))
	if err != nil {
		t.Fatalf("PreviewNodesFromDraft: %v", err)
	}
	if draftPreview.Total != 1 || draftPreview.Nodes[0].Name != "SG-01" {
		t.Fatalf("草稿预览 = %+v，期望只有 SG-01", draftPreview)
	}

	after, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("操作后调用 Status：%v", err)
	}
	if after.RuntimeConfigRevision != before.RuntimeConfigRevision || after.ConfigDirty != before.ConfigDirty || lastReloadTimeOf(after) != lastReloadTimeOf(before) {
		t.Fatalf("草稿预览改动了状态：before=%+v after=%+v", before, after)
	}
}

// T-APP-007：分组预览保留旧字段，并补充带身份信息的详情
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
		t.Fatalf("NodeGroups = %+v，期望 HK", result.NodeGroups)
	}
	if len(result.ChainedGroups) != 1 || result.ChainedGroups[0].Name != "SS-Chain" {
		t.Fatalf("ChainedGroups = %+v，期望 SS-Chain", result.ChainedGroups)
	}
	if len(result.ServiceGroups) != 1 {
		t.Fatalf("ServiceGroups = %+v，期望只有一个组", result.ServiceGroups)
	}
	if len(result.NodeGroups[0].Members) != 2 || result.NodeGroups[0].Members[0] != "HK-01" {
		t.Fatalf("旧字段中的节点成员 = %+v，期望为节点名", result.NodeGroups[0].Members)
	}
	if len(result.AllProxies) != 2 || result.AllProxies[0] != "HK-01" {
		t.Fatalf("旧字段 all_proxies = %+v，期望为节点名", result.AllProxies)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("序列化预览结果：%v", err)
	}
	for _, want := range []string{
		`"members":["HK-01","HK-02"]`,
		`"member_details":[{"kind":"proxy","id":"proxy-1","name":"HK-01"`,
		`"all_proxies":["HK-01","HK-02"]`,
		`"all_proxy_details":[{"kind":"proxy","id":"proxy-1","name":"HK-01"`,
	} {
		if !strings.Contains(string(encoded), want) {
			t.Fatalf("预览 JSON 缺少兼容字段/详情结构 %q：%s", want, encoded)
		}
	}
	var hasAllExpanded bool
	for _, member := range result.ServiceGroups[0].ExpandedMemberDetails {
		if member.Name == "HK-01" && member.ID != "" && member.Kind == string(model.PolicyRefProxy) && member.Origin == "all_expanded" {
			hasAllExpanded = true
		}
	}
	if !hasAllExpanded {
		t.Fatalf("ExpandedMemberDetails 缺少 all_expanded 的 HK-01：%+v", result.ServiceGroups[0].ExpandedMemberDetails)
	}
	var hasLegacyAllExpanded bool
	for _, member := range result.ServiceGroups[0].ExpandedMembers {
		if member.Value == "HK-01" && member.Origin == "all_expanded" {
			hasLegacyAllExpanded = true
		}
	}
	if !hasLegacyAllExpanded {
		t.Fatalf("旧字段 ExpandedMembers 缺少展开后的节点名：%+v", result.ServiceGroups[0].ExpandedMembers)
	}
	if len(result.ServiceGroups[0].ExpandedMembers) != len(result.ServiceGroups[0].ExpandedMemberDetails) {
		t.Fatalf(
			"旧字段与详情中的展开成员数量不同：legacy=%+v details=%+v",
			result.ServiceGroups[0].ExpandedMembers,
			result.ServiceGroups[0].ExpandedMemberDetails,
		)
	}
	for index, member := range result.ServiceGroups[0].ExpandedMembers {
		if member.Value != result.ServiceGroups[0].ExpandedMemberDetails[index].Name ||
			member.Origin != result.ServiceGroups[0].ExpandedMemberDetails[index].Origin {
			t.Fatalf(
				"旧字段与详情中的第 %d 个展开成员不一致：legacy=%+v detail=%+v",
				index,
				member,
				result.ServiceGroups[0].ExpandedMemberDetails[index],
			)
		}
	}
}

// T-APP-008：从草稿生成不会改变运行时状态或 config_dirty
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
		t.Fatalf("操作前调用 Status：%v", err)
	}

	result, err := svc.GenerateFromDraft(context.Background(), generate.Request{Format: "clash", Filename: "draft.yaml"}, json.RawMessage(configJSON(draftURL, "", "SG")))
	if err != nil {
		t.Fatalf("GenerateFromDraft: %v", err)
	}
	if !strings.Contains(string(result.Body), "SG-01") || strings.Contains(string(result.Body), "HK-01") {
		t.Fatalf("草稿生成的正文应只使用草稿：%s", result.Body)
	}

	after, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("操作后调用 Status：%v", err)
	}
	if after.RuntimeConfigRevision != before.RuntimeConfigRevision || after.ConfigDirty != before.ConfigDirty || lastReloadTimeOf(after) != lastReloadTimeOf(before) {
		t.Fatalf("草稿生成改动了状态：before=%+v after=%+v", before, after)
	}
}

// T-APP-009：status 会重新计算本地配置文件的哈希；远程来源不会触发拉取
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
		t.Fatalf("本地配置调用 New：%v", err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(validConfigYAML, "strategy: select", "strategy: select # same size not required", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("本地配置调用 Status：%v", err)
	}
	if !status.ConfigDirty || status.ConfigRevision == status.RuntimeConfigRevision {
		t.Fatalf("本地配置的 status 应重新计算哈希并变为 dirty：%+v", status)
	}
	if status.Version != "2.0.0" || status.ConfigSource.Type != "local" || !status.Capabilities.ConfigWrite {
		t.Fatalf("本地配置的 status 元数据 = %+v", status)
	}

	remoteURL := "https://config.example.com/config.yaml"
	remoteFetcher := &mapFetcher{
		responses: map[string][]byte{remoteURL: []byte(validConfigYAML)},
		calls:     map[string]int{},
	}
	remoteSvc, err := New(context.Background(), Options{ConfigLocation: remoteURL, Fetcher: remoteFetcher})
	if err != nil {
		t.Fatalf("远程配置调用 New：%v", err)
	}
	if _, err := remoteSvc.Status(context.Background()); err != nil {
		t.Fatalf("远程配置调用 Status：%v", err)
	}
	if remoteFetcher.calls[remoteURL] != 1 {
		t.Fatalf("远程配置的 status 拉取了配置；调用次数 = %d，期望仅启动时一次", remoteFetcher.calls[remoteURL])
	}
}

// T-PRV-004：status 反映本地配置文件及其目录是否可写
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
		t.Fatalf("可写时调用 Status：%v", err)
	}
	if !status.ConfigSource.Writable || !status.Capabilities.ConfigWrite {
		t.Fatalf("可写的本地配置被报告为只读：%+v", status)
	}

	if err := os.Chmod(path, 0o400); err != nil {
		t.Fatal(err)
	}
	status, err = svc.Status(context.Background())
	if err != nil {
		t.Fatalf("文件只读时调用 Status：%v", err)
	}
	if status.ConfigSource.Writable || status.Capabilities.ConfigWrite {
		t.Fatalf("只读的本地文件被报告为可写：%+v", status)
	}

	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil { //nolint:gosec // 测试在去掉写权限时需要保留执行位。
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(dir, 0o700) }() //nolint:gosec // 恢复临时目录权限以便清理。
	status, err = svc.Status(context.Background())
	if err != nil {
		t.Fatalf("目录只读时调用 Status：%v", err)
	}
	if status.ConfigSource.Writable || status.Capabilities.ConfigWrite {
		t.Fatalf("只读的配置目录被报告为可写：%+v", status)
	}
}

// T-APP-010：生成链接使用服务端令牌和 base_url
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
		t.Fatalf("GenerateLink = %+v，期望带令牌的 %s", result, want)
	}
	publicLink, err := svc.GenerateLink(context.Background(), &GenerateLinkInput{Format: "surge", Filename: "phone.conf", IncludeToken: false})
	if err != nil {
		t.Fatalf("GenerateLink include_token=false: %v", err)
	}
	if publicLink.TokenIncluded || strings.Contains(publicLink.URL, "token=") {
		t.Fatalf("GenerateLink include_token=false = %+v，期望不带令牌", publicLink)
	}

	noTokenSvc, err := New(context.Background(), Options{ConfigLocation: path})
	if err != nil {
		t.Fatalf("无令牌时调用 New：%v", err)
	}
	noToken, err := noTokenSvc.GenerateLink(context.Background(), &GenerateLinkInput{Format: "clash", Filename: "clash.yaml", IncludeToken: true})
	if err != nil {
		t.Fatalf("无令牌时调用 GenerateLink：%v", err)
	}
	if noToken.TokenIncluded || strings.Contains(noToken.URL, "token=") {
		t.Fatalf("无服务端令牌时 GenerateLink = %+v，期望 token_included=false 且没有 token 查询参数", noToken)
	}

	noBasePath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(noBasePath, []byte(validConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	noBaseSvc, err := New(context.Background(), Options{ConfigLocation: noBasePath})
	if err != nil {
		t.Fatalf("无 base_url 时调用 New：%v", err)
	}
	_, err = noBaseSvc.GenerateLink(context.Background(), &GenerateLinkInput{Format: "surge", Filename: "phone.conf", IncludeToken: true})
	var badReq *BadRequestError
	if !errors.As(err, &badReq) || badReq.Code != "base_url_required" {
		t.Fatalf("无 base_url 时 GenerateLink 的错误 = %T %[1]v，期望 base_url_required", err)
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
		t.Fatalf("打开配置包：%v", err)
	}
	entries := make(map[string]archiveEntryForTest, len(zr.File))
	for _, file := range zr.File {
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("打开配置包条目 %q：%v", file.Name, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("读取配置包条目 %q：%v", file.Name, err)
		}
		entries[file.Name] = archiveEntryForTest{
			Body:     data,
			Modified: file.Modified,
		}
	}
	return entries
}

func buildDirectoryArchiveForTest(t *testing.T, name string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	header := &zip.FileHeader{Name: name}
	header.SetMode(os.ModeDir | 0o700)
	if _, err := zw.CreateHeader(header); err != nil {
		t.Fatalf("创建目录型配置包条目 %q：%v", name, err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("关闭目录型配置包：%v", err)
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

type blockingFetcher struct {
	mu        sync.Mutex
	responses map[string][]byte
	blockURL  string
	entered   chan struct{}
	release   chan struct{}
}

func (f *blockingFetcher) Fetch(ctx context.Context, rawURL string) ([]byte, error) {
	f.mu.Lock()
	shouldBlock := f.blockURL != "" && f.blockURL == rawURL
	entered := f.entered
	release := f.release
	f.mu.Unlock()

	if shouldBlock {
		entered <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	f.mu.Lock()
	body, ok := f.responses[rawURL]
	f.mu.Unlock()
	if !ok {
		return nil, errors.New("缺少响应")
	}
	return append([]byte(nil), body...), nil
}

// T-RLD-001：并发 Reload 时以 ErrReloadInProgress 拒绝重入
// T-RLD-008：进行中的 Reload 完成后，后续 Reload 能成功
func TestReloadConcurrentReturnsInProgress(t *testing.T) {
	rawURL := "https://config.example.com/config.yaml"
	fetcher := &blockingFetcher{
		responses: map[string][]byte{rawURL: []byte(validConfigYAML)},
		entered:   make(chan struct{}, 1),
		release:   make(chan struct{}),
	}
	svc, err := New(context.Background(), Options{ConfigLocation: rawURL, Fetcher: fetcher})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	fetcher.mu.Lock()
	fetcher.blockURL = rawURL
	fetcher.responses[rawURL] = []byte(strings.Replace(validConfigYAML, "strategy: select", "strategy: url-test", 1))
	fetcher.mu.Unlock()

	firstDone := make(chan error, 1)
	go func() {
		_, reloadErr := svc.Reload(context.Background())
		firstDone <- reloadErr
	}()

	select {
	case <-fetcher.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("等待第一个 Reload 进入拉取阶段超时")
	}

	if _, err := svc.Reload(context.Background()); !errors.Is(err, errtype.ErrReloadInProgress) {
		t.Fatalf("并发 Reload 的错误 = %v，期望 ErrReloadInProgress", err)
	}

	close(fetcher.release)
	select {
	case err := <-firstDone:
		if err != nil {
			t.Fatalf("第一个 Reload 失败：%v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("等待第一个 Reload 完成超时")
	}

	fetcher.mu.Lock()
	fetcher.blockURL = ""
	fetcher.mu.Unlock()

	if _, err := svc.Reload(context.Background()); err != nil {
		t.Fatalf("前一个完成后，后续 Reload 失败：%v", err)
	}
}

// T-RLD-002：进行中的 Generate 和 Preview 请求不阻塞 Reload，并基于旧快照完成
// T-RLD-005：慢速 Generate 仍在拉取上游订阅时，Reload 也能完成
// T-PRV-012：慢速 PreviewNodes / PreviewGroups 在拉取上游期间不持有 RLock
func TestReloadDoesNotBlockOnSlowGenerateAndPreview(t *testing.T) {
	oldSubURL := "https://sub.example.com/old"
	newSubURL := "https://sub.example.com/new"
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte(configYAML(oldSubURL, "", "HK")), 0o600); err != nil {
		t.Fatal(err)
	}

	fetcher := &blockingFetcher{
		responses: map[string][]byte{
			oldSubURL: subResponse("ss://YWVzLTI1Ni1jZmI6cGFzcw@hk.example.com:8388#HK-OLD"),
			newSubURL: subResponse("ss://YWVzLTI1Ni1jZmI6cGFzcw@sg.example.com:8388#SG-NEW"),
		},
		blockURL: oldSubURL,
		entered:  make(chan struct{}, 3),
		release:  make(chan struct{}),
	}

	svc, err := New(context.Background(), Options{ConfigLocation: configPath, Fetcher: fetcher})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	type genOutcome struct {
		res *GenerateResult
		err error
	}
	type nodesOutcome struct {
		res *NodePreviewResult
		err error
	}
	type groupsOutcome struct {
		res *GroupPreviewResult
		err error
	}

	genCh := make(chan genOutcome, 1)
	nodesCh := make(chan nodesOutcome, 1)
	groupsCh := make(chan groupsOutcome, 1)

	go func() {
		res, genErr := svc.Generate(context.Background(), generate.Request{Format: "clash", Filename: "clash.yaml"})
		genCh <- genOutcome{res: res, err: genErr}
	}()
	go func() {
		res, previewErr := svc.PreviewNodes(context.Background())
		nodesCh <- nodesOutcome{res: res, err: previewErr}
	}()
	go func() {
		res, previewErr := svc.PreviewGroups(context.Background())
		groupsCh <- groupsOutcome{res: res, err: previewErr}
	}()

	for i := range 3 {
		select {
		case <-fetcher.entered:
		case <-time.After(2 * time.Second):
			t.Fatalf("等待第 %d/3 个慢速请求进入拉取阶段超时", i+1)
		}
	}

	if err := os.WriteFile(configPath, []byte(configYAML(newSubURL, "", "SG")), 0o600); err != nil {
		t.Fatal(err)
	}

	reloadDone := make(chan error, 1)
	go func() {
		_, reloadErr := svc.Reload(context.Background())
		reloadDone <- reloadErr
	}()
	select {
	case err := <-reloadDone:
		if err != nil {
			t.Fatalf("慢速请求进行中时 Reload 失败：%v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Reload 被慢速 Generate / Preview 请求阻塞")
	}

	// Reload 之后的新请求立即看到新快照。
	newGen, err := svc.Generate(context.Background(), generate.Request{Format: "clash", Filename: "clash.yaml"})
	if err != nil {
		t.Fatalf("Reload 后新的 Generate：%v", err)
	}
	if !strings.Contains(string(newGen.Body), "SG-NEW") || strings.Contains(string(newGen.Body), "HK-OLD") {
		t.Fatalf("新的 Generate 没有使用热重载后的快照：%s", newGen.Body)
	}
	newNodes, err := svc.PreviewNodes(context.Background())
	if err != nil {
		t.Fatalf("Reload 后新的 PreviewNodes：%v", err)
	}
	if len(newNodes.Nodes) != 1 || newNodes.Nodes[0].Name != "SG-NEW" {
		t.Fatalf("新的 PreviewNodes = %+v，期望 SG-NEW", newNodes.Nodes)
	}
	newGroups, err := svc.PreviewGroups(context.Background())
	if err != nil {
		t.Fatalf("Reload 后新的 PreviewGroups：%v", err)
	}
	if len(newGroups.NodeGroups) != 1 || newGroups.NodeGroups[0].Name != "SG" {
		t.Fatalf("新的 PreviewGroups = %+v，期望 SG 组", newGroups.NodeGroups)
	}

	// 放行进行中的旧请求，确认它们基于旧快照完成。
	close(fetcher.release)

	oldGen := <-genCh
	if oldGen.err != nil {
		t.Fatalf("进行中的 Generate 失败：%v", oldGen.err)
	}
	if !strings.Contains(string(oldGen.res.Body), "HK-OLD") || strings.Contains(string(oldGen.res.Body), "SG-NEW") {
		t.Fatalf("进行中的 Generate 没有保持旧快照：%s", oldGen.res.Body)
	}

	oldNodes := <-nodesCh
	if oldNodes.err != nil {
		t.Fatalf("进行中的 PreviewNodes 失败：%v", oldNodes.err)
	}
	if len(oldNodes.res.Nodes) != 1 || oldNodes.res.Nodes[0].Name != "HK-OLD" {
		t.Fatalf("进行中的 PreviewNodes = %+v，期望 HK-OLD", oldNodes.res.Nodes)
	}

	oldGroups := <-groupsCh
	if oldGroups.err != nil {
		t.Fatalf("进行中的 PreviewGroups 失败：%v", oldGroups.err)
	}
	if len(oldGroups.res.NodeGroups) != 1 || oldGroups.res.NodeGroups[0].Name != "HK" {
		t.Fatalf("进行中的 PreviewGroups = %+v，期望 HK 组", oldGroups.res.NodeGroups)
	}
}
