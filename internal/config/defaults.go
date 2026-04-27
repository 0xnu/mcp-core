package config

import (
	"os"
	"path/filepath"
	"runtime"
	"time"
)

func DefaultConfig() *Config {
	return &Config{
		Core: CoreConfig{
			Host:         "127.0.0.1",
			Port:         9020,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			MaxStreams:   100,
		},
		Cache: CacheConfig{
			Enabled:      true,
			SchemaTTL:    60 * time.Second,
			ResponseTTL:  1 * time.Second,
			MaxEntrySize: 1024 * 1024,
			MaxEntries:   1000,
		},
		Auth: AuthConfig{
			Enabled:  false,
			Strategy: "bearer",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
	}
}

func ScaffoldConfig() *Config {
	cfg := DefaultConfig()

	cfg.Backends = []BackendConfig{
		{
			Name:    "filesystem",
			Type:    "stdio",
			Command: "npx",
			Args:    []string{"-y", "@modelcontextprotocol/server-filesystem", "."},
		},
	}

	return cfg
}

func DetectExistingConfigs() ([]string, error) {
	var configs []string

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	candidatePaths := map[string]string{
		"cursor": filepath.Join(home, ".cursor", "mcp.json"),
		"claude": filepath.Join(home, ".claude", "mcp.json"),
		"vscode": filepath.Join(home, ".vscode", "mcp.json"),
	}

	for _, path := range candidatePaths {
		if _, err := os.Stat(path); err == nil {
			configs = append(configs, path)
		}
	}

	return configs, nil
}

func IsWindows() bool {
	return runtime.GOOS == "windows"
}

func DefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	if IsWindows() {
		return filepath.Join(home, "AppData", "Roaming", "mcp-core", "mcp-core.yaml")
	}
	return filepath.Join(home, ".config", "mcp-core", "mcp-core.yaml")
}
