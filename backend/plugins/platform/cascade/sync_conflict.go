package cascade

import (
	"encoding/json"
	"fmt"
	"time"

	"noyo/core/store"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Conflict resolutions an operator can choose.
const (
	syncResolveKeepLocal = "local"
	syncResolveTakePeer  = "peer"
)

// syncResolveRequestItem is one arbitration decision.
type syncResolveRequestItem struct {
	EntityType string `json:"entity_type"`
	EntityCode string `json:"entity_code"`
	// Resolution is "local" to keep this node's version or "peer" to accept the
	// peer's.
	Resolution string `json:"resolution"`
}

// syncConflictView is the arbitration payload handed to the UI. Both versions
// are presented side by side so an operator can decide without leaving the page.
type syncConflictView struct {
	EntityType string         `json:"entity_type"`
	EntityCode string         `json:"entity_code"`
	Name       string         `json:"name"`
	PeerNode   string         `json:"peer_node"`
	PeerRev    int64          `json:"peer_rev"`
	PeerOp     string         `json:"peer_op"`
	DetectedAt int64          `json:"detected_at"`
	Local      *store.Device  `json:"local,omitempty"`
	LocalProd  *store.Product `json:"local_product,omitempty"`
	Peer       *store.Device  `json:"peer,omitempty"`
	PeerProd   *store.Product `json:"peer_product,omitempty"`
	PeerDelete bool           `json:"peer_delete"`
}

// collectSyncConflicts lists every entity awaiting arbitration inside a scope.
func collectSyncConflicts(tenantID, projectID uint) ([]syncConflictView, error) {
	views := make([]syncConflictView, 0)

	deviceQuery := store.DB.Where("sync_state = ?", store.SyncStateConflict)
	if tenantID > 0 {
		deviceQuery = deviceQuery.Where("tenant_id = ?", tenantID)
	}
	if projectID > 0 {
		deviceQuery = deviceQuery.Where("project_id = ?", projectID)
	}
	var devices []store.Device
	if err := deviceQuery.Order("code asc").Find(&devices).Error; err != nil {
		return nil, err
	}
	for i := range devices {
		record, _ := decodeConflictRecord(devices[i].ConflictPayload)
		view := syncConflictView{
			EntityType: store.SyncEntityDevice,
			EntityCode: devices[i].Code,
			Name:       devices[i].Name,
			PeerNode:   devices[i].ConflictNode,
			PeerRev:    devices[i].ConflictRev,
			Local:      &devices[i],
		}
		if record != nil {
			view.PeerOp = record.Op
			view.DetectedAt = record.At
			view.PeerDelete = len(record.Payload) == 0
			if !view.PeerDelete {
				var peer store.Device
				if err := json.Unmarshal(record.Payload, &peer); err == nil {
					view.Peer = &peer
				}
			}
		}
		views = append(views, view)
	}

	productQuery := store.DB.Where("sync_state = ?", store.SyncStateConflict)
	if tenantID > 0 {
		productQuery = productQuery.Where("tenant_id = ?", tenantID)
	}
	if projectID > 0 {
		productQuery = productQuery.Where("project_id = ?", projectID)
	}
	var products []store.Product
	if err := productQuery.Order("code asc").Find(&products).Error; err != nil {
		return nil, err
	}
	for i := range products {
		record, _ := decodeConflictRecord(products[i].ConflictPayload)
		view := syncConflictView{
			EntityType: store.SyncEntityProduct,
			EntityCode: products[i].Code,
			Name:       products[i].Name,
			PeerNode:   products[i].ConflictNode,
			PeerRev:    products[i].ConflictRev,
			LocalProd:  &products[i],
		}
		if record != nil {
			view.PeerOp = record.Op
			view.DetectedAt = record.At
			view.PeerDelete = len(record.Payload) == 0
			if !view.PeerDelete {
				var peer store.Product
				if err := json.Unmarshal(record.Payload, &peer); err == nil {
					view.PeerProd = &peer
				}
			}
		}
		views = append(views, view)
	}

	return views, nil
}

func decodeConflictRecord(payload string) (*conflictRecord, error) {
	if payload == "" {
		return nil, nil
	}
	var record conflictRecord
	if err := json.Unmarshal([]byte(payload), &record); err != nil {
		return nil, err
	}
	return &record, nil
}

// resolveEntityConflict applies an operator's arbitration decision and returns
// the change that must be pushed to the peer, if any.
//
// Either choice emits a new change, because the peer still holds the losing
// version. Clearing the conflict without emitting one would leave the two sides
// permanently divergent while both reported themselves as synchronized.
//
// The emitted change is based on the peer's revision rather than our own, so the
// peer recognises it as a direct successor of what it sent and applies it
// without raising a second conflict.
func resolveEntityConflict(item syncResolveRequestItem, nodeID, origin string, logger *zap.Logger) (*store.SyncChange, error) {
	switch item.EntityType {
	case store.SyncEntityDevice:
		return resolveDeviceConflict(item, nodeID, origin, logger)
	case store.SyncEntityProduct:
		return resolveProductConflict(item, nodeID, origin, logger)
	default:
		return nil, fmt.Errorf("unsupported entity type %q", item.EntityType)
	}
}

func resolveDeviceConflict(item syncResolveRequestItem, nodeID, origin string, logger *zap.Logger) (*store.SyncChange, error) {
	code := item.EntityCode
	var current store.Device
	if err := store.DB.Where("code = ?", code).First(&current).Error; err != nil {
		return nil, fmt.Errorf("device %q not found: %w", code, err)
	}
	if current.SyncState != store.SyncStateConflict {
		return nil, fmt.Errorf("device %q has no unresolved conflict", code)
	}
	record, err := decodeConflictRecord(current.ConflictPayload)
	if err != nil || record == nil {
		return nil, fmt.Errorf("device %q has a malformed conflict record", code)
	}

	// Accepting the peer's deletion is a decision rather than a content merge.
	// It propagates through the local deletion path, so no change is returned.
	if item.Resolution == syncResolveTakePeer && len(record.Payload) == 0 {
		if err := store.DeleteDevice(code); err != nil {
			return nil, err
		}
		logger.Info("Conflict resolved by accepting the peer deletion",
			zap.String("entity", code), zap.String("peer", record.Node))
		return nil, nil
	}

	var decided store.Device
	switch item.Resolution {
	case syncResolveKeepLocal:
		decided = current
	case syncResolveTakePeer:
		if err := json.Unmarshal(record.Payload, &decided); err != nil {
			return nil, fmt.Errorf("device %q has an unreadable peer version: %w", code, err)
		}
	default:
		return nil, fmt.Errorf("unsupported resolution %q", item.Resolution)
	}

	newRev := maxInt64(current.Rev, record.Rev) + 1
	decided.ID = current.ID
	decided.CreatedAt = current.CreatedAt
	decided.UpdatedAt = time.Now()
	decided.DeletedAt = gorm.DeletedAt{}
	// Ownership is an addressing fact rather than content, so it survives the
	// arbitration whichever side wins. A device with no local owner may still
	// adopt the peer's.
	owner := current.OwnerNode
	if owner == "" {
		owner = decided.OwnerNode
	}
	decided.SyncMeta = store.SyncMeta{
		Rev:        newRev,
		SyncState:  store.SyncStateSynced,
		Origin:     valueOr(decided.Origin, origin),
		OwnerNode:  owner,
		SyncWriter: nodeID,
	}

	payload, err := buildDevicePayload(&decided)
	if err != nil {
		return nil, err
	}
	instanceID, err := store.NodeInstanceID(nodeID)
	if err != nil {
		return nil, err
	}

	// The decision and its propagation record are committed together: a crash
	// between them would leave the conflict cleared but the peer unpersuaded.
	var recorded *store.SyncChange
	err = store.SyncApplyingSession().Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Save(&decided).Error; err != nil {
			return err
		}
		ch := &store.SyncChange{
			EntityType:     store.SyncEntityDevice,
			EntityCode:     code,
			Op:             store.SyncOpUpsert,
			BaseRev:        record.Rev,
			NewRev:         newRev,
			Payload:        payload,
			OriginNode:     nodeID,
			OriginInstance: instanceID,
			TargetNode:     record.Node,
			TenantID:       decided.TenantID,
			ProjectID:      decided.ProjectID,
			CreatedAt:      time.Now().UnixMilli(),
		}
		if _, err := store.AppendSyncChangeTx(tx, ch); err != nil {
			return err
		}
		recorded = ch
		return nil
	})
	if err != nil {
		return nil, err
	}
	return recorded, nil
}

func resolveProductConflict(item syncResolveRequestItem, nodeID, origin string, logger *zap.Logger) (*store.SyncChange, error) {
	code := item.EntityCode
	var current store.Product
	if err := store.DB.Where("code = ?", code).First(&current).Error; err != nil {
		return nil, fmt.Errorf("product %q not found: %w", code, err)
	}
	if current.SyncState != store.SyncStateConflict {
		return nil, fmt.Errorf("product %q has no unresolved conflict", code)
	}
	record, err := decodeConflictRecord(current.ConflictPayload)
	if err != nil || record == nil {
		return nil, fmt.Errorf("product %q has a malformed conflict record", code)
	}

	var decided store.Product
	switch item.Resolution {
	case syncResolveKeepLocal:
		decided = current
	case syncResolveTakePeer:
		if len(record.Payload) == 0 {
			return nil, fmt.Errorf("product %q was not deleted by the peer; there is nothing to accept", code)
		}
		if err := json.Unmarshal(record.Payload, &decided); err != nil {
			return nil, fmt.Errorf("product %q has an unreadable peer version: %w", code, err)
		}
	default:
		return nil, fmt.Errorf("unsupported resolution %q", item.Resolution)
	}

	newRev := maxInt64(current.Rev, record.Rev) + 1
	decided.ID = current.ID
	decided.CreatedAt = current.CreatedAt
	decided.UpdatedAt = time.Now()
	decided.DeletedAt = gorm.DeletedAt{}
	decided.SyncMeta = store.SyncMeta{
		Rev:        newRev,
		SyncState:  store.SyncStateSynced,
		Origin:     valueOr(decided.Origin, origin),
		SyncWriter: nodeID,
	}

	payload, err := buildProductPayload(&decided)
	if err != nil {
		return nil, err
	}
	instanceID, err := store.NodeInstanceID(nodeID)
	if err != nil {
		return nil, err
	}

	var recorded *store.SyncChange
	err = store.SyncApplyingSession().Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Save(&decided).Error; err != nil {
			return err
		}
		ch := &store.SyncChange{
			EntityType:     store.SyncEntityProduct,
			EntityCode:     code,
			Op:             store.SyncOpUpsert,
			BaseRev:        record.Rev,
			NewRev:         newRev,
			Payload:        payload,
			OriginNode:     nodeID,
			OriginInstance: instanceID,
			TargetNode:     record.Node,
			TenantID:       decided.TenantID,
			ProjectID:      decided.ProjectID,
			CreatedAt:      time.Now().UnixMilli(),
		}
		if _, err := store.AppendSyncChangeTx(tx, ch); err != nil {
			return err
		}
		recorded = ch
		return nil
	})
	if err != nil {
		return nil, err
	}
	return recorded, nil
}

func valueOr(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
