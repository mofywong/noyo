package core

import (
	"fmt"

	"noyo/core/store"

	"gorm.io/gorm"
)

const (
	permissionLimitScopeTenant  = "tenant"
	permissionLimitScopeProject = "project"
)

func replaceTenantPermissionLimit(tx *gorm.DB, tenantID uint, permissionIDs []uint) error {
	if tenantID == 0 {
		return fmt.Errorf("tenant id is required")
	}
	permissionIDs = uniquePermissionIDs(permissionIDs)
	if len(permissionIDs) == 0 {
		return fmt.Errorf("permission limit cannot be empty")
	}
	if err := validateTenantPermissionLimitIDs(tx, permissionIDs); err != nil {
		return err
	}
	return replaceScopePermissionLimit(tx, permissionLimitScopeTenant, tenantID, 0, permissionIDs)
}

func replaceProjectPermissionLimit(tx *gorm.DB, tenantID, projectID uint, permissionIDs []uint) error {
	if tenantID == 0 || projectID == 0 {
		return fmt.Errorf("tenant id and project id are required")
	}
	if !projectBelongsToTenantInTx(tx, projectID, tenantID) {
		return fmt.Errorf("project does not belong to tenant")
	}
	permissionIDs = uniquePermissionIDs(permissionIDs)
	if len(permissionIDs) == 0 {
		return fmt.Errorf("permission limit cannot be empty")
	}
	if err := validateProjectPermissionLimitIDs(tx, tenantID, permissionIDs); err != nil {
		return err
	}
	return replaceScopePermissionLimit(tx, permissionLimitScopeProject, tenantID, projectID, permissionIDs)
}

func replaceScopePermissionLimit(tx *gorm.DB, scopeType string, tenantID, projectID uint, permissionIDs []uint) error {
	if err := tx.Unscoped().
		Where("scope_type = ? AND tenant_id = ? AND project_id = ?", scopeType, tenantID, projectID).
		Delete(&store.ScopePermissionLimit{}).Error; err != nil {
		return err
	}
	for _, permissionID := range permissionIDs {
		limit := store.ScopePermissionLimit{
			ScopeType:    scopeType,
			TenantID:     tenantID,
			ProjectID:    projectID,
			PermissionID: permissionID,
		}
		if err := tx.Create(&limit).Error; err != nil {
			return err
		}
	}
	return nil
}

func loadScopePermissionLimitIDs(tx *gorm.DB, scopeType string, tenantID, projectID uint) ([]uint, error) {
	var permissionIDs []uint
	if err := tx.Model(&store.ScopePermissionLimit{}).
		Where("scope_type = ? AND tenant_id = ? AND project_id = ?", scopeType, tenantID, projectID).
		Order("permission_id asc").
		Pluck("permission_id", &permissionIDs).Error; err != nil {
		return nil, err
	}
	return permissionIDs, nil
}

func validateTenantPermissionLimitIDs(tx *gorm.DB, permissionIDs []uint) error {
	permissions, err := permissionsByIDs(tx, permissionIDs)
	if err != nil {
		return err
	}
	if len(permissions) != len(uniquePermissionIDs(permissionIDs)) {
		return fmt.Errorf("invalid permission assignment")
	}
	for _, permission := range permissions {
		if !permissionAssignableToTenantLimit(permission) {
			return fmt.Errorf("permission %s is outside tenant limit scope", permission.Code)
		}
	}
	return nil
}

func validateProjectPermissionLimitIDs(tx *gorm.DB, tenantID uint, permissionIDs []uint) error {
	permissions, err := permissionsByIDs(tx, permissionIDs)
	if err != nil {
		return err
	}
	if len(permissions) != len(uniquePermissionIDs(permissionIDs)) {
		return fmt.Errorf("invalid permission assignment")
	}
	for _, permission := range permissions {
		if !permissionIDInScopeLimit(tx, permissionLimitScopeTenant, tenantID, 0, permission.ID) {
			return fmt.Errorf("permission %s is outside tenant limit", permission.Code)
		}
	}
	return nil
}

func permissionAssignableToTenantLimit(permission store.Permission) bool {
	if permission.Module == "system" && permission.Code != "dashboard:view" {
		return false
	}
	if permission.Module == "tenant" && permission.Code != "tenant:transfer" {
		return false
	}
	return true
}

func tenantPermissionOptionQuery(tx *gorm.DB) *gorm.DB {
	return tx.Model(&store.Permission{}).
		Where("module NOT IN ? OR code IN ?", []string{"tenant", "system"}, []string{"tenant:transfer", "dashboard:view"})
}

func permissionsByIDs(tx *gorm.DB, permissionIDs []uint) ([]store.Permission, error) {
	permissionIDs = uniquePermissionIDs(permissionIDs)
	if len(permissionIDs) == 0 {
		return []store.Permission{}, nil
	}
	var permissions []store.Permission
	if err := tx.Where("id IN ?", permissionIDs).Find(&permissions).Error; err != nil {
		return nil, err
	}
	return permissions, nil
}

func uniquePermissionIDs(permissionIDs []uint) []uint {
	seen := make(map[uint]bool, len(permissionIDs))
	result := make([]uint, 0, len(permissionIDs))
	for _, permissionID := range permissionIDs {
		if permissionID == 0 || seen[permissionID] {
			continue
		}
		seen[permissionID] = true
		result = append(result, permissionID)
	}
	return result
}

func permissionIDInScopeLimit(tx *gorm.DB, scopeType string, tenantID, projectID, permissionID uint) bool {
	var count int64
	tx.Model(&store.ScopePermissionLimit{}).
		Where("scope_type = ? AND tenant_id = ? AND project_id = ? AND permission_id = ?", scopeType, tenantID, projectID, permissionID).
		Count(&count)
	return count > 0
}

func projectBelongsToTenantInTx(tx *gorm.DB, projectID, tenantID uint) bool {
	var count int64
	tx.Model(&store.Project{}).Where("id = ? AND tenant_id = ?", projectID, tenantID).Count(&count)
	return count > 0
}

func permissionWithinAssignmentLimit(tx *gorm.DB, permissionID uint, targetRole store.Role, authCtx *AuthContext) bool {
	if authCtx == nil || authCtx.TenantID == 0 {
		return false
	}
	if targetRole.ProjectID > 0 {
		return permissionIDInScopeLimit(tx, permissionLimitScopeProject, authCtx.TenantID, targetRole.ProjectID, permissionID)
	}
	return permissionIDInScopeLimit(tx, permissionLimitScopeTenant, authCtx.TenantID, 0, permissionID)
}
