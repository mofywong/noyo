package cascade

import (
	"encoding/json"
	"fmt"

	"noyo/core/store"

	"gorm.io/gorm"
)

// A gateway may publish only resources in the scope of its registered gateway
// device. The MQTT topic proves the routed sender, but tenant/project and owner
// fields inside a change are untrusted. Validate them before touching the mirror
// or relaying the change to other gateways.
func (e *platformEngineImpl) authorizeGatewayChange(tx *gorm.DB, ch store.SyncChange) (bool, error) {
	var gateway store.Device
	if err := tx.Where("code = ? AND product_code = ?", ch.OriginNode, defaultGatewayProductCode).Take(&gateway).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, err
	}
	if ch.TenantID != gateway.TenantID || ch.ProjectID != gateway.ProjectID {
		return false, nil
	}
	if ch.EntityType == store.SyncEntityPluginConfig || ch.EntityType == store.SyncEntityNodeConfig {
		// Node/plugin configuration has a gateway-qualified business code and
		// cannot borrow a registered gateway's scope to address another node.
		var value store.ProtectedConfig
		if ch.Op != store.SyncOpUpsert || json.Unmarshal([]byte(ch.Payload), &value) != nil || value.Validate() != nil {
			return false, nil
		}
		if value.Code != ch.EntityCode || value.GatewaySN != ch.OriginNode || value.TenantID != gateway.TenantID || value.ProjectID != gateway.ProjectID || value.Revision != ch.NewRev || protectedConfigEntity(value) != ch.EntityType {
			return false, nil
		}
		return true, nil
	}

	var model interface{}
	switch ch.EntityType {
	case store.SyncEntityDevice:
		model = &store.Device{}
	case store.SyncEntityProduct:
		model = &store.Product{}
	case store.SyncEntityDriver:
		model = &store.ProtocolProfile{}
	case store.SyncEntityRule:
		model = &store.Rule{}
	case store.SyncEntityRuleGroup:
		model = &store.RuleGroup{}
	default:
		return false, nil
	}
	var wrongScope int64
	if err := tx.Unscoped().Model(model).Where("code = ? AND (tenant_id <> ? OR project_id <> ?)", ch.EntityCode, gateway.TenantID, gateway.ProjectID).Count(&wrongScope).Error; err != nil {
		return false, err
	}
	if wrongScope != 0 {
		return false, nil
	}
	if ch.EntityType == store.SyncEntityDevice || ch.EntityType == store.SyncEntityProduct || ch.EntityType == store.SyncEntityDriver {
		var foreignOwner int64
		if err := tx.Unscoped().Model(model).Where("code = ? AND owner_node <> '' AND owner_node <> ?", ch.EntityCode, ch.OriginNode).Count(&foreignOwner).Error; err != nil {
			return false, err
		}
		if foreignOwner != 0 {
			return false, nil
		}
	}
	if ch.EntityType == store.SyncEntityRule {
		var foreignRule int64
		if err := tx.Model(model).Where("code = ? AND gateway_sn <> '' AND gateway_sn <> ?", ch.EntityCode, ch.OriginNode).Count(&foreignRule).Error; err != nil {
			return false, err
		}
		if foreignRule != 0 {
			return false, nil
		}
	}
	if ch.Op == store.SyncOpDelete {
		return true, nil
	}
	if ch.Op != store.SyncOpUpsert || ch.Payload == "" {
		return false, nil
	}
	var payload struct {
		Code      string `json:"code"`
		TenantID  *uint  `json:"tenant_id"`
		ProjectID *uint  `json:"project_id"`
		OwnerNode string `json:"owner_node"`
		GatewaySN string `json:"gateway_sn"`
	}
	if err := json.Unmarshal([]byte(ch.Payload), &payload); err != nil {
		return false, fmt.Errorf("decode gateway resource scope: %w", err)
	}
	if payload.Code != ch.EntityCode || payload.TenantID == nil || payload.ProjectID == nil ||
		*payload.TenantID != gateway.TenantID || *payload.ProjectID != gateway.ProjectID {
		return false, nil
	}
	if payload.OwnerNode != "" && payload.OwnerNode != ch.OriginNode {
		return false, nil
	}
	if ch.EntityType == store.SyncEntityRule && payload.GatewaySN != "" && payload.GatewaySN != ch.OriginNode {
		return false, nil
	}
	return true, nil
}
