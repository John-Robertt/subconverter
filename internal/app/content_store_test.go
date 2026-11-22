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

// T-APP-STORE-001：正文直接存放在配置的目录中，重启后仍可编辑。
func TestContentStore_SaveAndRestart(t *testing.T) {
	svc, cfg, revision := contentStoreFixture(t)
	result, err := saveContentDraft(t, svc, cfg, revision)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(svc.configLocation) // #nosec G304 -- fixture 位置在 t.TempDir 内。
	if err != nil {
		t.Fatal(err)
	}
	var stored config.Config
	if err := yaml.Unmarshal(raw, &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Sources.Subscriptions[0].Content != "" || len(stored.Templates.Inline) != 0 || strings.Contains(string(raw), "secret-value") {
		t.Fatal("配置中存放的是正文而不是引用")
	}
	if stored.Sources.Subscriptions[0].Name != cfg.Sources.Subscriptions[0].Name {
		t.Fatalf("保存的名称=%q，期望 %q", stored.Sources.Subscriptions[0].Name, cfg.Sources.Subscriptions[0].Name)
	}
	paths := []string{stored.Sources.Subscriptions[0].ContentFile, stored.Templates.Clash, stored.Templates.Surge, stored.Templates.Loon}
	for _, path := range paths {
		if !strings.Contains(filepath.Base(path), "20261101T123456.123456789Z-") {
			t.Fatalf("文件名缺少时间戳：%q", path)
		}
		if filepath.Dir(path) != svc.dataDir {
			t.Fatalf("正文存放在固定目录之外：%q", path)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("正文文件权限=%o，期望 600", info.Mode().Perm())
		}
	}
	entries, err := os.ReadDir(svc.dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("期望 4 个正文文件且没有暂存文件，实际 %d", len(entries))
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
		t.Fatal("重启后丢失了可编辑的正文")
	}
	if snapshot.ConfigRevision != result.ConfigRevision {
		t.Fatal("快照的 revision 与磁盘上的配置不一致")
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
			t.Fatalf("%s 丢失了已保存的节点", format)
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
		t.Fatalf("内容未变的保存创建了多余的版本：files=%d，期望 4", len(after))
	}
}

// T-APP-STORE-002：目录出错时保留已保存的配置，修复权限后可以重试。
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
					_ = os.Chmod(svc.dataDir, 0o700) // #nosec G302 -- 临时目录需要所有者的执行权限。
				})
			default:
				if err := os.MkdirAll(svc.dataDir, 0o500); err != nil {
					t.Fatal(err)
				}
				parent := svc.dataDir
				svc.dataDir = filepath.Join(parent, "nested")
				t.Cleanup(func() {
					_ = os.Chmod(parent, 0o700) // #nosec G302 -- 临时目录需要所有者的执行权限。
				})
			}
			_, err := saveContentDraft(t, svc, cfg, revision)
			var writeErr *errtype.ResourceError
			if !errors.As(err, &writeErr) || writeErr.Code != errtype.CodeResourceContentWriteFailed || !strings.Contains(writeErr.Message, svc.dataDir) || strings.Contains(writeErr.Message, "secret-value") {
				t.Fatalf("期望可据以处理且已脱敏的写入错误，实际 %v", err)
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
				t.Fatal("正文保存失败却改动了配置或只读约定")
			}
			switch mode {
			case "file":
				if err := os.Remove(svc.dataDir); err != nil {
					t.Fatal(err)
				}
			case "readonly":
				if err := os.Chmod(svc.dataDir, 0o700); err != nil { // #nosec G302 -- 恢复所有者对临时目录的访问权限。
					t.Fatal(err)
				}
			default:
				if err := os.Chmod(filepath.Dir(svc.dataDir), 0o700); err != nil { // #nosec G302 -- 恢复所有者对临时目录的访问权限。
					t.Fatal(err)
				}
			}
			if _, err := saveContentDraft(t, svc, cfg, revision); err != nil {
				t.Fatalf("重试失败：%v", err)
			}
		})
	}
}

// T-APP-STORE-003：热重载前修改正文不会改变已有的运行时快照。
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
		t.Fatalf("成功=%d，冲突=%d，期望 1/3", success, conflicts)
	}
	body, err := os.ReadFile(old) // #nosec G304 -- 路径在测试的托管目录内生成。
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "HK-SAVED") {
		t.Fatal("旧快照的正文被覆盖了")
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
		t.Fatal("保存在热重载前就改动了生效中的运行时配置")
	}
	if _, err := svc.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	output, err = svc.Generate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output.Body), "HK-NEW") {
		t.Fatal("热重载没有让新正文生效")
	}
}

// T-APP-STORE-004：编辑只读来源时，只在托管目录中写入新版本。
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
	raw, err := os.ReadFile(original) // #nosec G304 -- original 位于 t.TempDir 内。
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != body {
		t.Fatal("只读的原始文件被覆盖了")
	}
}

// T-APP-STORE-005：生产环境的默认目录下没有额外的正文子目录。
func TestContentStore_DefaultDirectory(t *testing.T) {
	if got := (&Service{}).contentDirectory(); got != "/config" {
		t.Fatalf("默认目录=%q，期望 /config", got)
	}
}

// T-APP-STORE-009：复制按后缀识别格式的文档后，保存和重启都保留其格式。
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
				t.Fatalf("副本丢失了 .lcf 格式：%+v", editable.Sources.Subscriptions[0])
			}
			if _, err := svc.SaveConfig(context.Background(), &SaveConfigInput{ConfigRevision: snapshot.ConfigRevision, Config: snapshot.Config}); err != nil {
				t.Fatal(err)
			}
			savedRaw, err := os.ReadFile(svc.configLocation) // #nosec G304 -- fixture 配置位于 t.TempDir 内。
			if err != nil {
				t.Fatal(err)
			}
			var stored config.Config
			if err := yaml.Unmarshal(savedRaw, &stored); err != nil {
				t.Fatal(err)
			}
			if path := stored.Sources.Subscriptions[0].ContentFile; filepath.Ext(path) != ".txt" {
				t.Fatalf("Loon 来源文件名=%q，期望 .txt", path)
			}
			if filepath.Ext(stored.Templates.Loon) != ".lcf" {
				t.Fatalf("Loon 模板文件名=%q，期望 .lcf", stored.Templates.Loon)
			}
			if stored.Sources.Subscriptions[0].Format != "loon" {
				t.Fatalf("已保存的来源丢失了显式格式：%+v", stored.Sources.Subscriptions[0])
			}
			original, err := os.ReadFile(location) // #nosec G304 -- fixture 来源位于 t.TempDir 内。
			if err != nil || string(original) != body {
				t.Fatalf("保存改动了原始来源：body=%q err=%v", original, err)
			}
			savedBody, err := os.ReadFile(stored.Sources.Subscriptions[0].ContentFile) // #nosec G304 -- 服务托管的正文位于 t.TempDir 内。
			if err != nil || string(savedBody) != body {
				t.Fatalf("已保存的 Loon 正文 = %q err=%v，期望 %q", savedBody, err, body)
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
				t.Fatal("复制/保存/重启后 Loon 节点消失了")
			}
			edited, revision := editableContentConfig(t, restarted)
			if _, err := saveContentDraft(t, restarted, &edited, revision); err != nil {
				t.Fatal(err)
			}
			after := storedContentConfig(t, restarted)
			if after.Sources.Subscriptions[0] != stored.Sources.Subscriptions[0] || after.Templates.Loon != stored.Templates.Loon {
				t.Fatalf("内容未变的保存替换了来源或模板：before=%+v after=%+v", stored, after)
			}
		})
	}
}

// T-APP-STORE-008：某个正文缺失不妨碍修复配置的其余部分。
func TestContentStore_MissingBodyRemainsEditable(t *testing.T) {
	svc, cfg, revision := contentStoreFixture(t)
	if _, err := saveContentDraft(t, svc, cfg, revision); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(svc.configLocation) // #nosec G304 -- fixture 配置位于 t.TempDir 内。
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
		t.Fatalf("正文缺失导致配置编辑器无法使用：%v", err)
	}
	var got config.Config
	if err := json.Unmarshal(snapshot.Config, &got); err != nil {
		t.Fatal(err)
	}
	if got.Sources.Subscriptions[0].ContentFile != stored.Sources.Subscriptions[0].ContentFile || got.Templates.Loon != stored.Templates.Loon {
		t.Fatal("无法读取的正文引用丢失了")
	}
	if got.Templates.Inline["clash"] != cfg.Templates.Inline["clash"] {
		t.Fatal("一个正文读取失败导致其他可读正文无法恢复")
	}
	if _, err := svc.ImportConfigYAML(context.Background(), raw); err != nil {
		t.Fatalf("本地正文缺失导致草稿无法导入：%v", err)
	}
	if _, err := svc.EffectiveConfigArchive(context.Background()); err == nil {
		t.Fatal("配置包导出了不完整的正文")
	}
}

// T-APP-STORE-007：普通模板文件名不会让实时引用变成快照。
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
					t.Fatal("托管正文没有被恢复")
				}
			} else if got.Templates.Loon != location || len(got.Templates.Inline) != 0 {
				t.Fatalf("普通模板引用被改动：%+v", got.Templates)
			}
		})
	}
}

// T-APP-STORE-006：配置提交失败时，只删除这次保存创建的版本。
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
		t.Fatalf("期望配置提交失败，实际 %v", err)
	}
	svc.configLocation = original
	svc.now = now
	entries, err := os.ReadDir(svc.dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("仍残留未提交的版本：%d", len(entries))
	}
	snapshot, err := svc.ConfigSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ConfigRevision != revision {
		t.Fatal("配置提交失败却改变了 revision")
	}
}

// T-APP-STORE-010：后面的正文无法写入时，SaveConfig 同样回滚已写的文件。
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
		t.Fatalf("期望后续正文写入失败，实际 %v", err)
	}
	svc.dataDir, svc.now = root, clock
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("部分保存残留了 %d 个未提交的文件", len(entries))
	}
	snapshot, err := svc.ConfigSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ConfigRevision != revision {
		t.Fatal("部分正文保存改动了配置")
	}
}

func storedContentConfig(t *testing.T, svc *Service) config.Config {
	t.Helper()
	raw, err := os.ReadFile(svc.configLocation) // #nosec G304 -- fixture 配置位于 t.TempDir 下。
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

// T-APP-STORE-011：只有改动过的正文才创建版本；只改元数据或内容相同的保存都复用引用。
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
				if err := os.Chmod(svc.dataDir, 0o500); err != nil { // #nosec G302 -- 没有写权限时，需要目录的读取和执行权限才能复用已有版本。
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(svc.dataDir, 0o700) }) // #nosec G302 -- 恢复所有者对临时目录的访问权限。
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
					t.Fatalf("%s 引用 changed=%v，期望 %v：old=%q new=%q", kind, changed, kind == tt.changed, old, newPaths[kind])
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
				t.Fatalf("正文文件数=%d，期望 %d", len(entries), wantFiles)
			}
			if tt.patch == nil && !tt.missingSource {
				afterInfo, err := os.Stat(svc.configLocation)
				if err != nil {
					t.Fatal(err)
				}
				if result.ConfigRevision != savedRevision || !os.SameFile(info, afterInfo) {
					t.Fatalf("内容相同的保存重写了配置：revision=%s，期望 %s", result.ConfigRevision, savedRevision)
				}
			}
		})
	}
}

// T-APP-STORE-012：调整订阅顺序时复用各自的正文；新增的重复项仍是独立资源。
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
			t.Fatalf("调整顺序后来源 %d 丢失了引用：%+v", i, after.Sources.Subscriptions)
		}
	}
	entries, err := os.ReadDir(svc.dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 5 {
		t.Fatalf("调整顺序创建了新版本：files=%d，期望 5", len(entries))
	}
	edited, revision = editableContentConfig(t, svc)
	edited.Sources.Subscriptions = append(edited.Sources.Subscriptions, edited.Sources.Subscriptions[0])
	if _, err := saveContentDraft(t, svc, &edited, revision); err != nil {
		t.Fatal(err)
	}
	after = storedContentConfig(t, svc)
	if len(after.Sources.Subscriptions) != 3 || after.Sources.Subscriptions[0].ContentFile == after.Sources.Subscriptions[2].ContentFile {
		t.Fatalf("新订阅没有独立的资源：%+v", after.Sources.Subscriptions)
	}
}

// T-APP-STORE-013：显式编辑普通引用时，即使文本相同也会创建托管副本。
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
		t.Fatalf("普通引用被复用而没有复制：%q", after.Templates.Loon)
	}
}

// T-APP-STORE-014：配置提交失败时只删除新正文，被复用的版本保持可读。
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
		t.Fatalf("保存错误=%v，期望配置提交失败", err)
	}
	after := storedContentConfig(t, svc)
	if after.Templates.Loon != before.Templates.Loon {
		t.Fatal("保存失败却改动了配置引用")
	}
	entries, err := os.ReadDir(svc.dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("保存失败后残留了新版本或删除了旧版本：%d", len(entries))
	}
	if _, err := svc.ReadResource(context.Background(), before.Templates.Clash); err != nil {
		t.Fatalf("被复用的文件被删除了：%v", err)
	}
}

// T-APP-STORE-015：已保存的 YAML 格式错误时，仍能用有效草稿替换。
func TestContentStore_RepairInvalidSavedConfig(t *testing.T) {
	svc, cfg, _ := contentStoreFixture(t)
	raw := []byte("sources: [invalid yaml\n")
	if err := os.WriteFile(svc.configLocation, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := saveContentDraft(t, svc, cfg, configRevision(raw)); err != nil {
		t.Fatalf("用有效草稿替换失败：%v", err)
	}
	if stored := storedContentConfig(t, svc); stored.Sources.Subscriptions[0].ContentFile == "" {
		t.Fatal("替换时没有保存正文")
	}
}

// T-APP-STORE-016：保留一个显式引用时，不能占用另一个订阅可复用的版本。
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
			t.Fatalf("来源 %d 的引用被改动：old=%q new=%q", i, before.Sources.Subscriptions[i].ContentFile, after.Sources.Subscriptions[i].ContentFile)
		}
	}
}
