package cascade

import (
	"encoding/json"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"noyo/core/store"
)

// These errors describe an invalid business request, unlike a database error
// or a maintenance barrier, which must remain retryable.
type syncValidationError struct{ code, message string }

func (e *syncValidationError) Error() string { return e.message }
func invalidSync(code, format string, args ...interface{}) error {
	return &syncValidationError{code: code, message: fmt.Sprintf(format, args...)}
}

type dependencyResource struct {
	Code                string `json:"code"`
	Rev                 int64  `json:"rev"`
	TenantID            uint   `json:"tenant_id"`
	ProjectID           uint   `json:"project_id"`
	OwnerNode           string `json:"owner_node"`
	ProductCode         string `json:"product_code"`
	ProtocolProfileCode string `json:"protocol_profile_code"`
	ProtocolName        string `json:"protocol_name"`
}

func validateSyncRequest(ch store.SyncChange) error {
	switch ch.EntityType {
	case store.SyncEntityDevice, store.SyncEntityProduct, store.SyncEntityDriver, store.SyncEntityRule, store.SyncEntityRuleGroup, store.SyncEntityPluginConfig, store.SyncEntityNodeConfig:
	default:
		return invalidSync("INVALID_CONFIGURATION", "unsupported configuration type %s", ch.EntityType)
	}
	if ch.Op != store.SyncOpUpsert && ch.Op != store.SyncOpDelete {
		return invalidSync("INVALID_CONFIGURATION", "unsupported configuration operation %s", ch.Op)
	}
	if ch.Op == store.SyncOpDelete {
		return nil
	}
	var value dependencyResource
	if ch.Payload == "" || !validSyncDocument(ch.EntityType, ch.Payload) || json.Unmarshal([]byte(ch.Payload), &value) != nil {
		return invalidSync("INVALID_CONFIGURATION", "invalid configuration payload for %s %s", ch.EntityType, ch.EntityCode)
	}
	if value.Code != ch.EntityCode {
		return invalidSync("INVALID_CONFIGURATION", "configuration code mismatch for %s %s", ch.EntityType, ch.EntityCode)
	}
	if ch.BasePayload != "" && !validSyncDocument(ch.EntityType, ch.BasePayload) {
		return invalidSync("INVALID_CONFIGURATION", "invalid configuration payload for %s %s", ch.EntityType, ch.EntityCode)
	}
	if ch.EntityType == store.SyncEntityPluginConfig || ch.EntityType == store.SyncEntityNodeConfig {
		var config store.ProtectedConfig
		if json.Unmarshal([]byte(ch.Payload), &config) != nil || config.Validate() != nil {
			return invalidSync("INVALID_CONFIGURATION", "invalid configuration payload for %s %s", ch.EntityType, ch.EntityCode)
		}
	}
	return nil
}

func validSyncDocument(kind, payload string) bool {
	var object map[string]json.RawMessage
	if json.Unmarshal([]byte(payload), &object) != nil || object == nil {
		return false
	}
	var target interface{}
	switch kind {
	case store.SyncEntityDevice:
		target = &store.Device{}
	case store.SyncEntityProduct:
		target = &store.Product{}
	case store.SyncEntityDriver:
		target = &store.ProtocolProfile{}
	case store.SyncEntityRule:
		target = &store.Rule{}
	case store.SyncEntityRuleGroup:
		target = &store.RuleGroup{}
	case store.SyncEntityPluginConfig, store.SyncEntityNodeConfig:
		target = &store.ProtectedConfig{}
	default:
		return false
	}
	return json.Unmarshal([]byte(payload), target) == nil
}

// Capture immutable dependency baselines while saving the platform intent.
// This also prevents a platform write from producing a dangling binding.
func collectConfigDependenciesTx(tx *gorm.DB, ch *store.SyncChange) error {
	if ch.OriginNode != cloudSyncNodeID || ch.Op != store.SyncOpUpsert || (ch.EntityType != store.SyncEntityDevice && ch.EntityType != store.SyncEntityDriver) {
		return nil
	}
	var value dependencyResource
	if err := json.Unmarshal([]byte(ch.Payload), &value); err != nil {
		return err
	}
	seen := map[string]bool{}
	var collect func(string, string) error
	collect = func(kind, code string) error {
		if code == "" || seen[kind+":"+code] {
			return nil
		}
		seen[kind+":"+code] = true
		payload, revision, err := resourceSnapshotTx(tx, kind, code)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return invalidSync("DEPENDENCY_MISSING", "configuration dependency %s %s was not found", kind, code)
		}
		if err != nil {
			return err
		}
		var dependency dependencyResource
		if err := json.Unmarshal([]byte(payload), &dependency); err != nil {
			return err
		}
		if kind == store.SyncEntityDriver {
			if err := collect(store.SyncEntityProduct, dependency.ProductCode); err != nil {
				return err
			}
		}
		version := int64(1)
		if tx.Migrator().HasTable(&store.ResourceLifecycle{}) {
			var lifecycle store.ResourceLifecycle
			err := tx.Session(&gorm.Session{NewDB: true}).Where("entity_type = ? AND code = ?", kind, code).Take(&lifecycle).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err == nil {
				version = lifecycle.Version
			}
		}
		ch.Dependencies = append(ch.Dependencies, store.SyncDependency{EntityType: kind, EntityCode: code, Revision: revision, LifecycleVersion: version, Payload: payload})
		return nil
	}
	if err := collect(store.SyncEntityProduct, value.ProductCode); err != nil {
		return err
	}
	return collect(store.SyncEntityDriver, value.ProtocolProfileCode)
}

// Validate the entire binding group before writing anything. Returned plans
// seed missing definitions in the root request's transaction. Existing site
// definitions are checked but never overwritten by an attached baseline.
func validateConfigDependenciesTx(tx *gorm.DB, ch store.SyncChange) ([]store.SyncChange, error) {
	if ch.OriginNode != cloudSyncNodeID || preferRemoteChange(tx) || ch.Op != store.SyncOpUpsert || (ch.EntityType != store.SyncEntityDevice && ch.EntityType != store.SyncEntityDriver) {
		return nil, nil
	}
	var value dependencyResource
	if err := json.Unmarshal([]byte(ch.Payload), &value); err != nil {
		return nil, err
	}
	attached := map[string]store.SyncDependency{}
	for _, dependency := range ch.Dependencies {
		key := dependency.EntityType + ":" + dependency.EntityCode
		if dependency.EntityType != store.SyncEntityProduct && dependency.EntityType != store.SyncEntityDriver {
			return nil, invalidSync("INVALID_CONFIGURATION", "unsupported configuration dependency %s", key)
		}
		if _, duplicate := attached[key]; duplicate {
			return nil, invalidSync("INVALID_CONFIGURATION", "duplicate configuration dependency %s", key)
		}
		attached[key] = dependency
	}
	used := map[string]bool{}
	plans := []store.SyncChange{}
	localNode, _ := tx.Get(syncLocalNodeKey)
	gateway, _ := localNode.(string)
	var require func(string, string) (*dependencyResource, error)
	require = func(kind, code string) (*dependencyResource, error) {
		if code == "" {
			return nil, nil
		}
		key := kind + ":" + code
		baseline, provided := attached[key]
		var resource dependencyResource
		payload, revision, err := resourceSnapshotTx(tx, kind, code)
		missing := errors.Is(err, gorm.ErrRecordNotFound)
		if err != nil && !missing {
			return nil, err
		}
		if missing {
			if !provided {
				return nil, invalidSync("DEPENDENCY_MISSING", "configuration dependency %s %s was not found", kind, code)
			}
			payload, revision = baseline.Payload, baseline.Revision
		}
		if json.Unmarshal([]byte(payload), &resource) != nil || resource.Code != code {
			return nil, invalidSync("INVALID_CONFIGURATION", "invalid configuration dependency %s", key)
		}
		if !dependencyScopeMatches(resource, ch, gateway) {
			return nil, invalidSync("DEPENDENCY_SCOPE", "configuration dependency %s %s belongs to another scope or gateway", kind, code)
		}
		if provided {
			var proposed dependencyResource
			if !validSyncDocument(kind, baseline.Payload) || json.Unmarshal([]byte(baseline.Payload), &proposed) != nil || proposed.Code != code || baseline.LifecycleVersion < 1 || baseline.Revision < 1 || proposed.Rev != baseline.Revision {
				return nil, invalidSync("INVALID_CONFIGURATION", "invalid configuration dependency %s", key)
			}
			if !dependencyScopeMatches(proposed, ch, gateway) {
				return nil, invalidSync("DEPENDENCY_SCOPE", "configuration dependency %s %s belongs to another scope or gateway", kind, code)
			}
		}
		var lifecycle store.ResourceLifecycle
		lookup := tx.Session(&gorm.Session{NewDB: true}).Where("entity_type = ? AND code = ?", kind, code).Take(&lifecycle).Error
		if lookup != nil && !errors.Is(lookup, gorm.ErrRecordNotFound) {
			return nil, lookup
		}
		if lookup == nil && (lifecycle.Deleted || (provided && lifecycle.Version != baseline.LifecycleVersion)) {
			return nil, invalidSync("DEPENDENCY_CHANGED", "configuration dependency %s %s was deleted or recreated", kind, code)
		}
		if provided && !missing && revision < baseline.Revision {
			return nil, invalidSync("DEPENDENCY_CHANGED", "configuration dependency %s %s has not reached the required revision", kind, code)
		}
		if used[key] {
			return &resource, nil
		}
		used[key] = true
		if kind == store.SyncEntityDriver {
			if _, err := require(store.SyncEntityProduct, resource.ProductCode); err != nil {
				return nil, err
			}
		}
		if missing {
			plans = append(plans, store.SyncChange{EntityType: kind, EntityCode: code, Op: store.SyncOpUpsert, NewRev: revision, LifecycleVersion: baseline.LifecycleVersion, Payload: payload, OriginNode: ch.OriginNode, OriginInstance: ch.OriginInstance, TenantID: ch.TenantID, ProjectID: ch.ProjectID})
		}
		return &resource, nil
	}
	if _, err := require(store.SyncEntityProduct, value.ProductCode); err != nil {
		return nil, err
	}
	if value.ProtocolProfileCode != "" {
		driver, err := require(store.SyncEntityDriver, value.ProtocolProfileCode)
		if err != nil {
			return nil, err
		}
		if driver.ProductCode != "" && driver.ProductCode != value.ProductCode {
			return nil, invalidSync("DEPENDENCY_CHANGED", "configuration driver %s is bound to another product", value.ProtocolProfileCode)
		}
	}
	for key := range attached {
		if !used[key] {
			return nil, invalidSync("INVALID_CONFIGURATION", "unreferenced configuration dependency %s", key)
		}
	}
	return plans, nil
}

func dependencyScopeMatches(resource dependencyResource, ch store.SyncChange, gateway string) bool {
	// Zero scopes are existing system/tenant templates supported by the
	// device API. A concrete foreign scope or owner never becomes shareable.
	return (resource.TenantID == 0 || resource.TenantID == ch.TenantID) &&
		(resource.ProjectID == 0 || resource.ProjectID == ch.ProjectID) &&
		(resource.OwnerNode == "" || resource.OwnerNode == gateway)
}

func recordSyncBusinessFailure(ch store.SyncChange, failure *syncValidationError) (string, error) {
	status := syncStatusFailed
	err := store.SyncApplyingSession().Transaction(func(tx *gorm.DB) error {
		fresh, err := store.MarkChangeAppliedTx(tx, ch.ChangeID())
		if err != nil {
			return err
		}
		if !fresh {
			var prior store.AppliedChange
			if err := tx.Where("change_id = ?", ch.ChangeID()).Take(&prior).Error; err != nil {
				return err
			}
			status = prior.Status
			return nil
		}
		return tx.Model(&store.AppliedChange{}).Where("change_id = ?", ch.ChangeID()).Updates(map[string]interface{}{"status": syncStatusFailed, "error_code": failure.code, "last_error": failure.message, "entity_type": ch.EntityType, "entity_code": ch.EntityCode, "origin_instance": ch.OriginInstance, "tenant_id": ch.TenantID, "project_id": ch.ProjectID, "lifecycle_version": ch.LifecycleVersion}).Error
	})
	return status, err
}

func commitDependencyTx(tx *gorm.DB, dependency store.SyncChange, hooks syncApplyHooks) (store.SyncChange, error) {
	payload, revision, err := resourceSnapshotTx(tx, dependency.EntityType, dependency.EntityCode)
	if err != nil {
		return dependency, err
	}
	dependency.Payload, dependency.NewRev = payload, revision
	dependency.OriginNode, dependency.OriginInstance = hooks.localNode, hooks.localInstance
	dependency.TargetNode = cloudSyncNodeID
	if _, err := store.AppendSyncChangeTx(tx, &dependency); err != nil {
		return dependency, err
	}
	if hooks.afterDependencies != nil {
		if err := store.QueueRuntimeApplyTx(tx, dependency); err != nil {
			return dependency, err
		}
	}
	return dependency, nil
}
