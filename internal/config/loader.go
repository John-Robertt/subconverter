package config

import (
	"context"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/fetch"
	"gopkg.in/yaml.v3"
)

// Load reads a YAML configuration from a local file path or remote HTTP(S) URL.
// When f is nil, only local paths are supported.
func Load(ctx context.Context, location string, f fetch.Fetcher) (*Config, error) {
	data, err := fetch.LoadResource(ctx, location, f)
	if err != nil {
		return nil, &errtype.ConfigError{
			Code:    errtype.CodeConfigLoadFailed,
			Message: "读取配置失败：" + err.Error(),
		}
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, &errtype.ConfigError{
			Code:    errtype.CodeConfigYAMLInvalid,
			Message: "解析 YAML 失败：" + err.Error(),
		}
	}

	return &cfg, nil
}
