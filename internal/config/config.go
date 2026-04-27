package config

import (
	"errors"
	"fmt"
	"time"
)

type Config struct {
	Core     CoreConfig      `yaml:"core"`
	Auth     AuthConfig      `yaml:"auth,omitempty"`
	Cache    CacheConfig     `yaml:"cache,omitempty"`
	Backends []BackendConfig `yaml:"backends"`
	Logging  LoggingConfig   `yaml:"logging,omitempty"`
}

type CoreConfig struct {
	Host         string        `yaml:"host"`
	Port         int           `yaml:"port"`
	ReadTimeout  time.Duration `yaml:"readTimeout,omitempty"`
	WriteTimeout time.Duration `yaml:"writeTimeout,omitempty"`
	MaxStreams   int           `yaml:"maxStreams,omitempty"`
}

type AuthConfig struct {
	Enabled  bool     `yaml:"enabled"`
	Tokens   []string `yaml:"tokens,omitempty"`
	Strategy string   `yaml:"strategy,omitempty"`
}

type CacheConfig struct {
	Enabled      bool          `yaml:"enabled"`
	SchemaTTL    time.Duration `yaml:"schemaTTL,omitempty"`
	ResponseTTL  time.Duration `yaml:"responseTTL,omitempty"`
	MaxEntrySize int           `yaml:"maxEntrySize,omitempty"`
	MaxEntries   int           `yaml:"maxEntries,omitempty"`
}

type BackendConfig struct {
	Name        string             `yaml:"name"`
	Type        string             `yaml:"type"`
	Command     string             `yaml:"command,omitempty"`
	Args        []string           `yaml:"args,omitempty"`
	Env         map[string]string  `yaml:"env,omitempty"`
	URL         string             `yaml:"url,omitempty"`
	Timeout     time.Duration      `yaml:"timeout,omitempty"`
	MaxRetries  int                `yaml:"maxRetries,omitempty"`
	HealthCheck *HealthConfig      `yaml:"healthCheck,omitempty"`
	Auth        *BackendAuthConfig `yaml:"auth,omitempty"`
}

type BackendAuthConfig struct {
	Token string `yaml:"token,omitempty"`
}

type HealthConfig struct {
	Enabled     bool          `yaml:"enabled"`
	Interval    time.Duration `yaml:"interval,omitempty"`
	Timeout     time.Duration `yaml:"timeout,omitempty"`
	MaxFailures int           `yaml:"maxFailures,omitempty"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output,omitempty"`
}

func (c *Config) Validate() error {
	if err := c.validateBackends(); err != nil {
		return err
	}
	c.applyCoreDefaults()
	c.applyCacheDefaults()
	c.applyLoggingDefaults()
	return nil
}

func (c *Config) validateBackends() error {
	if len(c.Backends) == 0 {
		return errors.New("at least one backend must be configured")
	}

	seen := make(map[string]bool)
	for i, b := range c.Backends {
		if err := validateBackend(b, i, seen); err != nil {
			return err
		}
	}
	return nil
}

func validateBackend(b BackendConfig, i int, seen map[string]bool) error {
	if b.Name == "" {
		return fmt.Errorf("backend %d: name is required", i)
	}
	if seen[b.Name] {
		return fmt.Errorf("duplicate backend name: %s", b.Name)
	}
	seen[b.Name] = true

	if !isValidBackendType(b.Type) {
		return fmt.Errorf("backend %s: type must be 'stdio' or 'sse', got %q", b.Name, b.Type)
	}
	if err := validateBackendConn(b); err != nil {
		return err
	}
	if b.HealthCheck != nil {
		applyHealthCheckDefaults(b.HealthCheck)
	}
	return nil
}

func isValidBackendType(t string) bool {
	return t == "stdio" || t == "sse"
}

func validateBackendConn(b BackendConfig) error {
	if b.Type == "stdio" && b.Command == "" {
		return fmt.Errorf("backend %s: command is required for stdio backends", b.Name)
	}
	if b.Type == "sse" && b.URL == "" {
		return fmt.Errorf("backend %s: url is required for sse backends", b.Name)
	}
	return nil
}

func applyHealthCheckDefaults(hc *HealthConfig) {
	if hc.Interval <= 0 {
		hc.Interval = 5 * time.Second
	}
	if hc.Timeout <= 0 {
		hc.Timeout = 2 * time.Second
	}
	if hc.MaxFailures <= 0 {
		hc.MaxFailures = 3
	}
}

func (c *Config) applyCoreDefaults() {
	if c.Core.Port <= 0 {
		c.Core.Port = 9020
	}
	if c.Core.Host == "" {
		c.Core.Host = "127.0.0.1"
	}
	if c.Core.ReadTimeout <= 0 {
		c.Core.ReadTimeout = 30 * time.Second
	}
	if c.Core.WriteTimeout <= 0 {
		c.Core.WriteTimeout = 30 * time.Second
	}
	if c.Core.MaxStreams <= 0 {
		c.Core.MaxStreams = 100
	}
}

func (c *Config) applyCacheDefaults() {
	if c.Cache.SchemaTTL <= 0 {
		c.Cache.SchemaTTL = 60 * time.Second
	}
	if c.Cache.ResponseTTL <= 0 {
		c.Cache.ResponseTTL = 1 * time.Second
	}
	if c.Cache.MaxEntrySize <= 0 {
		c.Cache.MaxEntrySize = 1024 * 1024
	}
	if c.Cache.MaxEntries <= 0 {
		c.Cache.MaxEntries = 1000
	}
}

func (c *Config) applyLoggingDefaults() {
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "json"
	}
}

func (c *Config) ListenAddr() string {
	return fmt.Sprintf("%s:%d", c.Core.Host, c.Core.Port)
}
