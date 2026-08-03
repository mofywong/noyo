package store

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

const (
	scopePermissionPolicyMigrationKey              = "migration.scope_permission_policy.v1"
	scopePermissionPolicyNormalizationMigrationKey = "migration.scope_permission_policy.v2"
	scopePermissionPolicySnapshotMigrationKey      = "migration.scope_permission_policy.v3"
)

// migrateLegacyScopePermissionPolicies gives legacy permission limits an
// explicit policy. A scope without any legacy limit rows was never configured,
// so it adopts the product defaults instead of becoming an empty custom scope.
func migrateLegacyScopePermissionPolicies(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := migrateLegacyScopePermissionPoliciesV1(tx); err != nil {
			return err
		}
		if err := normalizeEmptyV1CustomScopePolicies(tx); err != nil {
			return err
		}
		return normalizeFullSnapshotV1ScopePolicies(tx)
	})
}

func migrateLegacyScopePermissionPoliciesV1(tx *gorm.DB) error {
	var marker SystemConfig
	err := tx.Where("key = ?", scopePermissionPolicyMigrationKey).First(&marker).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("read migration marker: %w", err)
	}

	var tenantIDs []uint
	if err := tx.Model(&Tenant{}).Order("id ASC").Pluck("id", &tenantIDs).Error; err != nil {
		return fmt.Errorf("load tenants: %w", err)
	}
	for _, tenantID := range tenantIDs {
		mode, err := legacyScopePermissionMode(tx, "tenant", tenantID, 0)
		if err != nil {
			return err
		}
		if err := createScopePermissionPolicyIfMissing(tx, "tenant", tenantID, 0, mode); err != nil {
			return err
		}
	}

	var projects []Project
	if err := tx.Select("id", "tenant_id").Order("id ASC").Find(&projects).Error; err != nil {
		return fmt.Errorf("load projects: %w", err)
	}
	for _, project := range projects {
		mode, err := legacyScopePermissionMode(tx, "project", project.TenantID, project.ID)
		if err != nil {
			return err
		}
		if err := createScopePermissionPolicyIfMissing(tx, "project", project.TenantID, project.ID, mode); err != nil {
			return err
		}
	}

	if err := tx.Create(&SystemConfig{
		Key:   scopePermissionPolicyMigrationKey,
		Value: "completed",
	}).Error; err != nil {
		return fmt.Errorf("save migration marker: %w", err)
	}
	return nil
}

func legacyScopePermissionMode(tx *gorm.DB, scopeType string, tenantID, projectID uint) (string, error) {
	var latestLimit ScopePermissionLimit
	err := tx.Where("scope_type = ? AND tenant_id = ? AND project_id = ?", scopeType, tenantID, projectID).
		Order("created_at DESC").
		First(&latestLimit).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if scopeType == "tenant" {
			return ScopePermissionModeAll, nil
		}
		return ScopePermissionModeInherit, nil
	}
	if err != nil {
		return "", fmt.Errorf("load latest %s scope permission limit %d/%d: %w", scopeType, tenantID, projectID, err)
	}

	var missingKnownPermissionCount int64
	if err := tx.Model(&Permission{}).
		Where("permissions.created_at <= ? AND NOT EXISTS (SELECT 1 FROM scope_permission_limits WHERE scope_type = ? AND tenant_id = ? AND project_id = ? AND permission_id = permissions.id AND deleted_at IS NULL)", latestLimit.CreatedAt, scopeType, tenantID, projectID).
		Count(&missingKnownPermissionCount).Error; err != nil {
		return "", fmt.Errorf("check %s scope permission snapshot %d/%d: %w", scopeType, tenantID, projectID, err)
	}
	if missingKnownPermissionCount > 0 {
		return ScopePermissionModeCustom, nil
	}
	if scopeType == "tenant" {
		return ScopePermissionModeAll, nil
	}
	return ScopePermissionModeInherit, nil
}

// normalizeFullSnapshotV1ScopePolicies upgrades untouched policies that were
// created by v1 from a complete legacy permission snapshot. A snapshot which
// contained every permission available at its creation time represents the old
// "all permissions" behavior, even if newer modules were added afterwards.
func normalizeFullSnapshotV1ScopePolicies(tx *gorm.DB) error {
	var snapshotMarker SystemConfig
	err := tx.Where("key = ?", scopePermissionPolicySnapshotMigrationKey).First(&snapshotMarker).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("read snapshot migration marker: %w", err)
	}

	var v1Marker SystemConfig
	if err := tx.Where("key = ?", scopePermissionPolicyMigrationKey).First(&v1Marker).Error; err != nil {
		return fmt.Errorf("read v1 migration marker: %w", err)
	}

	var policies []ScopePermissionPolicy
	if err := tx.Where("mode = ? AND created_at <= ? AND updated_at <= ?", ScopePermissionModeCustom, v1Marker.CreatedAt, v1Marker.CreatedAt).
		Find(&policies).Error; err != nil {
		return fmt.Errorf("load v1 snapshot scope policies: %w", err)
	}
	for _, policy := range policies {
		mode, err := legacyScopePermissionMode(tx, policy.ScopeType, policy.TenantID, policy.ProjectID)
		if err != nil {
			return err
		}
		if mode == ScopePermissionModeCustom {
			continue
		}
		if err := tx.Model(&ScopePermissionPolicy{}).Where("id = ?", policy.ID).Update("mode", mode).Error; err != nil {
			return fmt.Errorf("normalize %s scope permission snapshot %d/%d: %w", policy.ScopeType, policy.TenantID, policy.ProjectID, err)
		}
	}

	if err := tx.Create(&SystemConfig{
		Key:   scopePermissionPolicySnapshotMigrationKey,
		Value: "completed",
	}).Error; err != nil {
		return fmt.Errorf("save snapshot migration marker: %w", err)
	}
	return nil
}

// normalizeEmptyV1CustomScopePolicies repairs policies created by the first
// version of this migration, which represented an absent legacy limit list as
// an empty custom list. Policies changed after the v1 marker are intentionally
// left untouched so an administrator can still configure a deny-all scope.
func normalizeEmptyV1CustomScopePolicies(tx *gorm.DB) error {
	var normalizationMarker SystemConfig
	err := tx.Where("key = ?", scopePermissionPolicyNormalizationMigrationKey).First(&normalizationMarker).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("read normalization migration marker: %w", err)
	}

	var v1Marker SystemConfig
	if err := tx.Where("key = ?", scopePermissionPolicyMigrationKey).First(&v1Marker).Error; err != nil {
		return fmt.Errorf("read v1 migration marker: %w", err)
	}

	var policies []ScopePermissionPolicy
	if err := tx.Where("mode = ? AND created_at <= ?", ScopePermissionModeCustom, v1Marker.CreatedAt).
		Find(&policies).Error; err != nil {
		return fmt.Errorf("load v1 custom scope policies: %w", err)
	}
	for _, policy := range policies {
		mode, err := legacyScopePermissionMode(tx, policy.ScopeType, policy.TenantID, policy.ProjectID)
		if err != nil {
			return err
		}
		if mode == ScopePermissionModeCustom {
			continue
		}
		if err := tx.Model(&ScopePermissionPolicy{}).Where("id = ?", policy.ID).Update("mode", mode).Error; err != nil {
			return fmt.Errorf("normalize %s scope permission policy %d/%d: %w", policy.ScopeType, policy.TenantID, policy.ProjectID, err)
		}
	}

	if err := tx.Create(&SystemConfig{
		Key:   scopePermissionPolicyNormalizationMigrationKey,
		Value: "completed",
	}).Error; err != nil {
		return fmt.Errorf("save normalization migration marker: %w", err)
	}
	return nil
}

func createScopePermissionPolicyIfMissing(tx *gorm.DB, scopeType string, tenantID, projectID uint, mode string) error {
	policy := ScopePermissionPolicy{
		ScopeType: scopeType,
		TenantID:  tenantID,
		ProjectID: projectID,
		Mode:      mode,
	}
	if err := tx.Where(
		"scope_type = ? AND tenant_id = ? AND project_id = ?",
		scopeType,
		tenantID,
		projectID,
	).FirstOrCreate(&policy).Error; err != nil {
		return fmt.Errorf("create %s scope permission policy %d/%d: %w", scopeType, tenantID, projectID, err)
	}
	return nil
}
