package store

import (
	"encoding/json"
	"noyo/core/config"
	"gorm.io/gorm"
)

// SystemConfig stores global configuration in the database
type SystemConfig struct {
	gorm.Model
	Key   string `gorm:"uniqueIndex"`
	Value string `gorm:"type:text"`
}

// LoadGlobalConfig loads the global configuration from the database
func LoadGlobalConfig() (*config.GlobalConfig, error) {
	var sysConfig SystemConfig
	err := DB.Where("key = ?", "global_config").First(&sysConfig).Error
	if err != nil {
		return nil, err
	}

	var cfg config.GlobalConfig
	if err := json.Unmarshal([]byte(sysConfig.Value), &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// SaveGlobalConfig saves the global configuration to the database
func SaveGlobalConfig(cfg *config.GlobalConfig) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	var sysConfig SystemConfig
	err = DB.Where("key = ?", "global_config").First(&sysConfig).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return DB.Create(&SystemConfig{
				Key:   "global_config",
				Value: string(data),
			}).Error
		}
		return err
	}

	return DB.Model(&sysConfig).Update("value", string(data)).Error
}

// GetSystemConfigValue gets a string configuration value by key from system_configs
func GetSystemConfigValue(key string) (string, error) {
	if DB == nil {
		return "", gorm.ErrInvalidDB
	}
	var sysConfig SystemConfig
	err := DB.Where("key = ?", key).First(&sysConfig).Error
	if err != nil {
		return "", err
	}
	return sysConfig.Value, nil
}

// SetSystemConfigValue sets a string configuration value by key in system_configs
func SetSystemConfigValue(key, value string) error {
	if DB == nil {
		return gorm.ErrInvalidDB
	}
	var sysConfig SystemConfig
	err := DB.Where("key = ?", key).First(&sysConfig).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return DB.Create(&SystemConfig{
				Key:   key,
				Value: value,
			}).Error
		}
		return err
	}
	return DB.Model(&sysConfig).Update("value", value).Error
}

