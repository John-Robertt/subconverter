package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/fetch"
	"github.com/John-Robertt/subconverter/internal/pipeline"
)

const DefaultDataDir = "/config"

const contentVersionTimeLayout = "20060102T150405.000000000Z"

func (s *Service) contentDirectory() string {
	root := s.dataDir
	if root == "" {
		root = DefaultDataDir
	}
	return filepath.Clean(root)
}

func (s *Service) contentWriteError(cause error) error {
	dir := s.contentDirectory()
	return &errtype.ResourceError{
		Code: errtype.CodeResourceContentWriteFailed, Location: dir, Cause: cause,
		Message: fmt.Sprintf("正文保存失败：目录 %q 无法写入。请检查目录权限、容器挂载和运行用户；当前草稿已保留。原因：%v", dir, cause),
	}
}

// persistEditableContent publishes timestamped versions before the config references them.
// Old files remain readable by the active snapshot and in-flight requests.
func (s *Service) persistEditableContent(cfg *config.Config) ([]string, error) {
	var created []string
	ready := false
	store := func(kind, ext, content string) (string, error) {
		if !ready {
			if err := s.ensureContentDirectory(); err != nil {
				return "", err
			}
			ready = true
		}
		stamp := s.now().UTC().Format(contentVersionTimeLayout)
		tmp, err := os.CreateTemp(s.contentDirectory(), ".pending-"+kind+"-"+stamp+"-*"+ext)
		if err != nil {
			return "", s.contentWriteError(err)
		}
		defer func() { _ = os.Remove(tmp.Name()) }()
		if _, err = tmp.WriteString(content); err != nil {
			_ = tmp.Close()
			return "", s.contentWriteError(err)
		}
		if err = tmp.Sync(); err != nil {
			_ = tmp.Close()
			return "", s.contentWriteError(err)
		}
		if err = tmp.Close(); err != nil {
			return "", s.contentWriteError(err)
		}
		path := filepath.Join(s.contentDirectory(), strings.TrimPrefix(filepath.Base(tmp.Name()), ".pending-"))
		if err = os.Link(tmp.Name(), path); err != nil {
			return "", s.contentWriteError(err)
		}
		created = append(created, path)
		return path, nil
	}
	for i := range cfg.Sources.Subscriptions {
		sub := &cfg.Sources.Subscriptions[i]
		if sub.Content == "" {
			continue
		}
		path, writeErr := store("subscription", ".txt", sub.Content)
		if writeErr != nil {
			return created, writeErr
		}
		sub.ContentFile, sub.Content = path, ""
	}
	for _, format := range []string{"clash", "surge", "loon"} {
		content, ok := cfg.Templates.Inline[format]
		if !ok {
			continue
		}
		ext := ".conf"
		if format == "clash" {
			ext = ".yaml"
		}
		path, writeErr := store("template-"+format, ext, content)
		if writeErr != nil {
			return created, writeErr
		}
		switch format {
		case "clash":
			cfg.Templates.Clash = path
		case "surge":
			cfg.Templates.Surge = path
		case "loon":
			cfg.Templates.Loon = path
		}
		delete(cfg.Templates.Inline, format)
	}
	if ready {
		syncDirBestEffort(s.contentDirectory())
	}
	return created, nil
}

func (s *Service) ensureContentDirectory() error {
	dir := s.contentDirectory()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return s.contentWriteError(err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		return s.contentWriteError(err)
	}
	if !info.IsDir() || info.Mode().Perm()&0o222 == 0 {
		return s.contentWriteError(os.ErrPermission)
	}
	return nil
}

// restoreEditableContent restores readable bodies and keeps failed references intact.
// Editors may keep those references for repair; archive export must reject the errors.
func (s *Service) restoreEditableContent(ctx context.Context, cfg *config.Config) error {
	var failures []error
	for i := range cfg.Sources.Subscriptions {
		sub := &cfg.Sources.Subscriptions[i]
		if sub.ContentFile == "" {
			continue
		}
		if strings.Contains(sub.ContentFile, "://") {
			failures = append(failures, newBadRequestError("invalid_request", "content_file 必须为本地文件路径"))
			continue
		}
		body, err := fetch.LoadResource(ctx, sub.ContentFile, s.fetcher)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if sub.Format == "" || sub.Format == "auto" {
			if detected := pipeline.DetectSubscriptionFormat(sub.ContentFile, body); detected != "" {
				sub.Format = detected
			}
		}
		sub.Content, sub.ContentFile = string(body), ""
	}
	for _, entry := range []struct {
		format   string
		location *string
	}{{"clash", &cfg.Templates.Clash}, {"surge", &cfg.Templates.Surge}, {"loon", &cfg.Templates.Loon}} {
		if !s.isManagedTemplate(*entry.location, entry.format) {
			continue
		}
		body, err := fetch.LoadResource(ctx, *entry.location, s.fetcher)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if cfg.Templates.Inline == nil {
			cfg.Templates.Inline = make(map[string]string)
		}
		cfg.Templates.Inline[entry.format], *entry.location = string(body), ""
	}
	return errors.Join(failures...)
}

func (s *Service) isManagedTemplate(location, format string) bool {
	if location == "" {
		return false
	}
	dir, err := filepath.Abs(filepath.Dir(location))
	if err != nil {
		return false
	}
	managedDir, err := filepath.Abs(s.contentDirectory())
	if err != nil || dir != managedDir {
		return false
	}
	name, ok := strings.CutPrefix(filepath.Base(location), "template-"+format+"-")
	if !ok {
		return false
	}
	ext := ".conf"
	if format == "clash" {
		ext = ".yaml"
	}
	name, ok = strings.CutSuffix(name, ext)
	if !ok {
		return false
	}
	stamp, suffix, ok := strings.Cut(name, "-")
	if !ok || suffix == "" {
		return false
	}
	if _, err := time.Parse(contentVersionTimeLayout, stamp); err != nil {
		return false
	}
	return strings.IndexFunc(suffix, func(r rune) bool {
		return (r < '0' || r > '9') && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z')
	}) == -1
}
