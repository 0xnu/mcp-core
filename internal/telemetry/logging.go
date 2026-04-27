package telemetry

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"maps"
	"os"
	"strings"
	"sync"
	"time"
)

type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	case LevelFatal:
		return "fatal"
	default:
		return "unknown"
	}
}

func ParseLevel(s string) (LogLevel, error) {
	switch strings.ToLower(s) {
	case "debug":
		return LevelDebug, nil
	case "info":
		return LevelInfo, nil
	case "warn", "warning":
		return LevelWarn, nil
	case "error":
		return LevelError, nil
	case "fatal":
		return LevelFatal, nil
	default:
		return LevelInfo, fmt.Errorf("unknown log level: %s", s)
	}
}

type Logger struct {
	mu     sync.Mutex
	level  LogLevel
	format string
	output io.Writer
	attrs  map[string]string
}

type LoggerOption func(*Logger)

func WithLevel(level LogLevel) LoggerOption {
	return func(l *Logger) {
		l.level = level
	}
}

func WithFormat(format string) LoggerOption {
	return func(l *Logger) {
		l.format = format
	}
}

func WithOutput(w io.Writer) LoggerOption {
	return func(l *Logger) {
		l.output = w
	}
}

func WithAttr(key, value string) LoggerOption {
	return func(l *Logger) {
		l.attrs[key] = value
	}
}

func NewLogger(opts ...LoggerOption) *Logger {
	l := &Logger{
		level:  LevelInfo,
		format: "json",
		output: os.Stdout,
		attrs:  make(map[string]string),
	}

	for _, opt := range opts {
		opt(l)
	}

	return l
}

func (l *Logger) Debug(msg string, args ...any) {
	l.log(LevelDebug, msg, args...)
}

func (l *Logger) Info(msg string, args ...any) {
	l.log(LevelInfo, msg, args...)
}

func (l *Logger) Warn(msg string, args ...any) {
	l.log(LevelWarn, msg, args...)
}

func (l *Logger) Error(msg string, args ...any) {
	l.log(LevelError, msg, args...)
}

func (l *Logger) Fatal(msg string, args ...any) {
	l.log(LevelFatal, msg, args...)
	os.Exit(1)
}

func (l *Logger) log(level LogLevel, msg string, args ...any) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	entry := l.buildEntry(level, msg, args...)

	var output string
	switch l.format {
	case "json":
		data, _ := json.Marshal(entry) //nolint:errchkjson
		output = string(data)
	default:
		output = l.formatText(entry)
	}

	fmt.Fprintln(l.output, output)

	if level == LevelFatal {
		l.mu.Unlock()
		os.Exit(1) //nolint:gocritic
		return
	}
}

type logEntry struct {
	Level     string            `json:"level"`
	Message   string            `json:"message"`
	Timestamp string            `json:"timestamp"`
	Attrs     map[string]string `json:"attrs,omitempty"`
}

func (l *Logger) buildEntry(level LogLevel, msg string, args ...any) logEntry {
	entry := logEntry{
		Level:     level.String(),
		Message:   fmt.Sprintf(msg, args...),
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Attrs:     make(map[string]string),
	}

	maps.Copy(entry.Attrs, l.attrs)

	return entry
}

func (l *Logger) formatText(entry logEntry) string {
	return fmt.Sprintf("[%s] %s %s", entry.Level, entry.Timestamp, entry.Message)
}

func (l *Logger) WithAttrs(attrs map[string]string) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newLogger := &Logger{
		level:  l.level,
		format: l.format,
		output: l.output,
		attrs:  make(map[string]string),
	}

	maps.Copy(newLogger.attrs, l.attrs)
	maps.Copy(newLogger.attrs, attrs)

	return newLogger
}

func InitLogging(level, format string) *Logger {
	lvl, err := ParseLevel(level)
	if err != nil {
		lvl = LevelInfo
	}

	return NewLogger(
		WithLevel(lvl),
		WithFormat(format),
		WithOutput(os.Stdout),
	)
}

func SetGlobalLogger(l *Logger) {
	log.SetOutput(l.output)
	log.SetFlags(0)
}
