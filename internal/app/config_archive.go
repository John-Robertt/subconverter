package app

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/fetch"
)

const (
	MaxConfigImportArchiveBytes = 10 * 1024 * 1024
	ConfigArchiveFilename       = "subconverter-config.zip"
)

const (
	archiveConfigEntry        = "config.yaml"
	archiveClashTemplateEntry = "templates/clash.yaml"
	archiveSurgeTemplateEntry = "templates/surge.conf"

	configImportsDirName = ".imports"
)

type ConfigArchiveResult struct {
	Filename    string
	ContentType string
	Body        []byte
}

func (s *Service) EffectiveConfigArchive(ctx context.Context) (*ConfigArchiveResult, error) {
	raw, templates, err := s.effectiveArchiveSnapshot()
	if err != nil {
		return nil, err
	}

	entries := []archiveEntry{{Name: archiveConfigEntry, Body: raw}}
	if templates.Clash != "" {
		body, err := fetch.LoadResource(ctx, templates.Clash, s.fetcher)
		if err != nil {
			return nil, err
		}
		entries = append(entries, archiveEntry{Name: archiveClashTemplateEntry, Body: body})
	}
	if templates.Surge != "" {
		body, err := fetch.LoadResource(ctx, templates.Surge, s.fetcher)
		if err != nil {
			return nil, err
		}
		entries = append(entries, archiveEntry{Name: archiveSurgeTemplateEntry, Body: body})
	}

	body, err := buildConfigArchive(entries, s.now())
	if err != nil {
		return nil, err
	}
	return &ConfigArchiveResult{
		Filename:    ConfigArchiveFilename,
		ContentType: "application/zip",
		Body:        body,
	}, nil
}

func (s *Service) ImportConfigArchive(_ context.Context, raw []byte) (*ImportConfigYAMLResult, error) {
	if len(raw) == 0 {
		return nil, newBadRequestError("invalid_archive", "缺少配置包")
	}
	if len(raw) > MaxConfigImportArchiveBytes {
		return nil, newBadRequestError("invalid_archive", "配置包不能超过 10 MiB")
	}
	if s.configLocation == "" || isRemoteConfig(s.configLocation) {
		return nil, errtype.ErrConfigSourceReadonly
	}
	if err := ensureLocalConfigWritable(s.configLocation); err != nil {
		return nil, err
	}

	entries, err := readConfigArchive(raw)
	if err != nil {
		return nil, err
	}
	configRaw, ok := entries[archiveConfigEntry]
	if !ok {
		return nil, newBadRequestError("invalid_archive", "配置包缺少 config.yaml")
	}
	if strings.TrimSpace(string(configRaw)) == "" {
		return nil, newBadRequestError("invalid_archive", "配置包中的 config.yaml 不能为空")
	}
	cfg, err := parseConfigYAML(configRaw)
	if err != nil {
		return nil, err
	}

	importedTemplates, err := s.writeImportedTemplates(entries)
	if err != nil {
		return nil, err
	}
	if importedTemplates.Clash != "" {
		cfg.Templates.Clash = importedTemplates.Clash
	}
	if importedTemplates.Surge != "" {
		cfg.Templates.Surge = importedTemplates.Surge
	}

	configJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	return &ImportConfigYAMLResult{Config: configJSON}, nil
}

func (s *Service) effectiveArchiveSnapshot() ([]byte, config.Templates, error) {
	s.mu.RLock()
	raw := append([]byte(nil), s.effectiveConfigYAML...)
	var templates config.Templates
	if s.runtimeCfg != nil {
		templates = s.runtimeCfg.Templates()
	}
	s.mu.RUnlock()
	if len(raw) == 0 {
		return nil, config.Templates{}, newBadRequestError("effective_config_unavailable", "当前生效配置不可用")
	}
	return raw, templates, nil
}

func (s *Service) writeImportedTemplates(entries map[string][]byte) (config.Templates, error) {
	templateEntries := make([]archiveEntry, 0, 2)
	if body, ok := entries[archiveClashTemplateEntry]; ok {
		templateEntries = append(templateEntries, archiveEntry{Name: "clash.yaml", Body: body})
	}
	if body, ok := entries[archiveSurgeTemplateEntry]; ok {
		templateEntries = append(templateEntries, archiveEntry{Name: "surge.conf", Body: body})
	}
	if len(templateEntries) == 0 {
		return config.Templates{}, nil
	}

	configDir, err := filepath.Abs(filepath.Dir(filepath.Clean(s.configLocation)))
	if err != nil {
		return config.Templates{}, fmt.Errorf("%w: %v", errtype.ErrTemplateFileNotWritable, err)
	}
	importsDir := filepath.Join(configDir, configImportsDirName)
	if err := os.MkdirAll(importsDir, 0o700); err != nil {
		return config.Templates{}, fmt.Errorf("%w: %v", errtype.ErrTemplateFileNotWritable, err)
	}
	stagingDir, err := os.MkdirTemp(importsDir, ".staging-*")
	if err != nil {
		return config.Templates{}, fmt.Errorf("%w: %v", errtype.ErrTemplateFileNotWritable, err)
	}
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(stagingDir)
		}
	}()

	stagingTemplateDir := filepath.Join(stagingDir, "templates")
	if err := os.Mkdir(stagingTemplateDir, 0o700); err != nil {
		return config.Templates{}, fmt.Errorf("%w: %v", errtype.ErrTemplateFileNotWritable, err)
	}
	for _, entry := range templateEntries {
		if err := writeImportedTemplateFile(filepath.Join(stagingTemplateDir, entry.Name), entry.Body); err != nil {
			return config.Templates{}, err
		}
	}

	importID := strings.TrimPrefix(filepath.Base(stagingDir), ".staging-")
	finalDir := filepath.Join(importsDir, "import-"+importID)
	if err := os.Rename(stagingDir, finalDir); err != nil {
		return config.Templates{}, fmt.Errorf("%w: %v", errtype.ErrTemplateFileNotWritable, err)
	}
	published = true
	syncDirBestEffort(importsDir)

	imported := config.Templates{}
	if _, ok := entries[archiveClashTemplateEntry]; ok {
		imported.Clash = filepath.Join(finalDir, "templates", "clash.yaml")
	}
	if _, ok := entries[archiveSurgeTemplateEntry]; ok {
		imported.Surge = filepath.Join(finalDir, "templates", "surge.conf")
	}
	return imported, nil
}

func writeImportedTemplateFile(path string, body []byte) error {
	if err := os.WriteFile(path, body, 0o600); err != nil { // #nosec G304 -- path is under a service-managed import staging directory.
		return fmt.Errorf("%w: %v", errtype.ErrTemplateFileNotWritable, err)
	}
	syncDirBestEffort(filepath.Dir(path))
	return nil
}

type archiveEntry struct {
	Name string
	Body []byte
}

func buildConfigArchive(entries []archiveEntry, modifiedAt time.Time) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	modifiedAt = modifiedAt.UTC()
	for _, entry := range entries {
		if err := writeArchiveEntry(zw, entry, modifiedAt); err != nil {
			_ = zw.Close()
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("finalize config archive: %w", err)
	}
	return buf.Bytes(), nil
}

func writeArchiveEntry(zw *zip.Writer, entry archiveEntry, modifiedAt time.Time) error {
	header := &zip.FileHeader{
		Name:     entry.Name,
		Method:   zip.Deflate,
		Modified: modifiedAt,
	}
	header.SetMode(0o600)
	w, err := zw.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create config archive entry %q: %w", entry.Name, err)
	}
	if _, err := w.Write(entry.Body); err != nil {
		return fmt.Errorf("write config archive entry %q: %w", entry.Name, err)
	}
	return nil
}

func readConfigArchive(raw []byte) (map[string][]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, newBadRequestError("invalid_archive", "配置包不是有效 ZIP 文件")
	}
	want := map[string]bool{
		archiveConfigEntry:        true,
		archiveClashTemplateEntry: true,
		archiveSurgeTemplateEntry: true,
	}
	entries := make(map[string][]byte, len(want))
	remaining := int64(MaxConfigImportArchiveBytes)
	for _, file := range zr.File {
		if !want[file.Name] {
			continue
		}
		if _, seen := entries[file.Name]; seen {
			return nil, newBadRequestError("invalid_archive", "配置包包含重复文件："+file.Name)
		}
		if file.FileInfo().IsDir() {
			return nil, newBadRequestError("invalid_archive", "配置包文件不能是目录："+file.Name)
		}
		body, err := readArchiveFile(file, &remaining)
		if err != nil {
			return nil, err
		}
		entries[file.Name] = body
	}
	return entries, nil
}

func readArchiveFile(file *zip.File, remaining *int64) ([]byte, error) {
	if *remaining <= 0 {
		return nil, newBadRequestError("invalid_archive", "配置包解压后不能超过 10 MiB")
	}
	rc, err := file.Open()
	if err != nil {
		return nil, newBadRequestError("invalid_archive", "读取配置包文件失败："+file.Name)
	}
	defer func() { _ = rc.Close() }()

	limited := &io.LimitedReader{R: rc, N: *remaining + 1}
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, newBadRequestError("invalid_archive", "读取配置包文件失败："+file.Name)
	}
	if int64(len(body)) > *remaining {
		return nil, newBadRequestError("invalid_archive", "配置包解压后不能超过 10 MiB")
	}
	*remaining -= int64(len(body))
	return body, nil
}
