package core

import (
	"errors"
	"fmt"

	"noyo/core/store"

	"gorm.io/gorm"
)

const (
	permissionLimitScopeTenant  = "tenant"
	permissionLimitScopeProject = "project"
)

func normalizeTenantPermissionMode(mode string) (string, error) {
	if mode == "" {
		return store.ScopePermissionModeAll, nil
	}
	if mode == store.ScopePermissionModeAll || mode == store.ScopePermissionModeCustom {
		return mode, nil
	}
	return "", fmt.Errorf("invalid tenant permission mode")
}

func normalizeProjectPermissionMode(mode string) (string, error) {
	if mode == "" {
		return store.ScopePermissionModeInherit, nil
	}
	if mode == store.ScopePermissionModeAll || mode == store.ScopePermissionModeCustom || mode == store.ScopePermissionModeInherit {
		return mode, nil
	}
	return "", fmt.Errorf("invalid project permission mode")
}

func replaceTenantPermissionLimit(tx *gorm.DB, tenantID uint, permissionIDs []uint) error {
	return replaceTenantPermissionPolicy(tx, tenantID, store.ScopePermissionModeCustom, permissionIDs)
}

func replaceTenantPermissionPolicy(tx *gorm.DB, tenantID uint, mode string, permissionIDs []uint) error {
	if tenantID == 0 {
		return fmt.Errorf("tenant id is required")
	}
	mode, err := normalizeTenantPermissionMode(mode)
	if err != nil {
		return err
	}
	permissionIDs = uniquePermissionIDs(permissionIDs)
	if mode == store.ScopePermissionModeCustom {
		if err := validateTenantPermissionLimitIDs(tx, permissionIDs); err != nil {
			return err
		}
	} else if len(permissionIDs) > 0 {
		return fmt.Errorf("permission ids are only supported in custom mode")
	}
	return replaceScopePermissionPolicy(tx, permissionLimitScopeTenant, tenantID, 0, mode, permissionIDs)
}

func replaceProjectPermissionLimit(tx *gorm.DB, tenantID, projectID uint, permissionIDs []uint) error {
	return replaceProjectPermissionPolicy(tx, tenantID, projectID, store.ScopePermissionModeCustom, permissionIDs)
}

func replaceProjectPermissionPolicy(tx *gorm.DB, tenantID, projectID uint, mode string, permissionIDs []uint) error {
	if tenantID == 0 || projectID == 0 {
		return fmt.Errorf("tenant id and project id are required")
	}
	if !projectBelongsToTenantInTx(tx, projectID, tenantID) {
		return fmt.Errorf("project does not belong to tenant")
	}
	mode, err := normalizeProjectPermissionMode(mode)
	if err != nil {
		return err
	}
	permissionIDs = uniquePermissionIDs(permissionIDs)
	if mode == store.ScopePermissionModeCustom {
		if err := validateProjectPermissionLimitIDs(tx, tenantID, permissionIDs); err != nil {
			return err
		}
	} else if len(permissionIDs) > 0 {
		return fmt.Errorf("permission ids are only supported in custom mode")
	}
	return replaceScopePermissionPolicy(tx, permissionLimitScopeProject, tenantID, projectID, mode, permissionIDs)
}

// replaceScopePermissionLimit keeps legacy callers explicit: a permission list
// always means a custom policy.
func replaceScopePermissionLimit(tx *gorm.DB, scopeType string, tenantID, projectID uint, permissionIDs []uint) error {
	return replaceScopePermissionPolicy(tx, scopeType, tenantID, projectID, store.ScopePermissionModeCustom, permissionIDs)
}

func replaceScopePermissionPolicy(tx *gorm.DB, scopeType string, tenantID, projectID uint, mode string, permissionIDs []uint) error {
	if scopeType != permissionLimitScopeTenant && scopeType != permissionLimitScopeProject {
		return fmt.Errorf("invalid permission scope type")
	}
	if scopeType == permissionLimitScopeTenant && mode == store.ScopePermissionModeInherit {
		return fmt.Errorf("tenant permission policy cannot inherit")
	}

	if err := tx.Unscoped().
		Where("scope_type = ? AND tenant_id = ? AND project_id = ?", scopeType, tenantID, projectID).
		Delete(&store.ScopePermissionLimit{}).Error; err != nil {
		return err
	}
	if mode == store.ScopePermissionModeCustom {
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
	}

	var policy store.ScopePermissionPolicy
	err := tx.Where("scope_type = ? AND tenant_id = ? AND project_id = ?", scopeType, tenantID, projectID).First(&policy).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return tx.Create(&store.ScopePermissionPolicy{
			ScopeType: scopeType,
			TenantID:  tenantID,
			ProjectID: projectID,
			Mode:      mode,
		}).Error
	}
	if err != nil {
		return err
	}
	return tx.Model(&policy).Update("mode", mode).Error
}

func scopePermissionMode(tx *gorm.DB, scopeType string, tenantID, projectID uint) (string, error) {
	var policy store.ScopePermissionPolicy
	err := tx.Where("scope_type = ? AND tenant_id = ? AND project_id = ?", scopeType, tenantID, projectID).First(&policy).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Before policy support, a missing or empty list meant no permissions.
		return store.ScopePermissionModeCustom, nil
	}
	if err != nil {
		return "", err
	}
	return policy.Mode, nil
}

func loadScopePermissionLimitIDs(tx *gorm.DB, scopeType string, tenantID, projectID uint) ([]uint, error) {
	permissionIDs := make([]uint, 0)
	if err := tx.Model(&store.ScopePermissionLimit{}).
		Where("scope_type = ? AND tenant_id = ? AND project_id = ?", scopeType, tenantID, projectID).
		Order("permission_id asc").
		Pluck("permission_id", &permissionIDs).Error; err != nil {
		return nil, err
	}
	return permissionIDs, nil
}

func tenantPermissionOptionsQuery(tx *gorm.DB, tenantID uint) (*gorm.DB, error) {
	mode, err := scopePermissionMode(tx, permissionLimitScopeTenant, tenantID, 0)
	if err != nil {
		return nil, err
	}
	if mode == store.ScopePermissionModeAll {
		return tenantPermissionOptionQuery(tx), nil
	}
	return tx.Model(&store.Permission{}).Where(
		"id IN (?)",
		tx.Model(&store.ScopePermissionLimit{}).
			Select("permission_id").
			Where("scope_type = ? AND tenant_id = ? AND project_id = ?", permissionLimitScopeTenant, tenantID, 0),
	), nil
}

func projectPermissionOptionsQuery(tx *gorm.DB, tenantID, projectID uint) (*gorm.DB, error) {
	mode, err := scopePermissionMode(tx, permissionLimitScopeProject, tenantID, projectID)
	if err != nil {
		return nil, err
	}
	if mode != store.ScopePermissionModeCustom {
		return tenantPermissionOptionsQuery(tx, tenantID)
	}
	return tx.Model(&store.Permission{}).Where(
		"id IN (?)",
		tx.Model(&store.ScopePermissionLimit{}).
			Select("permission_id").
			Where("scope_type = ? AND tenant_id = ? AND project_id = ?", permissionLimitScopeProject, tenantID, projectID),
	), nil
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
		allowed, err := permissionAllowedByScopePolicy(tx, permissionLimitScopeTenant, tenantID, 0, permission.ID)
		if err != nil {
			return err
		}
		if !allowed {
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

func permissionAllowedByScopePolicy(tx *gorm.DB, scopeType string, tenantID, projectID, permissionID uint) (bool, error) {
	mode, err := scopePermissionMode(tx, scopeType, tenantID, projectID)
	if err != nil {
		return false, err
	}
	switch mode {
	case store.ScopePermissionModeCustom:
		return permissionIDInScopeLimit(tx, scopeType, tenantID, projectID, permissionID), nil
	case store.ScopePermissionModeAll:
		if scopeType == permissionLimitScopeProject {
			return permissionAllowedByScopePolicy(tx, permissionLimitScopeTenant, tenantID, 0, permissionID)
		}
		var permission store.Permission
		if err := tx.First(&permission, permissionID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return false, nil
			}
			return false, err
		}
		return permissionAssignableToTenantLimit(permission), nil
	case store.ScopePermissionModeInherit:
		if scopeType != permissionLimitScopeProject {
			return false, fmt.Errorf("tenant permission policy cannot inherit")
		}
		return permissionAllowedByScopePolicy(tx, permissionLimitScopeTenant, tenantID, 0, permissionID)
	default:
		return false, fmt.Errorf("invalid permission policy mode")
	}
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

	state, err := store.LoadSetupState()
	if err == nil && state != nil && IsSingleProjectSetupMode(state.Mode) {
		return true
	}

	if targetRole.ProjectID > 0 {
		allowed, err := permissionAllowedByScopePolicy(tx, permissionLimitScopeProject, authCtx.TenantID, targetRole.ProjectID, permissionID)
		return err == nil && allowed
	}
	allowed, err := permissionAllowedByScopePolicy(tx, permissionLimitScopeTenant, authCtx.TenantID, 0, permissionID)
	return err == nil && allowed
}
