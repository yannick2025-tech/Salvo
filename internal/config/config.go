package config

import (
	"fmt"
	"os"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/logger"
	"gopkg.in/yaml.v3"
)

type RunMode string

const (
	RunModeDuration RunMode = "duration"
	RunModeCount    RunMode = "count"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Log      LogConfig      `yaml:"log"`
	Pool     PoolConfig     `yaml:"pool"`
	Storage  StorageConfig  `yaml:"storage"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type DatabaseConfig struct {
	Driver   string `yaml:"driver"`
	DSN      string `yaml:"dsn"`
	MaxOpen  int    `yaml:"max_open"`
	MaxIdle  int    `yaml:"max_idle"`
	LogLevel string `yaml:"log_level"`
}

type LogConfig struct {
	Level      logger.LogLevel  `yaml:"level"`
	Format     logger.LogFormat `yaml:"format"`
	Output     string           `yaml:"output"`
	TimeFormat string           `yaml:"time_format"`
}

type PoolConfig struct {
	WorkerCount int           `yaml:"worker_count"`
	RunMode     RunMode       `yaml:"run_mode"`
	Duration    time.Duration `yaml:"duration"`
	Count       int64         `yaml:"count"`
}

type StorageConfig struct {
	Driver string `yaml:"driver"`
	DSN    string `yaml:"dsn"`
}

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
			TimeFormat: "2006-01-02T15:04:05.000Z07:00",
		},
		Pool: PoolConfig{
			WorkerCount: 20,
			RunMode:     RunModeCount,
			Count:       10000,
		},
	}
}

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

func (c *Config) ServerAddr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}
