package store

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Product represents a type of device (e.g., "Smart Meter Model X")
type Product struct {
	gorm.Model
	Code         string `gorm:"uniqueIndex;not null" json:"code"`
	Name         string `json:"name"`
	ProtocolName string `json:"protocol_name"` // e.g. "Modbus", "OPC-UA"
	// Config stores protocol-specific product configuration (e.g. Polling Groups, TSL)
	// Stored as JSON string
	Config string `json:"config"`
}

// Device represents a physical instance
type Device struct {
	gorm.Model
	Code        string `gorm:"uniqueIndex;not null" json:"code"`
	Name        string `json:"name"`
	ProductCode string `gorm:"index;not null" json:"product_code"`
	ParentCode  string `gorm:"index" json:"parent_code"`
	Enabled     bool   `json:"enabled"`
	// Config stores device-specific connection parameters (e.g. IP, Port, SlaveID)
	// Stored as JSON string
	Config string `json:"config"`
}

// --- Data Access Methods ---

func GetProduct(code string) (*Product, error) {
	var p Product
	result := DB.Where("code = ?", code).First(&p)
	if result.Error != nil {
		return nil, result.Error
	}
	return &p, nil
}

func ListProducts(page, pageSize int) ([]Product, int64, error) {
	var products []Product
	var total int64

	db := DB.Model(&Product{})
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if page > 0 && pageSize > 0 {
		offset := (page - 1) * pageSize
		db = db.Offset(offset).Limit(pageSize)
	}

	result := db.Find(&products)
	return products, total, result.Error
}

func SaveProduct(p *Product) error {
	// Upsert based on Code? Or just Save for ID?
	// If ID is 0, Create. If ID > 0, Update.
	// But often we want to update by Code.
	var existing Product
	err := DB.Where("code = ?", p.Code).First(&existing).Error
	if err == nil {
		p.ID = existing.ID
		return DB.Save(p).Error
	}
	return DB.Create(p).Error
}

func GetDevice(code string) (*Device, error) {
	var d Device
	result := DB.Where("code = ?", code).First(&d)
	if result.Error != nil {
		return nil, result.Error
	}
	return &d, nil
}

func ListDevices(page, pageSize int) ([]Device, int64, error) {
	var devices []Device
	var total int64

	db := DB.Model(&Device{})
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if page > 0 && pageSize > 0 {
		offset := (page - 1) * pageSize
		db = db.Offset(offset).Limit(pageSize)
	}

	result := db.Find(&devices)
	return devices, total, result.Error
}

func ListDevicesByParent(parentCode string) ([]Device, error) {
	var devices []Device
	result := DB.Where("parent_code = ?", parentCode).Find(&devices)
	return devices, result.Error
}

func ListDevicesByProduct(productCode string) ([]Device, error) {
	var devices []Device
	result := DB.Where("product_code = ?", productCode).Find(&devices)
	return devices, result.Error
}

func SaveDevice(d *Device) error {
	var existing Device
	err := DB.Where("code = ?", d.Code).First(&existing).Error
	if err == nil {
		d.ID = existing.ID
		return DB.Save(d).Error
	}
	return DB.Create(d).Error
}

func DeleteDevice(code string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var device Device
		if err := tx.Where("code = ?", code).First(&device).Error; err != nil {
			return err
		}

		newCode := fmt.Sprintf("%s_del_%d", device.Code, time.Now().Unix())

		// Rename code to release unique constraint
		if err := tx.Model(&device).Update("code", newCode).Error; err != nil {
			return err
		}

		// Soft delete
		return tx.Delete(&device).Error
	})
}
