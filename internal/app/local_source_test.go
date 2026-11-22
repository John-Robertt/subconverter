package app

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/config"
)

// T-APP-LOCAL-001：节点池在保存、重启和两种配置包导出后，仍保留可编辑的名称和正文。
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
			t.Fatalf("%s 没有保存带名称的文件引用：%+v", list.Kind, source)
		}
	}
	if _, err := saveContentDraft(t, svc, cfg, saved.ConfigRevision); err != nil {
		t.Fatal(err)
	}
	unchanged := storedContentConfig(t, svc)
	if unchanged.Sources.Snell[0] != stored.Sources.Snell[0] || unchanged.Sources.VLess[0] != stored.Sources.VLess[0] {
		t.Fatalf("内容未变的保存发布了新的节点池版本：%+v", unchanged.Sources)
	}
	restarted, err := New(ctx, Options{ConfigLocation: svc.configLocation, DataDir: svc.dataDir})
	if err != nil {
		t.Fatal(err)
	}
	editable, revision := editableContentConfig(t, restarted)
	if editable.Sources.Snell[0] != cfg.Sources.Snell[0] || editable.Sources.VLess[0] != cfg.Sources.VLess[0] {
		t.Fatalf("重启后丢失了带名称的可编辑节点池：%+v", editable.Sources)
	}
	for _, format := range []string{"clash", "surge", "loon"} {
		request, err := NewGenerateInput(format, "", false)
		if err != nil {
			t.Fatal(err)
		}
		output, err := restarted.Generate(ctx, request)
		if err != nil {
			t.Fatalf("%s 生成失败：%v", format, err)
		}
		body := string(output.Body)
		if strings.Contains(body, "HK-SN") != (format == "surge") || strings.Contains(body, "HK-VL") != (format != "surge") || !strings.Contains(body, "HK-SAVED") {
			t.Fatalf("%s 的协议过滤结果发生了变化：%s", format, body)
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
				t.Fatal("配置包丢失了节点池文件或改动了其内容")
			}
			packaged, err := parseConfigYAML(entries[archiveConfigEntry])
			if err != nil {
				t.Fatal(err)
			}
			if packaged.Sources.Snell[0].ContentFile != "snell/1.txt" || packaged.Sources.VLess[0].ContentFile != "vless/1.txt" {
				t.Fatalf("配置包丢失了节点池的相对引用：%+v", packaged.Sources)
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
				t.Fatalf("配置包往返后带名称的节点池被改动：%+v", imported.Sources)
			}
		})
	}
	// 更新节点池会发布新版本，但不改变当前生效的快照及其文件。
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
		t.Fatalf("保存在热重载前就改动了生效中的节点池：%v", err)
	}
	previous, err := os.ReadFile(stored.Sources.Snell[0].ContentFile) // #nosec G304 -- fixture 路径是 t.TempDir 内的托管版本文件。
	if err != nil || string(previous) != oldSnell {
		t.Fatalf("旧的节点池版本被改动：body=%q err=%v", previous, err)
	}
	if _, err := restarted.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	output, err = restarted.Generate(ctx, request)
	if err != nil || !strings.Contains(string(output.Body), "HK-SN-NEW") {
		t.Fatalf("热重载没有让新的节点池生效：%v", err)
	}
	revision = changed.ConfigRevision
	// 节点池文件缺失时，仍可在编辑器中修复，并阻止导出不完整的配置包。
	if err := os.Remove(stored.Sources.VLess[0].ContentFile); err != nil {
		t.Fatal(err)
	}
	broken, _ := editableContentConfig(t, restarted)
	if broken.Sources.VLess[0].ContentFile != stored.Sources.VLess[0].ContentFile || broken.Sources.Snell[0].Content != cfg.Sources.Snell[0].Content {
		t.Fatalf("VLESS 正文缺失破坏了编辑器中的其他输入：%+v", broken.Sources)
	}
	if _, err := restarted.SavedConfigArchive(ctx); err == nil {
		t.Fatal("配置包静默遗漏了缺失的 VLESS 文件")
	}
	if _, err := saveContentDraft(t, restarted, cfg, revision); err != nil {
		t.Fatalf("修复缺失的节点池文件失败：%v", err)
	}
}
