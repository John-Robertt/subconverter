package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/fetch"
	"github.com/John-Robertt/subconverter/internal/generate"
	"gopkg.in/yaml.v3"
)

type ConfigSnapshotResult struct {
	ConfigRevision string          `json:"config_revision"`
	Config         json.RawMessage `json:"config"`
}

type SaveConfigInput struct {
	ConfigRevision string          `json:"config_revision"`
	Config         json.RawMessage `json:"config"`
}

type SaveConfigResult struct {
	ConfigRevision string `json:"config_revision"`
}

type ReloadResult struct {
	Success    bool  `json:"success"`
	DurationMs int64 `json:"duration_ms"`
}

type lastReloadState struct {
	Time       time.Time
	Success    bool
	DurationMs int64
	Error      string
}

type Options struct {
	ConfigLocation string
	ListenAddr     string
	Fetcher        fetch.Fetcher
	Generate       generate.Options
	Now            func() time.Time
	Version        string
	Commit         string
	BuildDate      string
}

type Service struct {
	configLocation string
	listenAddr     string
	startedAt      time.Time
	fetcher        fetch.Fetcher
	generator      *generate.Service
	now            func() time.Time
	version        string
	commit         string
	buildDate      string
	accessToken    string

	requestCount atomic.Uint64

	mu                     sync.RWMutex
	runtimeCfg             *config.RuntimeConfig
	runtimeConfigRevision  string
	observedConfigRevision string
	configLoadedAt         time.Time
	lastReload             lastReloadState

	reloadMu sync.Mutex
}

// IncrementRequestCount records an inbound request for the runtime environment
// stat surface. Called from the HTTP middleware on every non-internal request.
func (s *Service) IncrementRequestCount() {
	s.requestCount.Add(1)
}

func New(ctx context.Context, opts Options) (*Service, error) {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	s := &Service{
		configLocation: opts.ConfigLocation,
		listenAddr:     opts.ListenAddr,
		startedAt:      now(),
		fetcher:        opts.Fetcher,
		generator:      generate.New(opts.Fetcher, opts.Generate),
		now:            now,
		version:        opts.Version,
		commit:         opts.Commit,
		buildDate:      opts.BuildDate,
		accessToken:    opts.Generate.AccessToken,
	}

	raw, err := s.readConfigBytes(ctx, false)
	if err != nil {
		return nil, err
	}
	cfg, err := parseConfigYAML(raw)
	if err != nil {
		return nil, err
	}
	runtimeCfg, err := prepareAdminConfig(cfg, false)
	if err != nil {
		return nil, err
	}
	s.runtimeCfg = runtimeCfg
	s.runtimeConfigRevision = configRevision(raw)
	s.observedConfigRevision = s.runtimeConfigRevision
	s.configLoadedAt = now()
	return s, nil
}

func NewWithRuntime(location string, runtimeCfg *config.RuntimeConfig, fetcher fetch.Fetcher, genOpts generate.Options) *Service {
	return &Service{
		configLocation:         location,
		startedAt:              time.Now(),
		fetcher:                fetcher,
		generator:              generate.New(fetcher, genOpts),
		now:                    time.Now,
		accessToken:            genOpts.AccessToken,
		runtimeCfg:             runtimeCfg,
		runtimeConfigRevision:  "",
		observedConfigRevision: "",
		configLoadedAt:         time.Now(),
	}
}

func (s *Service) ConfigSnapshot(ctx context.Context) (*ConfigSnapshotResult, error) {
	raw, err := s.readConfigBytes(ctx, false)
	if err != nil {
		return nil, err
	}
	cfg, err := parseConfigYAML(raw)
	if err != nil {
		return nil, err
	}
	configJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	revision := configRevision(raw)
	s.setObservedConfigRevision(revision)
	return &ConfigSnapshotResult{
		ConfigRevision: revision,
		Config:         configJSON,
	}, nil
}

func (s *Service) SaveConfig(ctx context.Context, input *SaveConfigInput) (*SaveConfigResult, error) {
	if input == nil || input.ConfigRevision == "" {
		return nil, newBadRequestError("config_revision_required", "缺少 config_revision")
	}
	if s.configLocation == "" || isRemoteConfig(s.configLocation) {
		return nil, errtype.ErrConfigSourceReadonly
	}
	if err := ensureLocalConfigWritable(s.configLocation); err != nil {
		return nil, err
	}

	currentRaw, err := s.readConfigBytes(ctx, false)
	if err != nil {
		return nil, err
	}
	currentRevision := configRevision(currentRaw)
	if input.ConfigRevision != currentRevision {
		return nil, &errtype.RevisionConflictError{CurrentConfigRevision: currentRevision}
	}

	cfg, err := parseConfigJSON(input.Config)
	if err != nil {
		return nil, err
	}
	if _, err := prepareAdminConfig(cfg, true); err != nil {
		return nil, err
	}

	yamlBytes, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal config yaml: %w", err)
	}
	if err := writeFileAtomically(s.configLocation, yamlBytes); err != nil {
		return nil, err
	}
	return &SaveConfigResult{ConfigRevision: configRevision(yamlBytes)}, nil
}

func (s *Service) ValidateDraft(_ context.Context, configJSON json.RawMessage) (*ValidateResult, error) {
	cfg, err := parseConfigJSON(configJSON)
	if err != nil {
		return nil, err
	}
	if _, err := prepareAdminConfig(cfg, true); err != nil {
		if result, ok := ValidateResultFromError(err); ok {
			return result, nil
		}
		return nil, err
	}
	return &ValidateResult{
		Valid:    true,
		Errors:   []DiagnosticItem{},
		Warnings: []DiagnosticItem{},
		Infos:    []DiagnosticItem{},
	}, nil
}

func (s *Service) Reload(ctx context.Context) (*ReloadResult, error) {
	if !s.reloadMu.TryLock() {
		return nil, errtype.ErrReloadInProgress
	}
	defer s.reloadMu.Unlock()

	start := s.now()
	raw, err := s.readConfigBytes(ctx, true)
	if err != nil {
		s.recordReload(start, false, err)
		return nil, err
	}
	revision := configRevision(raw)
	s.setObservedConfigRevision(revision)
	cfg, err := parseConfigYAML(raw)
	if err != nil {
		s.recordReload(start, false, err)
		return nil, err
	}
	runtimeCfg, err := prepareAdminConfig(cfg, false)
	if err != nil {
		s.recordReload(start, false, err)
		return nil, err
	}

	s.mu.Lock()
	s.runtimeCfg = runtimeCfg
	s.runtimeConfigRevision = revision
	s.observedConfigRevision = revision
	s.configLoadedAt = s.now()
	s.lastReload = lastReloadState{
		Time:       s.now(),
		Success:    true,
		DurationMs: s.now().Sub(start).Milliseconds(),
	}
	s.mu.Unlock()

	return &ReloadResult{
		Success:    true,
		DurationMs: s.now().Sub(start).Milliseconds(),
	}, nil
}

func (s *Service) Generate(ctx context.Context, req GenerateInput) (*GenerateResult, error) {
	cfg := s.runtimeSnapshot()
	return s.generator.Generate(ctx, cfg, req)
}

func (s *Service) runtimeSnapshot() *config.RuntimeConfig {
	s.mu.RLock()
	cfg := s.runtimeCfg
	s.mu.RUnlock()
	return cfg
}

func (s *Service) setObservedConfigRevision(revision string) {
	s.mu.Lock()
	s.observedConfigRevision = revision
	s.mu.Unlock()
}

func (s *Service) recordReload(start time.Time, success bool, reloadErr error) {
	s.mu.Lock()
	state := lastReloadState{
		Time:       s.now(),
		Success:    success,
		DurationMs: s.now().Sub(start).Milliseconds(),
	}
	if !success && reloadErr != nil {
		state.Error = reloadErr.Error()
	}
	s.lastReload = state
	s.mu.Unlock()
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

func parseConfigYAML(raw []byte) (*config.Config, error) {
	var cfg config.Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, &errtype.ConfigError{
			Code:    errtype.CodeConfigYAMLInvalid,
			Message: "解析 YAML 失败：" + err.Error(),
		}
	}
	return &cfg, nil
}

func parseConfigJSON(raw json.RawMessage) (*config.Config, error) {
	if !isJSONObject(raw) {
		return nil, newBadRequestError("invalid_config", "config 必须是对象")
	}
	var cfg config.Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, newBadRequestError("invalid_request", "config JSON 无法解析")
	}
	return &cfg, nil
}

func isJSONObject(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")
}

func configRevision(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func isRemoteConfig(location string) bool {
	return strings.HasPrefix(location, "http://") || strings.HasPrefix(location, "https://")
}

func writeFileAtomically(path string, data []byte) error {
	clean := filepath.Clean(path)
	dir := filepath.Dir(clean)
	base := filepath.Base(clean)
	tmp, err := os.OpenFile(filepath.Join(dir, "."+base+".tmp"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
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
	if err := os.Rename(tmpName, clean); err != nil {
		return fmt.Errorf("%w: %v", errtype.ErrConfigFileNotWritable, err)
	}
	syncDirBestEffort(dir)
	return nil
}

func ensureLocalConfigWritable(path string) error {
	clean := filepath.Clean(path)
	info, err := os.Stat(clean)
	if err != nil {
		return fmt.Errorf("%w: %v", errtype.ErrConfigFileNotWritable, err)
	}
	if info.IsDir() || info.Mode().Perm()&0o200 == 0 {
		return errtype.ErrConfigFileNotWritable
	}
	dirInfo, err := os.Stat(filepath.Dir(clean))
	if err != nil {
		return fmt.Errorf("%w: %v", errtype.ErrConfigFileNotWritable, err)
	}
	if !dirInfo.IsDir() || dirInfo.Mode().Perm()&0o200 == 0 {
		return errtype.ErrConfigFileNotWritable
	}
	return nil
}

func syncDirBestEffort(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	defer func() { _ = d.Close() }()
	_ = d.Sync()
}
