package core

import (
	"noyo/core/store"
	"strconv"

	"gorm.io/gorm"
)

func deleteProjectCascade(tx *gorm.DB, project store.Project) error {
	if project.ID == 0 {
		return nil
	}

	var roleIDs []uint
	if err := tx.Model(&store.Role{}).
		Where("tenant_id = ? AND project_id = ?", project.TenantID, project.ID).
		Pluck("id", &roleIDs).Error; err != nil {
		return err
	}
	if len(roleIDs) > 0 {
		if err := tx.Unscoped().Where("role_id IN ?", roleIDs).Delete(&store.RolePermission{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("role_id IN ?", roleIDs).Delete(&store.RoleDeviceTagPermission{}).Error; err != nil {
			return err
		}
	}
	if err := tx.Unscoped().Where("project_id = ?", project.ID).Delete(&store.RoleDeviceTagPermission{}).Error; err != nil {
		return err
	}
	var deviceCodes []string
	if err := tx.Model(&store.Device{}).
		Where("tenant_id = ? AND project_id = ?", project.TenantID, project.ID).
		Pluck("code", &deviceCodes).Error; err != nil {
		return err
	}
	if len(deviceCodes) > 0 {
		scopeID := strconv.FormatUint(uint64(project.TenantID), 10)
		if err := tx.Unscoped().
			Where("scope_type = ? AND scope_id = ? AND device_code IN ?", "tenant", scopeID, deviceCodes).
			Delete(&store.DeviceTagBinding{}).Error; err != nil {
			return err
		}
	}

	if err := tx.Unscoped().Where("scope_type = ? AND tenant_id = ? AND project_id = ?", permissionLimitScopeProject, project.TenantID, project.ID).Delete(&store.ScopePermissionLimit{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("tenant_id = ? AND project_id = ?", project.TenantID, project.ID).Delete(&store.UserRoleBinding{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("tenant_id = ? AND project_id = ?", project.TenantID, project.ID).Delete(&store.AppRole{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("tenant_id = ? AND project_id = ?", project.TenantID, project.ID).Delete(&store.AppProjectAccess{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("tenant_id = ? AND project_id = ?", project.TenantID, project.ID).Delete(&store.AppPermission{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("tenant_id = ? AND project_id = ?", project.TenantID, project.ID).Delete(&store.AppDeviceTagPermission{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("tenant_id = ? AND project_id = ?", project.TenantID, project.ID).Delete(&store.Role{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("tenant_id = ? AND project_id = ?", project.TenantID, project.ID).Delete(&store.Device{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("tenant_id = ? AND project_id = ?", project.TenantID, project.ID).Delete(&store.Product{}).Error; err != nil {
		return err
	}
	return tx.Unscoped().Delete(&store.Project{}, project.ID).Error
}

func deleteTenantCascade(tx *gorm.DB, tenantID uint) error {
	if tenantID == 0 {
		return nil
	}

	var projects []store.Project
	if err := tx.Where("tenant_id = ?", tenantID).Find(&projects).Error; err != nil {
		return err
	}
	for _, project := range projects {
		if err := deleteProjectCascade(tx, project); err != nil {
			return err
		}
	}

	var roleIDs []uint
	if err := tx.Model(&store.Role{}).Where("tenant_id = ?", tenantID).Pluck("id", &roleIDs).Error; err != nil {
		return err
	}
	if len(roleIDs) > 0 {
		if err := tx.Unscoped().Where("role_id IN ?", roleIDs).Delete(&store.RolePermission{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("role_id IN ?", roleIDs).Delete(&store.RoleDeviceTagPermission{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("role_id IN ?", roleIDs).Delete(&store.UserRoleBinding{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("role_id IN ?", roleIDs).Delete(&store.AppRole{}).Error; err != nil {
			return err
		}
	}

	var appIDs []uint
	if err := tx.Model(&store.App{}).Where("tenant_id = ?", tenantID).Pluck("id", &appIDs).Error; err != nil {
		return err
	}
	if len(appIDs) > 0 {
		if err := tx.Unscoped().Where("app_id IN ?", appIDs).Delete(&store.AppRole{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("app_id IN ?", appIDs).Delete(&store.AppProjectAccess{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("app_id IN ?", appIDs).Delete(&store.AppPermission{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("app_id IN ?", appIDs).Delete(&store.AppDeviceTagPermission{}).Error; err != nil {
			return err
		}
	}

	scopeID := strconv.FormatUint(uint64(tenantID), 10)
	if err := tx.Unscoped().Where("scope_type = ? AND scope_id = ?", "tenant", scopeID).Delete(&store.DeviceTagBinding{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("scope_type = ? AND scope_id = ?", "tenant", scopeID).Delete(&store.DeviceTag{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("tenant_id = ?", tenantID).Delete(&store.UserRoleBinding{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("tenant_id = ?", tenantID).Delete(&store.ScopePermissionLimit{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("tenant_id = ?", tenantID).Delete(&store.AuditLog{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("tenant_id = ?", tenantID).Delete(&store.AppRole{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("tenant_id = ?", tenantID).Delete(&store.AppProjectAccess{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("tenant_id = ?", tenantID).Delete(&store.AppPermission{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("tenant_id = ?", tenantID).Delete(&store.AppDeviceTagPermission{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("tenant_id = ?", tenantID).Delete(&store.App{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("tenant_id = ?", tenantID).Delete(&store.Role{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("tenant_id = ?", tenantID).Delete(&store.User{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("tenant_id = ?", tenantID).Delete(&store.Device{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("tenant_id = ?", tenantID).Delete(&store.Product{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("tenant_id = ?", tenantID).Delete(&store.Project{}).Error; err != nil {
		return err
	}
	return tx.Unscoped().Delete(&store.Tenant{}, tenantID).Error
}
