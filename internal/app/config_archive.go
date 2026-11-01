package app

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/fetch"
	"gopkg.in/yaml.v3"
)

const (
	MaxConfigImportArchiveBytes = 10 * 1024 * 1024
	ConfigArchiveFilename       = "subconverter-config.zip"
)

const (
	archiveConfigEntry        = "config.yaml"
	archiveClashTemplateEntry = "templates/clash.yaml"
	archiveSurgeTemplateEntry = "templates/surge.conf"
	archiveLoonTemplateEntry  = "templates/loon.lcf"
)

type ConfigArchiveResult struct {
	Filename    string
	ContentType string
	Body        []byte
}

func (s *Service) EffectiveConfigArchive(ctx context.Context) (*ConfigArchiveResult, error) {
	raw, err := s.EffectiveConfigYAML(ctx)
	if err != nil {
		return nil, err
	}
	return s.configArchive(ctx, raw)
}

func (s *Service) SavedConfigArchive(ctx context.Context) (*ConfigArchiveResult, error) {
	raw, err := s.readConfigBytes(ctx, false)
	if err != nil {
		return nil, err
	}
	return s.configArchive(ctx, raw)
}

func (s *Service) configArchive(ctx context.Context, raw []byte) (*ConfigArchiveResult, error) {
	cfg, err := parseConfigYAML(raw)
	if err != nil {
		return nil, err
	}
	if err := s.restoreEditableContent(ctx, cfg); err != nil {
		return nil, err
	}
	entries := []archiveEntry{}
	for _, entry := range archiveTemplates(cfg) {
		var body []byte
		if content, ok := cfg.Templates.Inline[entry.format]; ok {
			body = []byte(content)
		} else if *entry.location != "" {
			body, err = fetch.LoadResource(ctx, *entry.location, s.fetcher)
			if err != nil {
				return nil, err
			}
		} else {
			continue
		}
		entries = append(entries, archiveEntry{Name: entry.name, Body: body})
		*entry.location = entry.name
		delete(cfg.Templates.Inline, entry.format)
	}
	raw, err = yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	entries = append([]archiveEntry{{Name: archiveConfigEntry, Body: raw}}, entries...)

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

	if cfg.Templates.Inline == nil {
		cfg.Templates.Inline = make(map[string]string)
	}
	for _, entry := range archiveTemplates(cfg) {
		if body, ok := entries[entry.name]; ok {
			cfg.Templates.Inline[entry.format] = string(body)
			*entry.location = ""
		}
	}

	configJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	return &ImportConfigYAMLResult{Config: configJSON}, nil
}

type archiveTemplate struct {
	format, name string
	location     *string
}

func archiveTemplates(cfg *config.Config) []archiveTemplate {
	return []archiveTemplate{
		{"clash", archiveClashTemplateEntry, &cfg.Templates.Clash},
		{"surge", archiveSurgeTemplateEntry, &cfg.Templates.Surge},
		{"loon", archiveLoonTemplateEntry, &cfg.Templates.Loon},
	}
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
		archiveLoonTemplateEntry:  true,
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
