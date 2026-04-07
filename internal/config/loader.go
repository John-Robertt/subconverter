package config

import (
	"os"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"gopkg.in/yaml.v3"
)

// Load reads a YAML configuration file and returns the parsed Config.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &errtype.ConfigError{
			Message: "reading config file: " + err.Error(),
		}
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, &errtype.ConfigError{
			Message: "parsing YAML: " + err.Error(),
		}
	}

	return &cfg, nil
}
