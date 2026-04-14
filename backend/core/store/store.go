package store

import (
	"fmt"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// DB Global database instance
var DB *gorm.DB

// InitDB initializes the database
func InitDB(dsn string) error {
	var err error
	// Use pure go sqlite
	DB, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %w", err)
	}
	sqlDB.SetMaxIdleConns(10)           // Max idle connections
	sqlDB.SetMaxOpenConns(100)          // Max open connections
	sqlDB.SetConnMaxLifetime(time.Hour) // Connection max lifetime

	// AutoMigrate can be added here if we have models
	err = DB.AutoMigrate(&PluginModel{}, &Product{}, &Device{}, &SystemConfig{})
	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	return nil
}

// CloseDB closes the database connection (if needed, though GORM manages pool)
func CloseDB() {
	sqlDB, err := DB.DB()
	if err != nil {
		return
	}
	sqlDB.Close()
}

// --- Product CRUD Helper ---

func UpdateProduct(p *Product) error {
	return DB.Model(&Product{}).Where("code = ?", p.Code).Updates(p).Error
}

func DeleteProduct(code string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var product Product
		if err := tx.Where("code = ?", code).First(&product).Error; err != nil {
			return err
		}

		newCode := fmt.Sprintf("%s_del_%d", product.Code, time.Now().Unix())

		// Rename code to release unique constraint
		if err := tx.Model(&product).Update("code", newCode).Error; err != nil {
			return err
		}

		// Soft delete
		return tx.Delete(&product).Error
	})
}

// --- Device CRUD Helper ---

// UpdateDevice updates non-zero fields of a device
func UpdateDevice(d *Device) error {
	return DB.Model(&Device{}).Where("code = ?", d.Code).Updates(d).Error
}
