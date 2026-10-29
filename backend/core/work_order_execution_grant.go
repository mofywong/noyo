package core

import (
	"encoding/json"
	"fmt"
	"noyo/core/store"
	"strings"
	"time"
)

type WorkOrderExecutionGrantInput struct {
	PolicyVersionID uint      `json:"policy_version_id"`
	DeviceCodes     []string  `json:"device_codes"`
	ExpiresAt       time.Time `json:"expires_at"`
}

func (s *WorkOrderService) CreateWorkOrderExecutionGrant(scope WorkOrderScope, input WorkOrderExecutionGrantInput) (*store.WorkOrderExecutionGrant, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	if scope.ActorUserID == 0 || input.PolicyVersionID == 0 || len(input.DeviceCodes) == 0 || len(input.DeviceCodes) > 200 || !input.ExpiresAt.After(time.Now()) || input.ExpiresAt.After(time.Now().Add(366*24*time.Hour)) {
		return nil, fmt.Errorf("author, policy, devices and an expiry within one year are required")
	}
	if _, _, err := s.loadWorkOrderPolicy(s.db, scope, input.PolicyVersionID); err != nil {
		return nil, err
	}
	auth, err := ResolveUserAuthContext(scope.ActorUserID, scope.TenantID, scope.ProjectID)
	if err != nil {
		return nil, err
	}
	if !auth.HasProjectPermission("inspection:order", scope.ProjectID) || !auth.HasProjectPermission("work_order:create", scope.ProjectID) {
		return nil, fmt.Errorf("execution authorizer lacks required permissions")
	}
	for _, code := range input.DeviceCodes {
		if !CanInspectDeviceAsUser(auth, scope.TenantID, scope.ProjectID, code) {
			return nil, fmt.Errorf("execution device is outside authorizer access")
		}
	}
	raw, _ := json.Marshal(input.DeviceCodes)
	grant := store.WorkOrderExecutionGrant{TenantID: scope.TenantID, ProjectID: scope.ProjectID, PolicyVersionID: input.PolicyVersionID, AuthorizedBy: scope.ActorUserID, DeviceCodes: string(raw), ExpiresAt: input.ExpiresAt}
	return &grant, s.db.Create(&grant).Error
}

func (s *WorkOrderService) ListWorkOrderExecutionGrants(scope WorkOrderScope) ([]store.WorkOrderExecutionGrant, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	rows := []store.WorkOrderExecutionGrant{}
	err := policyScope(s.db, scope).Order("id DESC").Limit(200).Find(&rows).Error
	return rows, err
}

func (s *WorkOrderService) RevokeWorkOrderExecutionGrant(scope WorkOrderScope, id uint, reason string) error {
	if err := validateWorkOrderScope(scope); err != nil {
		return err
	}
	reason = strings.TrimSpace(reason)
	if scope.ActorUserID == 0 || reason == "" || len([]rune(reason)) > 2000 {
		return fmt.Errorf("actor and revocation reason are required")
	}
	changed := policyScope(s.db.Model(&store.WorkOrderExecutionGrant{}), scope).Where("id = ? AND revoked_at IS NULL", id).Updates(map[string]any{"revoked_at": time.Now().UTC(), "revoke_reason": reason, "revoked_by": scope.ActorUserID})
	if changed.Error != nil {
		return changed.Error
	}
	if changed.RowsAffected != 1 {
		return ErrWorkOrderVersionConflict
	}
	return nil
}

func (s *WorkOrderService) ValidateWorkOrderExecutionGrant(scope WorkOrderScope, id, policyVersionID uint, deviceCode string) (*store.WorkOrderExecutionGrant, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	var grant store.WorkOrderExecutionGrant
	if id == 0 || policyVersionID == 0 || deviceCode == "" {
		return nil, fmt.Errorf("execution grant, policy and device are required")
	}
	if err := policyScope(s.db, scope).First(&grant, id).Error; err != nil {
		return nil, err
	}
	if grant.RevokedAt != nil || !grant.ExpiresAt.After(time.Now()) || grant.PolicyVersionID != policyVersionID {
		return nil, fmt.Errorf("execution grant is revoked, expired or does not match the policy")
	}
	var devices []string
	if err := json.Unmarshal([]byte(grant.DeviceCodes), &devices); err != nil {
		return nil, err
	}
	if !containsPolicyValue(devices, deviceCode) {
		return nil, fmt.Errorf("device is outside execution grant")
	}
	auth, err := ResolveUserAuthContext(grant.AuthorizedBy, scope.TenantID, scope.ProjectID)
	if err != nil {
		return nil, err
	}
	if !auth.HasProjectPermission("inspection:order", scope.ProjectID) || !auth.HasProjectPermission("work_order:create", scope.ProjectID) || !CanInspectDeviceAsUser(auth, scope.TenantID, scope.ProjectID, deviceCode) {
		return nil, fmt.Errorf("execution authorizer access was revoked")
	}
	if _, _, err := s.loadWorkOrderPolicy(s.db, scope, policyVersionID); err != nil {
		return nil, err
	}
	return &grant, nil
}

func (s *WorkOrderService) GetWorkOrderPolicyDefinition(scope WorkOrderScope, id uint) (WorkOrderPolicyDefinition, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return WorkOrderPolicyDefinition{}, err
	}
	_, def, err := s.loadWorkOrderPolicy(s.db, scope, id)
	return def, err
}
