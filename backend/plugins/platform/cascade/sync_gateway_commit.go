package cascade

import (
	"encoding/json"
	"gorm.io/gorm"
	"noyo/core/store"
	"time"
)

// Every accepted platform request produces an authoritative gateway event.
// A transport ACK alone must never update the platform's confirmed projection.
func gatewayCommittedChangeTx(tx *gorm.DB, ch store.SyncChange) (store.SyncChange, error) {
	actual := ch
	if ch.Op != store.SyncOpDelete {
		if ch.EntityType == store.SyncEntityPluginConfig || ch.EntityType == store.SyncEntityNodeConfig {
			cfg, err := store.GetProtectedConfigTx(tx, ch.EntityCode)
			if err != nil {
				return ch, err
			}
			payload, err := json.Marshal(cfg)
			if err != nil {
				return ch, err
			}
			actual.Payload = string(payload)
			actual.NewRev = cfg.Revision
		} else {
			payload, rev, err := resourceSnapshotTx(tx, ch.EntityType, ch.EntityCode)
			if err != nil {
				return ch, err
			}
			actual.Payload = payload
			actual.NewRev = rev
		}
	}
	node, _ := tx.Get(syncLocalNodeKey)
	instance, _ := tx.Get(syncLocalInstanceKey)
	commit := actual
	commit.Seq = 0
	commit.OriginNode, _ = node.(string)
	commit.OriginInstance, _ = instance.(string)
	commit.TargetNode = cloudSyncNodeID
	commit.CreatedAt = time.Now().UnixMilli()
	commit.BasePayload = ""
	_, err := store.AppendSyncChangeTx(tx, &commit)
	return actual, err
}
