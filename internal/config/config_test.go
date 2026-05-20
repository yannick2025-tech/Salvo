package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yannick2025-tech/Salvo/internal/logger"
)

func TestDefaultConfig(t *testing.T) {
	cfg := Default()

	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "sqlite3", cfg.Database.Driver)
	assert.Equal(t, logger.InfoLevel, cfg.Log.Level)
	assert.Equal(t, logger.FormatJSON, cfg.Log.Format)
	assert.Equal(t, 20, cfg.Pool.WorkerCount)
	assert.Equal(t, RunModeCount, cfg.Pool.RunMode)
	assert.Equal(t, int64(10000), cfg.Pool.Count)
}

func TestLoadFromYAML(t *testing.T) {
	yamlContent := `
server:
  host: "127.0.0.1"
  port: 9090

database:
  driver: "mysql"
  dsn: "user:pass@tcp(localhost:3306)/salvo"
  max_open: 20
  max_idle: 10

log:
  level: "debug"
  format: "text"
  output: "/var/log/salvo.log"

pool:
  worker_count: 50
  run_mode: "duration"
  duration: "2h"
`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(yamlContent), 0644))

	cfg, err := Load(cfgPath)
	require.NoError(t, err)

	assert.Equal(t, "127.0.0.1", cfg.Server.Host)
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "mysql", cfg.Database.Driver)
	assert.Equal(t, "user:pass@tcp(localhost:3306)/salvo", cfg.Database.DSN)
	assert.Equal(t, 20, cfg.Database.MaxOpen)
	assert.Equal(t, 10, cfg.Database.MaxIdle)
	assert.Equal(t, logger.DebugLevel, cfg.Log.Level)
	assert.Equal(t, logger.FormatText, cfg.Log.Format)
	assert.Equal(t, "/var/log/salvo.log", cfg.Log.Output)
	assert.Equal(t, 50, cfg.Pool.WorkerCount)
	assert.Equal(t, RunModeDuration, cfg.Pool.RunMode)
	assert.Equal(t, 2*time.Hour, cfg.Pool.Duration)
}

func TestLoadPartialYAML(t *testing.T) {
	yamlContent := `
server:
  port: 3000
`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(yamlContent), 0644))

	cfg, err := Load(cfgPath)
	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 3000, cfg.Server.Port)
	assert.Equal(t, "sqlite3", cfg.Database.Driver)
	assert.Equal(t, logger.InfoLevel, cfg.Log.Level)
}

func TestLoadFileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read config file")
}

func TestLoadInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("{{invalid yaml"), 0644))

	_, err := Load(cfgPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse config file")
}

func TestValidateValidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "count mode",
			cfg: &Config{
				Server: ServerConfig{Host: "0.0.0.0", Port: 8080},
				Pool:  PoolConfig{WorkerCount: 10, RunMode: RunModeCount, Count: 1000},
			},
		},
		{
			name: "duration mode",
			cfg: &Config{
				Server: ServerConfig{Host: "0.0.0.0", Port: 8080},
				Pool:  PoolConfig{WorkerCount: 10, RunMode: RunModeDuration, Duration: time.Hour},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NoError(t, tt.cfg.Validate())
		})
	}
}

func TestValidateInvalidConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		errMsg  string
	}{
		{
			name:   "invalid port",
			cfg:    &Config{Server: ServerConfig{Port: -1}, Pool: PoolConfig{WorkerCount: 10, RunMode: RunModeCount, Count: 100}},
			errMsg: "invalid server port",
		},
		{
			name:   "port too high",
			cfg:    &Config{Server: ServerConfig{Port: 70000}, Pool: PoolConfig{WorkerCount: 10, RunMode: RunModeCount, Count: 100}},
			errMsg: "invalid server port",
		},
		{
			name:   "zero worker count",
			cfg:    &Config{Server: ServerConfig{Port: 8080}, Pool: PoolConfig{WorkerCount: 0, RunMode: RunModeCount, Count: 100}},
			errMsg: "pool worker_count must be > 0",
		},
		{
			name:   "negative worker count",
			cfg:    &Config{Server: ServerConfig{Port: 8080}, Pool: PoolConfig{WorkerCount: -1, RunMode: RunModeCount, Count: 100}},
			errMsg: "pool worker_count must be > 0",
		},
		{
			name:   "invalid run mode",
			cfg:    &Config{Server: ServerConfig{Port: 8080}, Pool: PoolConfig{WorkerCount: 10, RunMode: "invalid"}},
			errMsg: "invalid pool run_mode",
		},
		{
			name:   "count mode with zero count",
			cfg:    &Config{Server: ServerConfig{Port: 8080}, Pool: PoolConfig{WorkerCount: 10, RunMode: RunModeCount, Count: 0}},
			errMsg: "pool count must be > 0",
		},
		{
			name:   "duration mode with zero duration",
			cfg:    &Config{Server: ServerConfig{Port: 8080}, Pool: PoolConfig{WorkerCount: 10, RunMode: RunModeDuration, Duration: 0}},
			errMsg: "pool duration must be > 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestServerAddr(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Host: "127.0.0.1", Port: 9090},
	}
	assert.Equal(t, "127.0.0.1:9090", cfg.ServerAddr())
}
