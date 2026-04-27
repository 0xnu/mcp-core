package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Loader struct {
	paths []string
}

func NewLoader(paths ...string) *Loader {
	return &Loader{paths: paths}
}

func (l *Loader) Load() (*Config, error) {
	if len(l.paths) == 0 {
		l.paths = defaultConfigPaths()
	}

	var lastErr error
	for _, path := range l.paths {
		cfg, err := loadFile(path)
		if err == nil {
			return cfg, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("no config file found (searched %v): %w", l.paths, lastErr)
}

func loadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate %s: %w", path, err)
	}

	return &cfg, nil
}

func LoadBytes(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}

func defaultConfigPaths() []string {
	home, _ := os.UserHomeDir()
	return []string{
		"mcp-core.yaml",
		"mcp-core.yml",
		filepath.Join(home, ".config", "mcp-core", "mcp-core.yaml"),
		filepath.Join(home, ".config", "mcp-core", "mcp-core.yml"),
		"/etc/mcp-core/mcp-core.yaml",
		"/etc/mcp-core/mcp-core.yml",
	}
}

func (c *Config) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

func (c *Config) Dump() (string, error) {
	data, err := yaml.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("marshal config: %w", err)
	}
	return string(data), nil
}
