package store

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Sync change operations.
const (
	SyncOpUpsert = "upsert"
	SyncOpDelete = "delete"
)

// Entity types that participate in cloud-edge change propagation.
const (
	SyncEntityDevice  = "device"
	SyncEntityProduct = "product"
)

// Sync states exposed on propagated entities.
const (
	SyncStateSynced   = "synced"
	SyncStatePending  = "pending"
	SyncStateConflict = "conflict"
)

// SyncCursorProtocolVersion is the wire protocol version understood by this build.
const SyncCursorProtocolVersion = 1

// SyncApplyingKey marks a GORM session whose writes are replays of a peer's
// change stream. The change recorder must ignore those writes: recording them
// would emit a change for every applied change, and the two nodes would echo
// the same mutation back and forth forever.
const SyncApplyingKey = "noyo_cascade_sync_applying"

// SyncApplyingSession returns a session whose writes are treated as replays.
func SyncApplyingSession() *gorm.DB {
	return DB.Set(SyncApplyingKey, true)
}

// SyncChange is an append-only record of one local mutation. It is the unit of
// cloud-edge propagation: peers replay changes in Seq order and never infer a
// deletion from the absence of a record.
//
// Seq is local to the producing node. The globally unique identity of a change
// is ChangeID(), i.e. "<origin_node>:<seq>"; two nodes may legitimately hold
// the same Seq value without it denoting the same change.
type SyncChange struct {
	Seq        int64  `gorm:"primaryKey;autoIncrement" json:"seq"`
	EntityType string `gorm:"size:16;index;not null" json:"entity_type"`
	EntityCode string `gorm:"size:128;index;not null" json:"entity_code"`
	Op         string `gorm:"size:16;not null" json:"op"`
	BaseRev    int64  `gorm:"not null;default:0" json:"base_rev"`
	NewRev     int64  `gorm:"not null;default:0" json:"new_rev"`
	Payload    string `json:"payload,omitempty"`
	OriginNode string `gorm:"size:128;index;not null" json:"origin_node"`
	// OriginInstance identifies the producer's change log. It is what makes a
	// change identity survive a rebuild of that log: sequence numbers restart at
	// 1 after a database loss, but the instance identifier does not repeat.
	OriginInstance string `gorm:"size:64;index" json:"origin_instance,omitempty"`
	// TargetNode routes the change on the platform side. It holds a
	// comma-separated list of gateway codes, or "cloud" for edge-originated
	// changes. An empty value means "scope-wide" and is fanned out to every
	// gateway inside the change's tenant/project.
	TargetNode string `gorm:"size:512" json:"target_node,omitempty"`
	TenantID   uint   `gorm:"index;not null;default:0" json:"tenant_id"`
	ProjectID  uint   `gorm:"index;not null;default:0" json:"project_id"`
	CreatedAt  int64  `gorm:"index;not null" json:"created_at"`
}

// ChangeID returns the globally unique identity of the change.
func (c *SyncChange) ChangeID() string {
	return FormatChangeID(c.IdentityNode(), c.Seq)
}

// IdentityNode is the node portion of the change identity: the producing log's
// instance when known, otherwise its routing name.
func (c *SyncChange) IdentityNode() string {
	if c.OriginInstance != "" {
		return c.OriginInstance
	}
	return c.OriginNode
}

// TargetNodes splits TargetNode into routing codes.
func (c *SyncChange) TargetNodes() []string {
	if c.TargetNode == "" {
		return nil
	}
	parts := strings.Split(c.TargetNode, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// FormatChangeID builds the globally unique change identity.
func FormatChangeID(originNode string, seq int64) string {
	return fmt.Sprintf("%s:%d", originNode, seq)
}

// AppliedChange records that a change received from a peer has already been
// processed. It is what makes repeated delivery (network retries, overlapping
// catch-up ranges) side effect free.
type AppliedChange struct {
	ChangeID  string `gorm:"primaryKey;size:160"`
	AppliedAt int64  `gorm:"not null"`
}

// SyncCursor tracks, for a single peer, how far our own change stream has been
// sent and acknowledged and how far the peer's stream has been applied here.
//
// Cursors are positions, not timestamps: gateway clocks are not assumed to be
// accurate, so ordering is always expressed as change sequence numbers.
type SyncCursor struct {
	PeerID   string `gorm:"primaryKey;size:128"`
	PeerKind string `gorm:"size:16;not null"`
	// PeerInstance is the identifier of the peer's change log. When it changes,
	// the peer rebuilt its database and its sequence numbers restarted, so
	// LastAppliedSeq no longer describes anything and must be reset.
	PeerInstance string `gorm:"size:64"`
	// LastSentSeq is the highest local Seq handed to the peer.
	LastSentSeq int64 `gorm:"not null;default:0"`
	// LastAckedSeq is the highest local Seq the peer confirmed applying. Catch-up
	// resumes at LastAckedSeq+1.
	LastAckedSeq int64 `gorm:"not null;default:0"`
	// LastAppliedSeq is the highest peer Seq applied locally.
	LastAppliedSeq  int64 `gorm:"not null;default:0"`
	LastOnlineAt    int64 `gorm:"not null;default:0"`
	ProtocolVersion int   `gorm:"not null;default:1"`
	UpdatedAt       int64 `gorm:"not null;default:0"`
}

// SyncNodeIdentity records the instance identifier of this node's change log.
//
// It exists because sequence numbers alone are not a safe identity: a node whose
// database was rebuilt restarts at 1, and without an instance marker the peer
// would match those numbers against changes it already applied and skip them as
// duplicates, losing the data silently.
type SyncNodeIdentity struct {
	PeerID     string `gorm:"primaryKey;size:128"`
	InstanceID string `gorm:"size:64;not null"`
	CreatedAt  int64  `gorm:"not null"`
}

// NodeInstanceID returns the identifier of this node's current change log,
// creating it on first use. Losing the database produces a new identifier, which
// is exactly what makes the restart safe for the peer to observe.
func NodeInstanceID(nodeID string) (string, error) {
	if nodeID == "" {
		return "", fmt.Errorf("node id is required")
	}
	var row SyncNodeIdentity
	err := DB.Where("peer_id = ?", nodeID).First(&row).Error
	if err == nil && row.InstanceID != "" {
		return row.InstanceID, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}

	candidate := SyncNodeIdentity{
		PeerID:     nodeID,
		InstanceID: uuid.NewString(),
		CreatedAt:  time.Now().UnixMilli(),
	}
	if err := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&candidate).Error; err != nil {
		return "", err
	}
	// Re-read rather than trusting the candidate: a concurrent caller may have
	// won the insert.
	if err := DB.Where("peer_id = ?", nodeID).First(&row).Error; err != nil {
		return "", err
	}
	if row.InstanceID == "" {
		return "", fmt.Errorf("failed to establish a node instance identifier for %q", nodeID)
	}
	return row.InstanceID, nil
}

// --- Change log ---

// AppendSyncChange records a local mutation and returns the assigned sequence.
func AppendSyncChange(c *SyncChange) (int64, error) {
	if err := prepareSyncChange(c); err != nil {
		return 0, err
	}
	if err := DB.Create(c).Error; err != nil {
		return 0, err
	}
	return c.Seq, nil
}

// AppendSyncChangeTx records a local mutation inside an existing transaction.
// Callers use it to make the change log and the revision bump that produced it
// atomic: a crash between the two would otherwise leave the change log claiming
// a revision the entity never reached.
func AppendSyncChangeTx(tx *gorm.DB, c *SyncChange) (int64, error) {
	if err := prepareSyncChange(c); err != nil {
		return 0, err
	}
	if err := tx.Create(c).Error; err != nil {
		return 0, err
	}
	return c.Seq, nil
}

func prepareSyncChange(c *SyncChange) error {
	if c == nil {
		return fmt.Errorf("sync change is required")
	}
	if c.EntityType == "" || c.EntityCode == "" || c.Op == "" {
		return fmt.Errorf("sync change requires entity type, code and op")
	}
	if c.CreatedAt == 0 {
		c.CreatedAt = time.Now().UnixMilli()
	}
	return nil
}

// ListSyncChangesAfter returns at most limit changes with Seq greater than afterSeq.
func ListSyncChangesAfter(afterSeq int64, limit int) ([]SyncChange, error) {
	if limit <= 0 {
		limit = 200
	}
	var changes []SyncChange
	err := DB.Where("seq > ?", afterSeq).Order("seq asc").Limit(limit).Find(&changes).Error
	return changes, err
}

// MaxSyncChangeSeq returns the highest recorded sequence, or 0 when empty.
func MaxSyncChangeSeq() (int64, error) {
	var seq int64
	row := DB.Model(&SyncChange{}).Select("COALESCE(MAX(seq), 0)").Row()
	if err := row.Scan(&seq); err != nil {
		return 0, err
	}
	return seq, nil
}

// ListSyncChangesRange returns the changes with afterSeq < Seq <= throughSeq.
func ListSyncChangesRange(afterSeq, throughSeq int64) ([]SyncChange, error) {
	if throughSeq <= afterSeq {
		return nil, nil
	}
	var changes []SyncChange
	err := DB.Where("seq > ? AND seq <= ?", afterSeq, throughSeq).
		Order("seq asc").Find(&changes).Error
	return changes, err
}

// MarkChangesSettled clears the pending flag on entities whose change a peer has
// acknowledged. An entity is only settled when its stored revision still equals
// the acknowledged one, so a newer local write correctly stays pending.
func MarkChangesSettled(afterSeq, throughSeq int64) error {
	changes, err := ListSyncChangesRange(afterSeq, throughSeq)
	if err != nil {
		return err
	}
	for i := range changes {
		ch := changes[i]
		if ch.Op != SyncOpUpsert {
			continue
		}
		var model interface{}
		switch ch.EntityType {
		case SyncEntityDevice:
			model = &Device{}
		case SyncEntityProduct:
			model = &Product{}
		default:
			continue
		}
		if err := DB.Model(model).
			Where("code = ? AND rev = ? AND sync_state = ?", ch.EntityCode, ch.NewRev, SyncStatePending).
			Update("sync_state", SyncStateSynced).Error; err != nil {
			return err
		}
	}
	return nil
}

// MinSyncChangeSeq returns the lowest recorded sequence, or 0 when empty.
func MinSyncChangeSeq() (int64, error) {
	var seq int64
	row := DB.Model(&SyncChange{}).Select("COALESCE(MIN(seq), 0)").Row()
	if err := row.Scan(&seq); err != nil {
		return 0, err
	}
	return seq, nil
}

// PruneSyncChangesThrough removes tombstoned change records that every peer has
// already acknowledged. Callers must verify acknowledgement before pruning;
// removing a record a peer still needs forces a full bootstrap on that peer.
func PruneSyncChangesThrough(seq int64) (int64, error) {
	if seq <= 0 {
		return 0, nil
	}
	res := DB.Where("seq <= ?", seq).Delete(&SyncChange{})
	return res.RowsAffected, res.Error
}

// --- Idempotency ---

// IsChangeApplied reports whether the change has already been processed.
func IsChangeApplied(changeID string) (bool, error) {
	var count int64
	if err := DB.Model(&AppliedChange{}).Where("change_id = ?", changeID).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// MarkChangeAppliedTx records the change inside an existing transaction. It
// reports false when the change had already been applied, which is the normal
// outcome for a redundant delivery and must not be treated as an error.
func MarkChangeAppliedTx(tx *gorm.DB, changeID string) (bool, error) {
	res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&AppliedChange{
		ChangeID:  changeID,
		AppliedAt: time.Now().UnixMilli(),
	})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// PruneAppliedChanges keeps the idempotency table bounded by dropping entries
// older than the retention window.
func PruneAppliedChanges(retention time.Duration) (int64, error) {
	cutoff := time.Now().Add(-retention).UnixMilli()
	res := DB.Where("applied_at < ?", cutoff).Delete(&AppliedChange{})
	return res.RowsAffected, res.Error
}

// --- Cursors ---

// GetSyncCursor returns the cursor for a peer, or a zero cursor when absent.
func GetSyncCursor(peerID string) (*SyncCursor, error) {
	if peerID == "" {
		return nil, fmt.Errorf("sync cursor peer id is required")
	}
	var c SyncCursor
	err := DB.Where("peer_id = ?", peerID).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &SyncCursor{PeerID: peerID, ProtocolVersion: SyncCursorProtocolVersion}, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// SaveSyncCursor upserts a cursor record.
func SaveSyncCursor(c *SyncCursor) error {
	if c == nil || c.PeerID == "" {
		return fmt.Errorf("sync cursor peer id is required")
	}
	if c.ProtocolVersion == 0 {
		c.ProtocolVersion = SyncCursorProtocolVersion
	}
	c.UpdatedAt = time.Now().UnixMilli()
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "peer_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"peer_kind", "peer_instance", "last_sent_seq", "last_acked_seq", "last_applied_seq",
			"last_online_at", "protocol_version", "updated_at",
		}),
	}).Create(c).Error
}

// ListSyncCursors returns every known peer cursor.
func ListSyncCursors() ([]SyncCursor, error) {
	var cursors []SyncCursor
	err := DB.Order("peer_id asc").Find(&cursors).Error
	return cursors, err
}

// ResetSyncProgress clears every cursor position while preserving peer identity.
// It is used when a peer's position can no longer be satisfied incrementally and
// both sides must re-handshake from scratch.
func ResetSyncProgress() error {
	return DB.Model(&SyncCursor{}).Where("1 = 1").Updates(map[string]interface{}{
		"last_sent_seq":    0,
		"last_acked_seq":   0,
		"last_applied_seq": 0,
		"updated_at":       time.Now().UnixMilli(),
	}).Error
}

// --- Mutation sink ---

// SyncChangeSink receives mutations that cannot be reconstructed from GORM
// callbacks. Device deletion is the motivating case: DeleteDevice renames the
// code before soft-deleting the row, so by the time any callback observes the
// statement the original code is already gone.
//
// The cascade plugin installs a sink during engine startup. When no sink is
// installed these notifications are dropped, which keeps the store usable in
// tests and in deployments that do not run the cascade plugin.
type SyncChangeSink interface {
	DeviceDeleted(d *Device)
}

var syncChangeSink SyncChangeSink

// SetSyncChangeSink installs the mutation sink. Passing nil clears it.
func SetSyncChangeSink(s SyncChangeSink) {
	syncChangeSink = s
}

func notifyDeviceDeleted(d *Device) {
	if sink := syncChangeSink; sink != nil {
		sink.DeviceDeleted(d)
	}
}
