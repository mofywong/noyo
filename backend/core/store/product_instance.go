package store

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// ForkGatewayProduct turns a local edit to a shared product into one gateway's
// instance and rebinds only devices in the gateway's database. The product and
// each device change are recorded by the write callbacks in this transaction.
func ForkGatewayProduct(baseCode, gatewayCode string, edited *Product) (*Product, error) {
	if baseCode == "" || gatewayCode == "" || edited == nil {
		return nil, fmt.Errorf("product and gateway codes are required")
	}
	if baseCode == "noyo-gw" {
		return nil, ErrGatewayIdentityEdit
	}
	var result Product
	err := DB.Transaction(func(tx *gorm.DB) error {
		var base Product
		if err := tx.Where("code = ?", baseCode).First(&base).Error; err != nil {
			return err
		}
		if base.OwnerNode != "" {
			return fmt.Errorf("product %s is already gateway-owned", baseCode)
		}
		instanceCode := gatewayInstanceCode(baseCode, gatewayCode)
		var instance Product
		err := tx.Where("code = ?", instanceCode).First(&instance).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			tenantID, projectID := base.TenantID, base.ProjectID
			if tenantID == 0 {
				tenantID = edited.TenantID
			}
			if projectID == 0 {
				projectID = edited.ProjectID
			}
			instance = Product{
				SyncMeta: SyncMeta{OwnerNode: gatewayCode},
				TenantID: tenantID, ProjectID: projectID, Code: instanceCode,
				Name: edited.Name, Config: edited.Config,
				SourceTemplateCode: baseCode, SourceTemplateRev: base.Rev,
			}
			if err := tx.Create(&instance).Error; err != nil {
				return err
			}
		} else {
			if instance.OwnerNode != gatewayCode || instance.SourceTemplateCode != baseCode {
				return fmt.Errorf("product instance code collision")
			}
			instance.Name, instance.Config, instance.SourceTemplateRev = edited.Name, edited.Config, base.Rev
			if err := tx.Save(&instance).Error; err != nil {
				return err
			}
		}
		var devices []Device
		if err := tx.Where("product_code = ?", baseCode).Find(&devices).Error; err != nil {
			return err
		}
		for i := range devices {
			devices[i].ProductCode = instanceCode
			if err := tx.Save(&devices[i]).Error; err != nil {
				return err
			}
		}
		result = instance
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}
