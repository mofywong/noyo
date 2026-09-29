package store

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

func gatewayInstanceCode(baseCode, gatewayCode string) string {
	digest := sha256.Sum256([]byte(baseCode + "\x00" + gatewayCode))
	prefix := baseCode
	if len(prefix) > 46 {
		prefix = prefix[:46]
	}
	return prefix + "_site_" + hex.EncodeToString(digest[:6])
}

// ForkGatewayProtocolProfile atomically creates or updates a site-owned driver
// instance and moves this gateway's devices off the shared profile.
func ForkGatewayProtocolProfile(baseCode, gatewayCode string, edited *ProtocolProfile) (*ProtocolProfile, error) {
	if baseCode == "" || gatewayCode == "" || edited == nil {
		return nil, fmt.Errorf("profile and gateway codes are required")
	}
	if baseCode == "noyo-gw-default" {
		return nil, ErrGatewayIdentityEdit
	}
	var result ProtocolProfile
	err := DB.Transaction(func(tx *gorm.DB) error {
		var base ProtocolProfile
		if err := tx.Where("code = ?", baseCode).First(&base).Error; err != nil {
			return err
		}
		if base.OwnerNode != "" {
			return fmt.Errorf("profile %s is already gateway-owned", baseCode)
		}
		instanceCode := gatewayInstanceCode(baseCode, gatewayCode)
		var instance ProtocolProfile
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
			instance = ProtocolProfile{
				SyncMeta: SyncMeta{OwnerNode: gatewayCode},
				TenantID: tenantID, ProjectID: projectID, Code: instanceCode,
				Name: edited.Name, ProtocolName: edited.ProtocolName, ProductCode: edited.ProductCode,
				Description: edited.Description, Config: edited.Config,
				SourceTemplateCode: baseCode, SourceTemplateRev: base.Rev,
			}
			if err := tx.Create(&instance).Error; err != nil {
				return err
			}
		} else {
			if instance.OwnerNode != gatewayCode || instance.SourceTemplateCode != baseCode {
				return fmt.Errorf("profile instance code collision")
			}
			instance.Name, instance.ProtocolName, instance.ProductCode = edited.Name, edited.ProtocolName, edited.ProductCode
			instance.Description, instance.Config, instance.SourceTemplateRev = edited.Description, edited.Config, base.Rev
			if err := tx.Save(&instance).Error; err != nil {
				return err
			}
		}
		var devices []Device
		if err := tx.Where("protocol_profile_code = ?", baseCode).Find(&devices).Error; err != nil {
			return err
		}
		for i := range devices {
			devices[i].ProtocolProfileCode = instanceCode
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
