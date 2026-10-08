package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

const legacyJWTSecret = "noyo_default_secret_key_change_me_in_production"

// NormalizeAuth fills server-side defaults and replaces the legacy signing key.
// The caller must persist changes before accepting any requests.
func (c *GlobalConfig) NormalizeAuth() (bool, error) {
	changed := false
	if strings.TrimSpace(c.Auth.JWTSecret) == "" || c.Auth.JWTSecret == legacyJWTSecret {
		var key [32]byte
		if _, err := rand.Read(key[:]); err != nil {
			return false, err
		}
		c.Auth.JWTSecret = hex.EncodeToString(key[:])
		changed = true
	}
	for _, setting := range []struct {
		value    *int
		fallback int
	}{
		{&c.Auth.AccessTokenExpiry, 120}, {&c.Auth.RefreshTokenExpiry, 10080},
		{&c.Auth.LoginFailLockCount, 5}, {&c.Auth.LoginFailLockMin, 15},
	} {
		if *setting.value <= 0 {
			*setting.value = setting.fallback
			changed = true
		}
	}
	return changed, nil
}

// GlobalConfig represents the root configuration structure
type GlobalConfig struct {
	Server     ServerConfig     `yaml:"server" json:"server"`
	TSDB       TSDBConfig       `yaml:"tsdb" json:"tsdb"`
	Log        LogConfig        `yaml:"log" json:"log"`
	Auth       AuthConfig       `yaml:"auth" json:"auth"`
	AlarmMedia AlarmMediaConfig `yaml:"alarm_media" json:"alarm_media"`
}

type AlarmMediaConfig struct {
	MaxStorageGiB int64 `yaml:"max_storage_gib" json:"max_storage_gib"`
	RetentionDays int   `yaml:"retention_days" json:"retention_days"`
}

func DefaultAlarmMediaConfig() AlarmMediaConfig {
	return AlarmMediaConfig{MaxStorageGiB: 10, RetentionDays: 30}
}

// Normalize supplies defaults when loading configurations predating this policy.
func (c *AlarmMediaConfig) Normalize() {
	defaults := DefaultAlarmMediaConfig()
	if c.MaxStorageGiB == 0 {
		c.MaxStorageGiB = defaults.MaxStorageGiB
	}
	if c.RetentionDays == 0 {
		c.RetentionDays = defaults.RetentionDays
	}
}

func (c AlarmMediaConfig) Validate() error {
	if c.MaxStorageGiB < 1 || c.MaxStorageGiB > 1048576 {
		return fmt.Errorf("alarm recording capacity must be an integer between 1 and 1048576 GiB / 告警录像容量必须为 1 到 1048576 GiB 的整数")
	}
	if c.RetentionDays < 1 || c.RetentionDays > 36500 {
		return fmt.Errorf("alarm recording retention must be an integer between 1 and 36500 days / 告警录像保存时间必须为 1 到 36500 天的整数")
	}
	return nil
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
	Dir             string `yaml:"dir" json:"dir"`                             // e.g. "./data/db/history"
	RetentionDays   int    `yaml:"retention_days" json:"retention_days"`       // e.g. 90
	BatchSize       int    `yaml:"batch_size" json:"batch_size"`               // e.g. 100
	FlushIntervalMs int    `yaml:"flush_interval_ms" json:"flush_interval_ms"` // e.g. 1000
}

type AuthConfig struct {
	JWTSecret          string `yaml:"jwt_secret" json:"jwt_secret"`
	AccessTokenExpiry  int    `yaml:"access_token_expiry" json:"access_token_expiry"`     // 分钟，默认 120
	RefreshTokenExpiry int    `yaml:"refresh_token_expiry" json:"refresh_token_expiry"`   // 分钟，默认 10080
	LoginFailLockCount int    `yaml:"login_fail_lock_count" json:"login_fail_lock_count"` // 默认 5
	LoginFailLockMin   int    `yaml:"login_fail_lock_min" json:"login_fail_lock_min"`     // 默认 15
}

// DefaultConfig returns the default configuration
func DefaultConfig() *GlobalConfig {
	return &GlobalConfig{
		AlarmMedia: DefaultAlarmMediaConfig(),
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
		Auth: AuthConfig{
			JWTSecret:          "",    // Generated and persisted before server startup.
			AccessTokenExpiry:  120,   // 2 小时
			RefreshTokenExpiry: 10080, // 7 天
			LoginFailLockCount: 5,
			LoginFailLockMin:   15,
		},
	}
}
