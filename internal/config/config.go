// Package config provides YAML-based configuration loading and validation
// for the Salvo performance testing engine.
//
// A Config struct aggregates all subsystem configurations (server, database,
// logging, pool, storage) and offers sensible defaults via the Default
// function. Partial YAML files overlay the defaults, so only the fields
// that differ need to be specified.
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/logger"
	"gopkg.in/yaml.v3"
)

// RunMode determines how the goroutine pool terminates a test run.
type RunMode string

const (
	// RunModeDuration runs the test for a fixed wall-clock duration.
	RunModeDuration RunMode = "duration"
	// RunModeCount runs the test for a fixed number of iterations.
	RunModeCount RunMode = "count"
)

// Config is the top-level configuration for the Salvo application.
type Config struct {
	Server    ServerConfig      `yaml:"server"`
	Database  DatabaseConfig    `yaml:"database"`
	Log       LogConfig         `yaml:"log"`
	Pool      PoolConfig        `yaml:"pool"`
	Storage   StorageConfig     `yaml:"storage"`
	Auth      AuthConfig        `yaml:"auth"`
	Mock      MockConfig        `yaml:"mock"`
	Variables map[string]string `yaml:"variables"`
}

// ServerConfig holds the HTTP server listen address.
type ServerConfig struct {
	Host   string `yaml:"host"`
	Port   int    `yaml:"port"`
	WebDir string `yaml:"web_dir"`
}

// DatabaseConfig holds the relational database connection parameters.
type DatabaseConfig struct {
	// Driver is the SQL driver name (e.g. "sqlite3", "mysql", "postgres").
	Driver string `yaml:"driver"`
	// DSN is the data source name / connection string.
	DSN string `yaml:"dsn"`
	// MaxOpen is the maximum number of open database connections.
	MaxOpen int `yaml:"max_open"`
	// MaxIdle is the maximum number of idle database connections.
	MaxIdle int `yaml:"max_idle"`
	// LogLevel controls the ORM query log verbosity.
	LogLevel string `yaml:"log_level"`
}

// LogConfig configures the structured logger.
type LogConfig struct {
	Level      logger.LogLevel  `yaml:"level"`
	Format     logger.LogFormat `yaml:"format"`
	Output     string           `yaml:"output"`
	MaxSize    int              `yaml:"max_size"`
	MaxBackups int              `yaml:"max_backups"`
	MaxAge     int              `yaml:"max_age"`
	Compress   bool             `yaml:"compress"`
	TimeFormat string           `yaml:"time_format"`
}

// PoolConfig configures the goroutine pool that drives test execution.
type PoolConfig struct {
	// WorkerCount is the fixed number of goroutines in the pool.
	WorkerCount int `yaml:"worker_count"`
	// RunMode determines whether the run stops by duration or iteration count.
	RunMode RunMode `yaml:"run_mode"`
	// Duration is the total run time when RunMode is RunModeDuration.
	Duration time.Duration `yaml:"duration"`
	// Count is the total iteration count when RunMode is RunModeCount.
	Count int64 `yaml:"count"`
}

// StorageConfig holds the object/file storage connection parameters.
type StorageConfig struct {
	// Driver is the storage backend identifier.
	Driver string `yaml:"driver"`
	// DSN is the storage connection string.
	DSN string `yaml:"dsn"`
}

// AuthConfig holds the authentication configuration.
type AuthConfig struct {
	JWTSecret string `yaml:"jwt_secret"`
}

type MockConfig struct {
	Enabled      bool    `yaml:"enabled"`
	Port         int     `yaml:"port"`
	ErrorRate    float64 `yaml:"error_rate"`
	LatencyMinMs int     `yaml:"latency_min_ms"`
	LatencyMaxMs int     `yaml:"latency_max_ms"`
}

// Default returns a Config populated with sensible defaults.
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Database: DatabaseConfig{
			Driver:  "sqlite3",
			DSN:     "salvo.db",
			MaxOpen: 10,
			MaxIdle: 5,
		},
		Log: LogConfig{
			Level:      logger.InfoLevel,
			Format:     logger.FormatJSON,
			MaxSize:    500,
			MaxBackups: 5,
			MaxAge:     30,
			Compress:   true,
			TimeFormat: "2006-01-02T15:04:05.000Z07:00",
		},
		Pool: PoolConfig{
			WorkerCount: 20,
			RunMode:     RunModeCount,
			Count:       10000,
		},
		Auth: AuthConfig{
			JWTSecret: "salvo-default-jwt-secret-change-me",
		},
		Mock: MockConfig{
			Enabled:      false,
			Port:         9090,
			ErrorRate:    0.03,
			LatencyMinMs: 30,
			LatencyMaxMs: 226,
		},
	}
}

// Load reads a YAML configuration file at path, overlays it onto the
// defaults, and validates the result. Returns an error if the file
// cannot be read, parsed, or validated.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config file %s: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
}

// Validate checks the Config for semantic errors and returns the first
// error encountered.
func (c *Config) Validate() error {
	if c.Server.Port < 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if c.Pool.WorkerCount <= 0 {
		return fmt.Errorf("pool worker_count must be > 0, got %d", c.Pool.WorkerCount)
	}

	switch c.Pool.RunMode {
	case RunModeDuration:
		if c.Pool.Duration <= 0 {
			return fmt.Errorf("pool duration must be > 0 when run_mode is duration")
		}
	case RunModeCount:
		if c.Pool.Count <= 0 {
			return fmt.Errorf("pool count must be > 0 when run_mode is count")
		}
	default:
		return fmt.Errorf("invalid pool run_mode: %s", c.Pool.RunMode)
	}

	return nil
}

// ServerAddr returns the combined host:port listen address string.
func (c *Config) ServerAddr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}
