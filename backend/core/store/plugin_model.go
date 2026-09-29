package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// PluginModel represents the database structure for plugin configuration
type PluginModel struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:128;not null;uniqueIndex:idx_plugin_scope" json:"name"`
	TenantID  uint      `gorm:"index;not null;default:0;uniqueIndex:idx_plugin_scope" json:"tenant_id"`
	ProjectID uint      `gorm:"index;not null;default:0;uniqueIndex:idx_plugin_scope" json:"project_id"`
	Enabled   bool      `json:"enabled"`
	Config    string    `json:"config"` // JSON string
	Revision  int64     `gorm:"not null;default:0" json:"revision"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName overrides the table name
func (PluginModel) TableName() string {
	return "plugins"
}

type legacyPluginModel struct {
	Name      string
	TenantID  uint
	Enabled   bool
	Config    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

const pluginScopeMigrationBackupTable = "plugins_legacy_scope_migration"

func migratePluginScopeSchema() error {
	if DB == nil {
		return nil
	}
	hasPluginTable := DB.Migrator().HasTable(&PluginModel{})
	hasBackupTable := DB.Migrator().HasTable(pluginScopeMigrationBackupTable)
	if hasBackupTable {
		if err := dropPluginScopeMigrationIndexes(); err != nil {
			return err
		}
	}

	if !hasPluginTable {
		if !hasBackupTable {
			return nil
		}
		if err := DB.AutoMigrate(&PluginModel{}); err != nil {
			return err
		}
		return restorePluginScopeMigrationBackup()
	}

	if DB.Migrator().HasColumn(&PluginModel{}, "id") && DB.Migrator().HasColumn(&PluginModel{}, "project_id") {
		if hasBackupTable {
			return restorePluginScopeMigrationBackup()
		}
		return nil
	}

	var legacyRows []legacyPluginModel
	if err := DB.Table("plugins").Find(&legacyRows).Error; err != nil {
		return err
	}

	if hasBackupTable {
		if err := DB.Migrator().DropTable(pluginScopeMigrationBackupTable); err != nil {
			return err
		}
	}
	if err := DB.Migrator().RenameTable("plugins", pluginScopeMigrationBackupTable); err != nil {
		return err
	}
	if err := dropPluginScopeMigrationIndexes(); err != nil {
		return err
	}
	if err := DB.AutoMigrate(&PluginModel{}); err != nil {
		return err
	}
	return insertLegacyPluginRows(legacyRows, true)
}

func dropPluginScopeMigrationIndexes() error {
	for _, indexName := range []string{"idx_plugins_tenant_id", "idx_plugins_project_id", "idx_plugin_scope"} {
		if err := DB.Exec("DROP INDEX IF EXISTS " + indexName).Error; err != nil {
			return err
		}
	}
	return nil
}

func restorePluginScopeMigrationBackup() error {
	var legacyRows []legacyPluginModel
	if err := DB.Table(pluginScopeMigrationBackupTable).Find(&legacyRows).Error; err != nil {
		return err
	}
	return insertLegacyPluginRows(legacyRows, true)
}

func insertLegacyPluginRows(legacyRows []legacyPluginModel, dropBackup bool) error {
	now := time.Now()
	for _, legacy := range legacyRows {
		row := PluginModel{
			Name:      legacy.Name,
			TenantID:  legacy.TenantID,
			ProjectID: 0,
			Enabled:   legacy.Enabled,
			Config:    legacy.Config,
			CreatedAt: legacy.CreatedAt,
			UpdatedAt: legacy.UpdatedAt,
		}
		if row.CreatedAt.IsZero() {
			row.CreatedAt = now
		}
		if row.UpdatedAt.IsZero() {
			row.UpdatedAt = now
		}
		if err := DB.Where("name = ? AND tenant_id = ? AND project_id = ?", row.Name, row.TenantID, row.ProjectID).
			FirstOrCreate(&PluginModel{}, row).Error; err != nil {
			return err
		}
	}
	if dropBackup {
		return DB.Migrator().DropTable(pluginScopeMigrationBackupTable)
	}
	return nil
}

// GetPlugin retrieves a plugin configuration by name
func GetPlugin(name string) (*PluginModel, error) {
	return GetPluginForScope(name, 0, 0)
}

// GetPluginForScope retrieves a plugin configuration for a tenant/project scope.
func GetPluginForScope(name string, tenantID, projectID uint) (*PluginModel, error) {
	plugin, err := getPluginForExactScope(name, tenantID, projectID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if tenantID != 0 || projectID != 0 {
				return GetPluginForScope(name, 0, 0)
			}
			return nil, nil
		}
		return nil, err
	}
	return plugin, nil
}

func getPluginForExactScope(name string, tenantID, projectID uint) (*PluginModel, error) {
	var plugin PluginModel
	result := DB.Where("name = ? AND tenant_id = ? AND project_id = ?", name, tenantID, projectID).First(&plugin)
	if result.Error != nil {
		return nil, result.Error
	}
	return &plugin, nil
}

// SavePlugin saves or updates a plugin configuration
func SavePlugin(name string, enabled bool, config interface{}) error {
	return SavePluginForScope(name, 0, 0, enabled, config)
}

// SavePluginForScope saves or updates a plugin configuration for a tenant/project scope.
func SavePluginForScope(name string, tenantID, projectID uint, enabled bool, config interface{}) error {
	return savePluginForScope(DB, name, tenantID, projectID, enabled, config)
}

// SaveRemotePluginForScope persists an authenticated remote command without
// recording it as a new local configuration edit.
func SaveRemotePluginForScope(name string, tenantID, projectID uint, enabled bool, config interface{}) error {
	return savePluginForScope(DB.Set(SyncApplyingKey, true), name, tenantID, projectID, enabled, config)
}

func savePluginForScope(db *gorm.DB, name string, tenantID, projectID uint, enabled bool, config interface{}) error {
	configBytes, err := json.Marshal(config)
	if err != nil {
		return err
	}
	// Plugin parameters have a single local copy and do not create sync intents.
	return db.Transaction(func(tx *gorm.DB) error {
		var plugin PluginModel
		err := tx.Where("name = ? AND tenant_id = ? AND project_id = ?", name, tenantID, projectID).Take(&plugin).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			plugin = PluginModel{Name: name, TenantID: tenantID, ProjectID: projectID,
				Enabled: enabled, Config: string(configBytes), Revision: 1}
			if err := tx.Create(&plugin).Error; err != nil {
				return err
			}
			return nil
		}
		if err != nil {
			return err
		}
		result := tx.Model(&PluginModel{}).Where("id = ? AND revision = ?", plugin.ID, plugin.Revision).
			Updates(map[string]interface{}{
				"enabled": enabled, "config": string(configBytes), "revision": plugin.Revision + 1,
				"updated_at": time.Now(),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("plugin %s changed concurrently", name)
		}
		return nil
	})
}

// AnyScopedPluginEnabled returns true if any tenant project has enabled this plugin.
func AnyScopedPluginEnabled(name string) (bool, error) {
	var count int64
	result := DB.Model(&PluginModel{}).
		Where("name = ? AND tenant_id > 0 AND project_id > 0 AND enabled = ?", name, true).
		Count(&count)
	return count > 0, result.Error
}

// GetAllPlugins retrieves all plugin configurations
func GetAllPlugins() ([]PluginModel, error) {
	var plugins []PluginModel
	result := DB.Find(&plugins)
	return plugins, result.Error
}
