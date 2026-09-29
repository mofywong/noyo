package cascade

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"noyo/core/store"
)

type intentLineage struct{ base, payload string }

// Resolve only the fields authored by an applied predecessor. Unrelated site
// changes in its canonical result must not become a new editing baseline.
func resolveIntentPredecessorTx(tx *gorm.DB, ch *store.SyncChange) (intentLineage, error) {
	lineage := intentLineage{ch.BasePayload, ch.Payload}
	if ch.OriginNode != cloudSyncNodeID || ch.PredecessorCode == "" {
		return lineage, nil
	}
	var prior store.AppliedChange
	if err := tx.Session(&gorm.Session{NewDB: true}).Where("change_id = ?", ch.PredecessorCode).Take(&prior).Error; err != nil {
		// A restored gateway may no longer have the predecessor's result. It
		// can only perform the original conservative three-way comparison.
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return lineage, nil
		}
		return lineage, err
	}
	if prior.Status != syncStatusApplied {
		return lineage, nil
	}
	if prior.EntityType != ch.EntityType || prior.EntityCode != ch.EntityCode || prior.OriginInstance != ch.OriginInstance || prior.TenantID != ch.TenantID || prior.ProjectID != ch.ProjectID || prior.LifecycleVersion > ch.LifecycleVersion {
		return lineage, fmt.Errorf("configuration predecessor does not match resource, scope or generation")
	}
	if prior.LifecycleVersion < ch.LifecycleVersion {
		// Older senders may have persisted a link across code recreation. Ignore
		// that old baseline and compare the original proposal with the current
		// lifecycle, so pending delivery can recover without inheriting authority
		// or revisions from the previous incarnation.
		return lineage, nil
	}
	// A failed/conflicting predecessor grants no authority to rebase. The
	// ordinary three-way comparison still evaluates this original proposal.
	if prior.Status != syncStatusApplied || prior.ResultPayload == "" {
		return lineage, nil
	}
	if ch.Op == store.SyncOpDelete {
		ch.BasePayload, ch.BaseRev = prior.ResultPayload, prior.ResultRevision
		ch.NewRev = max(ch.NewRev, prior.ResultRevision+1)
		return lineage, nil
	}
	sourceBase, err := intentDocument(ch.BasePayload, ch.EntityType)
	if err != nil {
		return lineage, err
	}
	source, err := intentDocument(ch.Payload, ch.EntityType)
	if err != nil {
		return lineage, err
	}
	oldBase, err := intentDocument(prior.IntentBase, ch.EntityType)
	if err != nil {
		return lineage, err
	}
	oldIntent, err := intentDocument(prior.IntentPayload, ch.EntityType)
	if err != nil {
		return lineage, err
	}
	result, err := intentDocument(prior.ResultPayload, ch.EntityType)
	if err != nil {
		return lineage, err
	}
	base, desired := cloneIntentDocument(sourceBase), cloneIntentDocument(source)
	chainBase, chainDesired := cloneIntentDocument(oldBase), cloneIntentDocument(oldIntent)
	groups := syncMergeGroups[ch.EntityType]
	if ch.EntityType == store.SyncEntityPluginConfig || ch.EntityType == store.SyncEntityNodeConfig {
		keys := map[string]bool{}
		for _, doc := range []map[string]json.RawMessage{sourceBase, source, oldBase, oldIntent} {
			for key := range doc {
				keys[key] = true
			}
		}
		for key := range keys {
			groups = append(groups, []string{key})
		}
	}
	if len(groups) == 0 {
		return lineage, nil
	}
	for _, group := range groups {
		ownChange := !intentGroupEqual(oldBase, oldIntent, group)
		compatibleBase := intentGroupEqual(sourceBase, oldBase, group) || intentGroupEqual(sourceBase, oldIntent, group)
		if ownChange && compatibleBase {
			copyIntentGroup(base, result, group)
			copyIntentGroup(desired, result, group)
			// Apply the new editor's actual delta, preserving fields changed by
			// earlier operations that this editor has not explicitly changed.
			for _, field := range group {
				if !bytes.Equal(source[field], sourceBase[field]) {
					copyIntentField(desired, source, field)
				}
			}
		} else if !compatibleBase {
			// A newly confirmed editing baseline starts this group's lineage.
			copyIntentGroup(chainBase, sourceBase, group)
			copyIntentGroup(chainDesired, sourceBase, group)
		}
		for _, field := range group {
			if !bytes.Equal(source[field], sourceBase[field]) {
				copyIntentField(chainDesired, source, field)
			}
		}
	}
	ch.NewRev = max(ch.NewRev, prior.ResultRevision+1)
	ch.BasePayload, err = encodeIntentDocument(ch.BasePayload, ch.EntityType, base, 0)
	if err != nil {
		return lineage, err
	}
	ch.Payload, err = encodeIntentDocument(ch.Payload, ch.EntityType, desired, ch.NewRev)
	if err != nil {
		return lineage, err
	}
	// Force group comparison even when the immediately preceding revision is
	// current: fields outside the predecessor's patch keep their old baseline.
	ch.BaseRev = 0
	lineage.base, err = encodeIntentDocument(prior.IntentBase, ch.EntityType, chainBase, 0)
	if err != nil {
		return lineage, err
	}
	lineage.payload, err = encodeIntentDocument(ch.Payload, ch.EntityType, chainDesired, ch.NewRev)
	return lineage, err
}

func recordIntentResultTx(tx *gorm.DB, ch store.SyncChange, lineage intentLineage, status string) error {
	if ch.OriginNode != cloudSyncNodeID {
		return nil
	}
	payload, revision := "", ch.NewRev
	if ch.Op != store.SyncOpDelete {
		var err error
		if ch.EntityType == store.SyncEntityPluginConfig || ch.EntityType == store.SyncEntityNodeConfig {
			value, loadErr := store.GetProtectedConfigTx(tx, ch.EntityCode)
			err = loadErr
			if err == nil {
				data, encodeErr := json.Marshal(value)
				payload, revision, err = string(data), value.Revision, encodeErr
			}
		} else {
			payload, revision, err = resourceSnapshotTx(tx, ch.EntityType, ch.EntityCode)
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}
	return tx.Session(&gorm.Session{NewDB: true}).Model(&store.AppliedChange{}).Where("change_id = ?", ch.ChangeID()).Updates(map[string]interface{}{
		"entity_type": ch.EntityType, "entity_code": ch.EntityCode, "origin_instance": ch.OriginInstance, "tenant_id": ch.TenantID, "project_id": ch.ProjectID, "lifecycle_version": ch.LifecycleVersion,
		"result_revision": revision, "result_payload": payload, "intent_base": lineage.base, "intent_payload": lineage.payload,
	}).Error
}

func intentDocument(payload, kind string) (map[string]json.RawMessage, error) {
	doc := map[string]json.RawMessage{}
	if payload != "" {
		if err := json.Unmarshal([]byte(payload), &doc); err != nil {
			return nil, err
		}
	}
	if kind == store.SyncEntityPluginConfig || kind == store.SyncEntityNodeConfig {
		config := map[string]json.RawMessage{}
		if raw := doc["config"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &config); err != nil {
				return nil, err
			}
		}
		return config, nil
	}
	return doc, nil
}

func encodeIntentDocument(original, kind string, doc map[string]json.RawMessage, revision int64) (string, error) {
	if kind == store.SyncEntityPluginConfig || kind == store.SyncEntityNodeConfig {
		root := map[string]json.RawMessage{}
		if original != "" {
			if err := json.Unmarshal([]byte(original), &root); err != nil {
				return "", err
			}
		}
		data, err := json.Marshal(doc)
		if err != nil {
			return "", err
		}
		root["config"] = data
		if revision > 0 {
			root["revision"], _ = json.Marshal(revision)
		}
		doc = root
	}
	data, err := json.Marshal(doc)
	return string(data), err
}

func cloneIntentDocument(doc map[string]json.RawMessage) map[string]json.RawMessage {
	result := map[string]json.RawMessage{}
	for key, value := range doc {
		result[key] = value
	}
	return result
}
func intentGroupEqual(a, b map[string]json.RawMessage, group []string) bool {
	for _, field := range group {
		if !bytes.Equal(a[field], b[field]) {
			return false
		}
	}
	return true
}
func copyIntentField(to, from map[string]json.RawMessage, key string) {
	if value, ok := from[key]; ok {
		to[key] = value
	} else {
		delete(to, key)
	}
}
func copyIntentGroup(to, from map[string]json.RawMessage, group []string) {
	for _, key := range group {
		copyIntentField(to, from, key)
	}
}
