package inspection

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrExecutionGrant = errors.New("execution_grant_revoked")

// ExecutionGrant records project service delegation and its bounded run grants.
// A parent has no RunID or expiry and is explicitly revocable. Its children
// expire with the run. AuthorizedBy is audit data, never an impersonated user.
type ExecutionGrant struct {
	ID           string `gorm:"primaryKey;size:36"`
	Scope        `gorm:"embedded"`
	PlanID       string `gorm:"index"`
	RunID        string `gorm:"index"`
	ParentID     string `gorm:"index"`
	Capability   string
	SnapshotHash string
	AuthorizedBy uint
	CreatedAt    time.Time
	ExpiresAt    *time.Time
	RevokedAt    *time.Time
	RevokedBy    uint
}

func (ExecutionGrant) TableName() string { return "inspection_execution_grants" }

func snapshotHash(snapshot string) string {
	h := sha256.Sum256([]byte(snapshot))
	return hex.EncodeToString(h[:])
}

// Callers must verify inspection:execute and camera read/control rights before
// saving or enabling AI plans. These trusted service methods accept that actor.
func (s *Service) issuePlanGrant(tx *gorm.DB, p *Plan, actor Actor) error {
	if p.Mode != "ai" {
		return nil
	}
	g := ExecutionGrant{ID: uuid.NewString(), Scope: p.Scope, PlanID: p.ID,
		Capability: "inspection:analyze", SnapshotHash: snapshotHash(p.SpecJSON),
		AuthorizedBy: actor.UserID, CreatedAt: s.Now()}
	if err := tx.Create(&g).Error; err != nil {
		return err
	}
	p.ExecutionGrantID = g.ID
	return nil
}

func (s *Service) issueRunGrant(tx *gorm.DB, p Plan, r *Run) error {
	if r.Actor.Kind != "service" || r.Mode != "ai" {
		return nil
	}
	// Legacy plans deliberately fail closed at execution until an authorized
	// operator saves/enables them; never infer authority from their creator.
	var parent ExecutionGrant
	if p.ExecutionGrantID == "" {
		return nil
	}
	if err := scoped(tx, p.Scope).First(&parent, "id = ? AND plan_id = ? AND run_id = ?", p.ExecutionGrantID, p.ID, "").Error; err != nil {
		return err
	}
	if parent.RevokedAt != nil || parent.SnapshotHash != snapshotHash(r.Snapshot) {
		return nil
	}
	g := ExecutionGrant{ID: uuid.NewString(), Scope: p.Scope, PlanID: p.ID, RunID: r.ID,
		ParentID: parent.ID, Capability: "inspection:analyze", SnapshotHash: parent.SnapshotHash,
		AuthorizedBy: parent.AuthorizedBy, CreatedAt: s.Now(), ExpiresAt: r.DeadlineAt}
	if err := tx.Create(&g).Error; err != nil {
		return err
	}
	r.ExecutionGrantID = g.ID
	return nil
}

// ValidateServiceExecution authorizes only the stored run's exact cameras and
// scene parameters. The adapter additionally checks live device scope/state.
func (s *Service) ValidateServiceExecution(run Run, cameras []Camera) error {
	var current Run
	if err := scoped(s.DB, run.Scope).First(&current, "id = ?", run.ID).Error; err != nil {
		return ErrExecutionGrant
	}
	if current.Actor.Kind != "service" || current.Actor.UserID != 0 || current.Mode != "ai" ||
		(current.Status != "queued" && current.Status != "running") ||
		current.ExecutionGrantID == "" || current.ExecutionGrantID != run.ExecutionGrantID ||
		current.Snapshot != run.Snapshot || current.PlanID != run.PlanID {
		return ErrExecutionGrant
	}
	var grant, parent ExecutionGrant
	if err := scoped(s.DB, run.Scope).First(&grant, "id = ? AND run_id = ? AND plan_id = ?", current.ExecutionGrantID, current.ID, current.PlanID).Error; err != nil {
		return ErrExecutionGrant
	}
	if grant.RevokedAt != nil || grant.Capability != "inspection:analyze" || grant.ExpiresAt == nil || !s.Now().Before(*grant.ExpiresAt) || grant.SnapshotHash != snapshotHash(current.Snapshot) {
		return ErrExecutionGrant
	}
	if err := scoped(s.DB, run.Scope).First(&parent, "id = ? AND plan_id = ? AND run_id = ?", grant.ParentID, current.PlanID, "").Error; err != nil {
		return ErrExecutionGrant
	}
	if parent.RevokedAt != nil || parent.Capability != grant.Capability || parent.SnapshotHash != grant.SnapshotHash {
		return ErrExecutionGrant
	}
	allowed := map[string]string{}
	for _, camera := range current.Spec().Cameras {
		allowed[camera.Code] = encoded(camera)
	}
	for _, camera := range cameras {
		if allowed[camera.Code] != encoded(camera) {
			return ErrExecutionGrant
		}
	}
	return nil
}

// RevokePlanExecutionGrants revokes every revision, including in-flight runs.
// Saving/enabling again issues fresh authority. Disabling a schedule by itself
// continues to leave an already running snapshot intact.
func (s *Service) RevokePlanExecutionGrants(scope Scope, actor Actor, id string) error {
	if scope.TenantID == 0 || scope.ProjectID == 0 || actor.UserID == 0 {
		return ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var plan Plan
		if err := scoped(tx, scope).First(&plan, "id = ?", id).Error; err != nil {
			return err
		}
		if err := scoped(tx.Model(&ExecutionGrant{}), scope).Where("plan_id = ? AND revoked_at IS NULL", id).Updates(map[string]any{"revoked_at": s.Now(), "revoked_by": actor.UserID}).Error; err != nil {
			return err
		}
		return scoped(tx.Model(&Plan{}), scope).Where("id = ?", id).Updates(map[string]any{"execution_grant_id": "", "revision": gorm.Expr("revision + 1"), "updated_at": s.Now()}).Error
	})
}
