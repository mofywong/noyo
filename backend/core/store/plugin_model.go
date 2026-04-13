package store

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// PluginModel represents the database structure for plugin configuration
type PluginModel struct {
	Name      string    `gorm:"primaryKey" json:"name"`
	Enabled   bool      `json:"enabled"`
	Config    string    `json:"config"` // JSON string
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName overrides the table name
func (PluginModel) TableName() string {
	return "plugins"
}

// GetPlugin retrieves a plugin configuration by name
func GetPlugin(name string) (*PluginModel, error) {
	var plugin PluginModel
	result := DB.First(&plugin, "name = ?", name)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &plugin, nil
}

// SavePlugin saves or updates a plugin configuration
func SavePlugin(name string, enabled bool, config interface{}) error {
	configBytes, err := json.Marshal(config)
	if err != nil {
		return err
	}

	plugin := PluginModel{
		Name:    name,
		Enabled: enabled,
		Config:  string(configBytes),
	}

	// Use Save (Upsert)
	return DB.Save(&plugin).Error
}

// GetAllPlugins retrieves all plugin configurations
func GetAllPlugins() ([]PluginModel, error) {
	var plugins []PluginModel
	result := DB.Find(&plugins)
	return plugins, result.Error
}
