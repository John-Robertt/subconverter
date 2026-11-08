package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/fetch"
)

const missingConfigRevision = "missing"

type savedConfig struct {
	raw      []byte
	revision string
	exists   bool
}

// readSavedConfig distinguishes an absent local file from an existing empty or invalid document.
func (s *Service) readSavedConfig(ctx context.Context, invalidate bool) (savedConfig, error) {
	if s.configLocation != "" && !isRemoteConfig(s.configLocation) {
		if _, err := os.Lstat(filepath.Clean(s.configLocation)); errors.Is(err, os.ErrNotExist) {
			return savedConfig{revision: missingConfigRevision}, nil
		}
	}
	raw, err := s.readConfigBytes(ctx, invalidate)
	if err != nil {
		return savedConfig{}, err
	}
	return savedConfig{raw: raw, revision: configRevision(raw), exists: true}, nil
}

func (s *Service) readConfigBytes(ctx context.Context, invalidate bool) ([]byte, error) {
	if s.configLocation == "" {
		return nil, &errtype.ConfigError{
			Code:    errtype.CodeConfigLoadFailed,
			Message: "配置路径为空",
		}
	}
	if invalidate && isRemoteConfig(s.configLocation) {
		if invalidator, ok := s.fetcher.(interface{ Invalidate(string) }); ok {
			invalidator.Invalidate(s.configLocation)
		}
	}
	return fetch.LoadResource(ctx, s.configLocation, s.fetcher)
}

func configRevision(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func isRemoteConfig(location string) bool {
	return strings.HasPrefix(location, "http://") || strings.HasPrefix(location, "https://")
}

func writeFileAtomically(path string, data []byte, createOnly bool) error {
	clean := filepath.Clean(path)
	dir := filepath.Dir(clean)
	base := filepath.Base(clean)
	tmp, err := os.OpenFile(filepath.Join(dir, "."+base+".tmp"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) // #nosec G304 -- path is derived from the configured local config file location.
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			tmp, err = os.CreateTemp(dir, "."+base+".tmp-*")
		}
		if err != nil {
			return fmt.Errorf("%w: %v", errtype.ErrConfigFileNotWritable, err)
		}
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("%w: %v", errtype.ErrConfigFileNotWritable, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("%w: %v", errtype.ErrConfigFileNotWritable, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("%w: %v", errtype.ErrConfigFileNotWritable, err)
	}
	if createOnly {
		err = os.Link(tmpName, clean)
	} else {
		err = os.Rename(tmpName, clean)
	}
	if err != nil {
		if createOnly && errors.Is(err, os.ErrExist) {
			return err
		}
		return fmt.Errorf("%w: %v", errtype.ErrConfigFileNotWritable, err)
	}
	syncDirBestEffort(dir)
	return nil
}

func ensureLocalConfigWritable(path string) error {
	const writableAccessMode = 0x2

	clean := filepath.Clean(path)
	info, err := os.Stat(clean)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: %v", errtype.ErrConfigFileNotWritable, err)
	}
	if err == nil {
		if info.IsDir() || info.Mode().Perm()&0o200 == 0 {
			return errtype.ErrConfigFileNotWritable
		}
		if err := syscall.Access(clean, writableAccessMode); err != nil {
			return fmt.Errorf("%w: %v", errtype.ErrConfigFileNotWritable, err)
		}
	}

	dirInfo, err := os.Stat(filepath.Dir(clean))
	if err != nil {
		return fmt.Errorf("%w: %v", errtype.ErrConfigFileNotWritable, err)
	}
	if !dirInfo.IsDir() || dirInfo.Mode().Perm()&0o200 == 0 {
		return errtype.ErrConfigFileNotWritable
	}
	if err := syscall.Access(filepath.Dir(clean), writableAccessMode); err != nil {
		return fmt.Errorf("%w: %v", errtype.ErrConfigFileNotWritable, err)
	}
	return nil
}

func syncDirBestEffort(dir string) {
	d, err := os.Open(dir) // #nosec G304 -- dir is the cleaned parent of the configured local config path.
	if err != nil {
		return
	}
	defer func() { _ = d.Close() }()
	_ = d.Sync()
}
