package app

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"
)

type StatusResult struct {
	Version               string             `json:"version"`
	Commit                string             `json:"commit"`
	BuildDate             string             `json:"build_date"`
	ConfigSource          ConfigSource       `json:"config_source"`
	ConfigRevision        string             `json:"config_revision"`
	RuntimeConfigRevision string             `json:"runtime_config_revision"`
	ConfigLoadedAt        string             `json:"config_loaded_at"`
	ConfigDirty           bool               `json:"config_dirty"`
	Capabilities          Capabilities       `json:"capabilities"`
	LastReload            *LastReload        `json:"last_reload,omitempty"`
	RuntimeEnvironment    RuntimeEnvironment `json:"runtime_environment"`
}

type RuntimeEnvironment struct {
	ListenAddr      string `json:"listen_addr"`
	WorkingDir      string `json:"working_dir"`
	GoRuntime       string `json:"go_runtime"`
	MemoryAllocMB   string `json:"memory_alloc_mb"`
	RequestCount24h uint64 `json:"request_count_24h"`
	UptimeSeconds   int64  `json:"uptime_seconds"`
}

type ConfigSource struct {
	Location string `json:"location"`
	Type     string `json:"type"`
	Writable bool   `json:"writable"`
}

type Capabilities struct {
	ConfigWrite bool `json:"config_write"`
	Reload      bool `json:"reload"`
}

type LastReload struct {
	Time       string `json:"time"`
	Success    bool   `json:"success"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

func (s *Service) Status(ctx context.Context) (*StatusResult, error) {
	s.mu.RLock()
	runtimeRevision := s.runtimeConfigRevision
	observedRevision := s.observedConfigRevision
	loadedAt := s.configLoadedAt
	lastReload := s.lastReload
	s.mu.RUnlock()

	configRevision := observedRevision
	if !isRemoteConfig(s.configLocation) {
		raw, err := s.readConfigBytes(ctx, false)
		if err != nil {
			return nil, err
		}
		configRevision = configRevisionForStatus(raw)
	}

	source := s.configSource()
	result := &StatusResult{
		Version:               s.version,
		Commit:                s.commit,
		BuildDate:             s.buildDate,
		ConfigSource:          source,
		ConfigRevision:        configRevision,
		RuntimeConfigRevision: runtimeRevision,
		ConfigLoadedAt:        formatStatusTime(loadedAt),
		ConfigDirty:           configRevision != runtimeRevision,
		Capabilities: Capabilities{
			ConfigWrite: source.Writable,
			Reload:      true,
		},
		RuntimeEnvironment: s.runtimeEnvironment(),
	}
	if !lastReload.Time.IsZero() {
		result.LastReload = &LastReload{
			Time:       formatStatusTime(lastReload.Time),
			Success:    lastReload.Success,
			DurationMs: lastReload.DurationMs,
			Error:      lastReload.Error,
		}
	}
	return result, nil
}

func (s *Service) runtimeEnvironment() RuntimeEnvironment {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	wd, _ := os.Getwd()
	uptime := int64(0)
	if !s.startedAt.IsZero() {
		uptime = int64(s.now().Sub(s.startedAt).Seconds())
	}
	return RuntimeEnvironment{
		ListenAddr:      s.listenAddr,
		WorkingDir:      wd,
		GoRuntime:       fmt.Sprintf("%s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH),
		MemoryAllocMB:   fmt.Sprintf("%.1f", float64(mem.Alloc)/1024.0/1024.0),
		RequestCount24h: s.requestCount.Load(),
		UptimeSeconds:   uptime,
	}
}

func (s *Service) configSource() ConfigSource {
	sourceType := "local"
	writable := s.configLocation != ""
	if isRemoteConfig(s.configLocation) {
		sourceType = "remote"
		writable = false
	}
	return ConfigSource{
		Location: s.configLocation,
		Type:     sourceType,
		Writable: writable,
	}
}

func configRevisionForStatus(raw []byte) string {
	return configRevision(raw)
}

func formatStatusTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
