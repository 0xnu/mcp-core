package client

import (
	"time"

	"github.com/0xnu/mcp-core/pkg/protocol"
)

type BackendType string

const (
	BackendTypeStdio BackendType = "stdio"
	BackendTypeSSE   BackendType = "sse"
)

type BackendConfig struct {
	Name        string             `yaml:"name"`
	Type        BackendType        `yaml:"type"`
	Command     string             `yaml:"command,omitempty"`
	Args        []string           `yaml:"args,omitempty"`
	Env         map[string]string  `yaml:"env,omitempty"`
	URL         string             `yaml:"url,omitempty"`
	Timeout     time.Duration      `yaml:"timeout,omitempty"`
	MaxRetries  int                `yaml:"maxRetries,omitempty"`
	HealthCheck *HealthCheckConfig `yaml:"healthCheck,omitempty"`
}

type HealthCheckConfig struct {
	Interval    time.Duration `yaml:"interval,omitempty"`
	Timeout     time.Duration `yaml:"timeout,omitempty"`
	MaxFailures int           `yaml:"maxFailures,omitempty"`
}

type BackendStatus string

const (
	StatusConnected    BackendStatus = "connected"
	StatusDisconnected BackendStatus = "disconnected"
	StatusError        BackendStatus = "error"
	StatusDraining     BackendStatus = "draining"
)

type BackendInfo struct {
	Config      BackendConfig
	Status      BackendStatus
	Tools       []protocol.ToolDefinition
	Resources   []protocol.Resource
	Prompts     []protocol.Prompt
	Latency     time.Duration
	ConnectedAt time.Time
	ErrorCount  int
}

type ClientOption func(*ClientConfig)

type ClientConfig struct {
	MaxRetries     int
	RetryDelay     time.Duration
	RequestTimeout time.Duration
	BufferSize     int
}

func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		MaxRetries:     3,
		RetryDelay:     time.Second,
		RequestTimeout: 30 * time.Second,
		BufferSize:     4096,
	}
}
