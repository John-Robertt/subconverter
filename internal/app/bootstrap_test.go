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
		t.Fatalf("缺少本地配置时应进入初始化模式：%v", err)
	}
	return svc
}

// T-APP-BOOT-001：空部署可以导入任一种包类型，创建配置并显式加载。
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
				t.Fatalf("初始化状态不正确：%+v", status)
			}
			snapshot, err := svc.ConfigSnapshot(ctx)
			if err != nil || snapshot.ConfigRevision != status.ConfigRevision {
				t.Fatalf("空草稿/revision 不正确：snapshot=%+v err=%v", snapshot, err)
			}
			if _, err := svc.SaveConfig(ctx, &SaveConfigInput{ConfigRevision: snapshot.ConfigRevision, Config: snapshot.Config}); err == nil {
				t.Fatal("无效的空草稿被保存了")
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
				t.Fatalf("导入时创建了主配置：%v", err)
			}
			if _, err := os.Stat(svc.dataDir); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("导入时发布了正文文件：%v", err)
			}
			if _, err := svc.PreviewNodesFromDraft(ctx, imported.Config); err != nil {
				t.Fatalf("草稿预览不应依赖已生效的配置：%v", err)
			}
			saved, err := svc.SaveConfig(ctx, &SaveConfigInput{ConfigRevision: snapshot.ConfigRevision, Config: imported.Config})
			if err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(svc.configLocation)
			if err != nil || info.Mode().Perm() != 0o600 {
				t.Fatalf("新建主配置的权限不正确：info=%v err=%v", info, err)
			}
			status, err = svc.Status(ctx)
			if err != nil || !status.ConfigSource.Exists || !status.Capabilities.Reload || !status.ConfigDirty || status.RuntimeConfigRevision != "" || saved.ConfigRevision == missingConfigRevision {
				t.Fatalf("保存后应等待热重载才生效：status=%+v saved=%+v err=%v", status, saved, err)
			}
			if _, err := svc.SavedConfigArchive(ctx); err != nil {
				t.Fatalf("热重载前导出已保存配置失败：%v", err)
			}
			if _, err := svc.EffectiveConfigYAML(ctx); err == nil {
				t.Fatal("保存在未热重载时就让配置生效了")
			}
			if _, err := svc.Reload(ctx); err != nil {
				t.Fatal(err)
			}
			status, err = svc.Status(ctx)
			if err != nil || status.ConfigDirty || status.RuntimeConfigRevision != saved.ConfigRevision || status.ConfigLoadedAt == "" {
				t.Fatalf("加载后的状态不正确：status=%+v err=%v", status, err)
			}
			for _, format := range []string{"clash", "surge", "loon"} {
				output, err := svc.Generate(ctx, GenerateInput{Format: format, Filename: "config"})
				if err != nil {
					t.Fatalf("%s 生成失败：%v", format, err)
				}
				if !strings.Contains(string(output.Body), "HK_BOOT") {
					t.Fatalf("%s 丢失了导入的节点", format)
				}
			}
			if _, err := New(ctx, Options{ConfigLocation: svc.configLocation, DataDir: svc.dataDir}); err != nil {
				t.Fatalf("首次保存后重启失败：%v", err)
			}
		})
	}
}

// T-APP-BOOT-002：依赖运行时配置的操作明确失败，编辑和草稿检查仍然可用。
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
				t.Fatalf("实际=%v，期望 config_not_initialized", err)
			}
		})
	}
}

// T-APP-BOOT-003：首次发布不能覆盖外部创建的文件，并回滚新建的正文版本。
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
				t.Fatalf("实际=%v，期望首次创建时的 revision 冲突", err)
			}
			actual, err := os.ReadFile(path) // #nosec G304 -- fixture 路径位于 t.TempDir 内。
			if err != nil || string(actual) != string(external) {
				t.Fatalf("外部文件被改动：body=%q err=%v", actual, err)
			}
			entries, err := os.ReadDir(svc.dataDir)
			if err != nil && !errors.Is(err, os.ErrNotExist) || len(entries) != 0 {
				t.Fatalf("保存失败后仍残留正文版本：entries=%v err=%v", entries, err)
			}
		})
	}
}

// T-APP-BOOT-004：目录权限不足或已有无效文件时，不会被当作可写的初始化默认状态。
func TestBootstrap_PermissionsAndInvalidFiles(t *testing.T) {
	t.Run("broken existing symlink", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		if err := os.Symlink(filepath.Join(dir, "missing-target.yaml"), path); err != nil {
			t.Fatal(err)
		}
		if _, err := New(context.Background(), Options{ConfigLocation: path}); err == nil {
			t.Fatal("已存在但无法读取的符号链接被静默当作初始化模式")
		}
	})
	t.Run("readonly directory", func(t *testing.T) {
		svc := bootstrapService(t)
		dir := filepath.Dir(svc.configLocation)
		if err := os.Chmod(dir, 0o500); err != nil { // #nosec G302 -- 保留所有者的读取和检索权限，以便检查缺失的配置。
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) // #nosec G302 -- 恢复临时目录的访问权限以便清理。
		status, err := svc.Status(context.Background())
		if err != nil || status.ConfigSource.Exists || status.ConfigSource.Writable || status.Capabilities.ConfigWrite {
			t.Fatalf("只读目录下的初始化状态不正确：status=%+v err=%v", status, err)
		}
		snapshot, err := svc.ConfigSnapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		_, err = svc.SaveConfig(context.Background(), &SaveConfigInput{ConfigRevision: snapshot.ConfigRevision, Config: json.RawMessage(`{}`)})
		if !errors.Is(err, errtype.ErrConfigFileNotWritable) {
			t.Fatalf("只读目录下首次保存：%v", err)
		}
	})
	for _, body := range []string{"", "sources: [", "{}"} {
		t.Run("invalid "+body, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := New(context.Background(), Options{ConfigLocation: path}); err == nil {
				t.Fatal("已存在的无效文件被静默当作初始化模式")
			}
		})
	}
}

// T-APP-BOOT-005：两个独立的服务实例不能都发布首个主配置。
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
			t.Fatalf("创建时出现意外失败：%v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("成功创建者=%d，冲突创建者=%d，期望 1/1", successes, conflicts)
	}
	entries, err := os.ReadDir(first.dataDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("冲突失败的创建者残留了正文版本：entries=%v err=%v", entries, err)
	}
}
