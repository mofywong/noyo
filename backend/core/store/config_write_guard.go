package store

import (
	"errors"
	"fmt"
	"sync"

	"gorm.io/gorm"
)

var (
	ErrGatewayReadOnly     = errors.New("gateway configuration is read-only outside local maintenance")
	ErrGatewayMaintenance  = errors.New("gateway is in local maintenance; remote configuration edits are blocked")
	ErrSharedTemplateEdit  = errors.New("shared template must be forked before gateway-local editing")
	ErrConfigResourceInUse = errors.New("configuration resource is still referenced by devices")
	ErrGatewayIdentityEdit = errors.New("gateway identity template cannot be forked")
	configGuardIdentityMu  sync.RWMutex
	configGuardRole        string
	configGuardGatewayCode string
)

const automaticConfigWriteKey = "noyo:automatic_config_write"

func LocalConfigGatewayCode() string {
	configGuardIdentityMu.RLock()
	defer configGuardIdentityMu.RUnlock()
	if configGuardRole != "gateway" {
		return ""
	}
	return configGuardGatewayCode
}

// RegisterConfigWriteGuard is installed by Cascade on the active database. It
// protects product, driver and device edits from HTTP and imports. Protocol registration and
// system initialization explicitly mark automatic writes, which still record
// ordinary sync changes. Replayed commits carry SyncApplyingKey.
func RegisterConfigWriteGuard(role, gatewayCode string) error {
	configGuardIdentityMu.Lock()
	configGuardRole, configGuardGatewayCode = role, gatewayCode
	configGuardIdentityMu.Unlock()
	guard := func(mutatingExisting bool) func(*gorm.DB) {
		return func(db *gorm.DB) {
			if db.Error != nil || db.Statement == nil || db.Statement.Schema == nil {
				return
			}
			if _, applying := db.Get(SyncApplyingKey); applying {
				return
			}
			if automatic, _ := db.Get(automaticConfigWriteKey); automatic == true {
				return
			}
			kind := db.Statement.Schema.Name
			if !protectedConfigKind(kind) || bookkeepingUpdate(db.Statement.Dest) {
				return
			}
			if role == "gateway" {
				state, err := getGatewayMaintenance(db, gatewayCode)
				if err != nil {
					db.AddError(err)
				} else if state.Mode != GatewayModeMaintenance {
					db.AddError(ErrGatewayReadOnly)
				} else if mutatingExisting && (kind == "Product" || kind == "ProtocolProfile") {
					if err := rejectSharedTemplateWrite(db, kind); err != nil {
						db.AddError(err)
					}
				}
				return
			}
			if role != "platform" {
				return
			}
			targets, err := targetGatewayCodes(db, kind)
			if err != nil {
				db.AddError(err)
				return
			}
			for _, code := range targets {
				state, err := getGatewayMaintenance(db, code)
				if err != nil {
					db.AddError(err)
					return
				}
				if state.Mode != GatewayModeNormal {
					db.AddError(fmt.Errorf("%w: %s", ErrGatewayMaintenance, code))
					return
				}
			}
		}
	}
	if err := DB.Callback().Create().Before("gorm:create").Replace("cascade:config_write_guard", guard(false)); err != nil {
		return err
	}
	if err := DB.Callback().Update().Before("gorm:update").Replace("cascade:config_write_guard", guard(true)); err != nil {
		return err
	}
	return DB.Callback().Delete().Before("gorm:delete").Replace("cascade:config_write_guard", guard(true))
}

func rejectSharedTemplateWrite(db *gorm.DB, kind string) error {
	var code string
	switch v := db.Statement.Model.(type) {
	case *Product:
		code = v.Code
	case *ProtocolProfile:
		code = v.Code
	}
	if code == "" {
		switch v := db.Statement.Dest.(type) {
		case *Product:
			code = v.Code
		case *ProtocolProfile:
			code = v.Code
		}
	}
	if code == "" {
		return nil
	}
	var owner string
	query := db.Session(&gorm.Session{NewDB: true})
	if kind == "Product" {
		if err := query.Model(&Product{}).Where("code = ?", code).Select("owner_node").Scan(&owner).Error; err != nil {
			return err
		}
	} else {
		if err := query.Model(&ProtocolProfile{}).Where("code = ?", code).Select("owner_node").Scan(&owner).Error; err != nil {
			return err
		}
	}
	if owner == "" {
		return fmt.Errorf("%w: %s", ErrSharedTemplateEdit, code)
	}
	return nil
}

func protectedConfigKind(kind string) bool {
	switch kind {
	case "Device", "Product", "ProtocolProfile":
		return true
	default:
		return false
	}
}

func bookkeepingUpdate(value interface{}) bool {
	columns, ok := value.(map[string]interface{})
	if !ok || len(columns) == 0 {
		return false
	}
	for column := range columns {
		switch column {
		case "rev", "sync_state", "sync_writer", "sync_error", "conflict_payload", "conflict_node", "conflict_at", "updated_at", "last_triggered_at", "trigger_count", "error_message", "last_synced_at":
		default:
			return false
		}
	}
	return true
}

func targetGatewayCodes(db *gorm.DB, kind string) ([]string, error) {
	add := func(codes []string, code string) []string {
		if code == "" {
			return codes
		}
		for _, existing := range codes {
			if existing == code {
				return codes
			}
		}
		return append(codes, code)
	}
	var codes []string
	switch value := db.Statement.Dest.(type) {
	case *Device:
		codes = add(codes, value.OwnerNode)
		if len(codes) == 0 {
			gateway, err := gatewayAncestorCode(db, value.ParentCode)
			if err != nil {
				return nil, err
			}
			codes = add(codes, gateway)
		}
		if len(codes) == 0 && value.ProductCode == "noyo-gw" {
			codes = add(codes, value.Code)
		}
	case *Rule:
		codes = add(codes, value.GatewaySN)
	case *Product:
		codes = add(codes, value.OwnerNode)
	case *ProtocolProfile:
		codes = add(codes, value.OwnerNode)
	case *GatewayPluginStateModel:
		codes = add(codes, value.GatewaySN)
	}
	if len(codes) == 0 {
		switch value := db.Statement.Model.(type) {
		case *Device:
			codes = add(codes, value.OwnerNode)
			if len(codes) == 0 {
				gateway, err := gatewayAncestorCode(db, value.ParentCode)
				if err != nil {
					return nil, err
				}
				codes = add(codes, gateway)
			}
		case *Product:
			codes = add(codes, value.OwnerNode)
		case *ProtocolProfile:
			codes = add(codes, value.OwnerNode)
		case *Rule:
			codes = add(codes, value.GatewaySN)
		case *GatewayPluginStateModel:
			codes = add(codes, value.GatewaySN)
		}
	}
	if len(codes) > 0 {
		return codes, nil
	}
	// A shared product/profile is distributed to every gateway in its scope,
	// including when it is newly created and no device references it yet.
	// Maintenance blocks that platform edit before it becomes a pending change.
	if kind == "Product" || kind == "ProtocolProfile" {
		var tenantID, projectID uint
		switch value := db.Statement.Dest.(type) {
		case *Product:
			tenantID, projectID = value.TenantID, value.ProjectID
		case *ProtocolProfile:
			tenantID, projectID = value.TenantID, value.ProjectID
		}
		query := db.Session(&gorm.Session{NewDB: true}).Model(&Device{}).Where("product_code = ?", "noyo-gw")
		if tenantID != 0 {
			query = query.Where("tenant_id = ?", tenantID)
		}
		if projectID != 0 {
			query = query.Where("project_id = ?", projectID)
		}
		var gateways []string
		if err := query.Pluck("code", &gateways).Error; err != nil {
			return nil, err
		}
		for _, gateway := range gateways {
			codes = add(codes, gateway)
		}
		return codes, nil
	}
	if kind == "RuleGroup" {
		if group, ok := db.Statement.Dest.(*RuleGroup); ok && group != nil {
			var gateways []string
			if err := db.Session(&gorm.Session{NewDB: true}).Model(&Device{}).
				Where("tenant_id = ? AND project_id = ? AND product_code = ?", group.TenantID, group.ProjectID, "noyo-gw").
				Pluck("code", &gateways).Error; err != nil {
				return nil, err
			}
			for _, gateway := range gateways {
				codes = add(codes, gateway)
			}
		}
	}
	return codes, nil
}

func gatewayAncestorCode(db *gorm.DB, parentCode string) (string, error) {
	seen := make(map[string]bool)
	for parentCode != "" {
		if seen[parentCode] {
			return "", fmt.Errorf("device parent cycle at %s", parentCode)
		}
		seen[parentCode] = true
		var parent Device
		err := db.Session(&gorm.Session{NewDB: true}).Select("code", "parent_code", "product_code", "owner_node").
			Where("code = ?", parentCode).First(&parent).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		if err != nil {
			return "", err
		}
		if parent.OwnerNode != "" {
			return parent.OwnerNode, nil
		}
		if parent.ProductCode == "noyo-gw" {
			return parent.Code, nil
		}
		parentCode = parent.ParentCode
	}
	return "", nil
}
