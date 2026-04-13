package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// GlobalConfig represents the root configuration structure
type GlobalConfig struct {
	Server ServerConfig `yaml:"server"`
	TSDB   TSDBConfig   `yaml:"tsdb"`
	Log    LogConfig    `yaml:"log"`
}

type ServerConfig struct {
	Port int `yaml:"port"`
}

type LogConfig struct {
	Level      string `yaml:"level" json:"level"`             // "debug", "info", "warn", "error"
	Dir        string `yaml:"dir" json:"dir"`                 // "./data/logs"
	MaxDays    int    `yaml:"max_days" json:"max_days"`       // 7
	MaxBackups int    `yaml:"max_backups" json:"max_backups"` // 10
	MaxSize    int    `yaml:"max_size" json:"max_size"`       // 50
	Compress   bool   `yaml:"compress" json:"compress"`       // false
}

type TSDBConfig struct {
	Enabled         bool   `yaml:"enabled"`
	Dir             string `yaml:"dir"`               // e.g. "./data/db/history"
	RetentionDays   int    `yaml:"retention_days"`    // e.g. 90
	BatchSize       int    `yaml:"batch_size"`        // e.g. 100
	FlushIntervalMs int    `yaml:"flush_interval_ms"` // e.g. 1000
}

// DefaultConfig returns the default configuration
func DefaultConfig() *GlobalConfig {
	return &GlobalConfig{
		Server: ServerConfig{
			Port: 8989,
		},
		TSDB: TSDBConfig{
			Enabled:         true,
			Dir:             "./data/db/history",
			RetentionDays:   90,
			BatchSize:       100,
			FlushIntervalMs: 1000,
		},
		Log: LogConfig{
			Level:      "info",
			Dir:        "./data/logs",
			MaxDays:    7,
			MaxBackups: 10,
			MaxSize:    50,
			Compress:   true,
		},
	}
}

// LoadConfig loads configuration from a file, falling back to defaults if file not found
func LoadConfig(path string) (*GlobalConfig, error) {
	config := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default if file does not exist
			return config, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, err
	}

	return config, nil
}

// SaveConfig saves configuration to a file
func SaveConfig(path string, cfg *GlobalConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
