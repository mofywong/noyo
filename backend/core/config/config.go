package config

// GlobalConfig represents the root configuration structure
type GlobalConfig struct {
	Server ServerConfig `yaml:"server" json:"server"`
	TSDB   TSDBConfig   `yaml:"tsdb" json:"tsdb"`
	Log    LogConfig    `yaml:"log" json:"log"`
}

type ServerConfig struct {
	Port int `yaml:"port" json:"port"`
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
	Enabled         bool   `yaml:"enabled" json:"enabled"`
	Dir             string `yaml:"dir" json:"dir"`                               // e.g. "./data/db/history"
	RetentionDays   int    `yaml:"retention_days" json:"retention_days"`         // e.g. 90
	BatchSize       int    `yaml:"batch_size" json:"batch_size"`                 // e.g. 100
	FlushIntervalMs int    `yaml:"flush_interval_ms" json:"flush_interval_ms"` // e.g. 1000
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

