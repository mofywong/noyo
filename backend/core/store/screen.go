package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"noyo/core/screen"
)

var ErrScreenConflict = errors.New("screen draft version has changed")
var ErrScreenRequestConflict = errors.New("request id was already used with different parameters")

type IOCScreen struct {
	ID                     uint           `gorm:"primaryKey" json:"id"`
	TenantID               uint           `gorm:"index:idx_ioc_screen_scope;not null" json:"tenantId"`
	ProjectID              uint           `gorm:"index:idx_ioc_screen_scope;not null" json:"projectId"`
	Name                   string         `gorm:"size:128;not null" json:"name"`
	MetadataVersion        uint64         `gorm:"not null;default:1" json:"metadataVersion"`
	DraftJSON              string         `gorm:"type:text;not null" json:"-"`
	DraftVersion           uint64         `gorm:"not null;default:1" json:"draftVersion"`
	PublishedReleaseID     *uint          `json:"publishedReleaseId"`
	CreatedBy              uint           `gorm:"uniqueIndex:idx_ioc_screen_copy_request" json:"createdBy"`
	UpdatedBy              uint           `json:"updatedBy"`
	CreatedAt              time.Time      `json:"createdAt"`
	UpdatedAt              time.Time      `json:"updatedAt"`
	DeletedAt              gorm.DeletedAt `gorm:"index" json:"-"`
	LastRestoreRequestID   string         `gorm:"size:128" json:"-"`
	LastRestoreReleaseID   uint           `json:"-"`
	LastRestoreBaseVersion uint64         `json:"-"`
	LastRestoreVersion     uint64         `json:"-"`
	CopySourceID           uint           `gorm:"uniqueIndex:idx_ioc_screen_copy_request" json:"-"`
	CopyRequestID          *string        `gorm:"size:128;uniqueIndex:idx_ioc_screen_copy_request" json:"-"`
	CopyRequestHash        string         `gorm:"size:64" json:"-"`
}

func (IOCScreen) TableName() string { return "ioc_screens" }

type IOCScreenRelease struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	ScreenID           uint      `gorm:"not null;uniqueIndex:idx_ioc_screen_release_no;uniqueIndex:idx_ioc_screen_publish_request" json:"screenId"`
	ReleaseNo          uint64    `gorm:"not null;uniqueIndex:idx_ioc_screen_release_no" json:"releaseNo"`
	SourceDraftVersion uint64    `gorm:"not null" json:"sourceDraftVersion"`
	PublishRequestID   string    `gorm:"size:128;not null;uniqueIndex:idx_ioc_screen_publish_request" json:"requestId"`
	RequestHash        string    `gorm:"size:64;not null" json:"-"`
	ContentJSON        string    `gorm:"type:text;not null" json:"-"`
	ContentHash        string    `gorm:"size:64;not null" json:"contentHash"`
	SchemaVersion      string    `json:"schemaVersion"`
	EngineBuildID      string    `json:"engineBuildId"`
	UpstreamSHA        string    `json:"upstreamSha"`
	CreatedBy          uint      `json:"createdBy"`
	CreatedAt          time.Time `json:"createdAt"`
}

func (IOCScreenRelease) TableName() string { return "ioc_screen_releases" }

type IOCScreenResourceRef struct {
	ID           uint   `gorm:"primaryKey"`
	TenantID     uint   `gorm:"index:idx_ioc_ref_scope;not null"`
	ProjectID    uint   `gorm:"index:idx_ioc_ref_scope;not null"`
	OwnerKind    string `gorm:"size:16;not null;index:idx_ioc_ref_owner"`
	OwnerID      uint   `gorm:"not null;index:idx_ioc_ref_owner"`
	ResourceType string `gorm:"size:16;not null;index:idx_ioc_ref_resource"`
	ResourceID   string `gorm:"size:128;not null;index:idx_ioc_ref_resource"`
	FieldKey     string `gorm:"size:128"`
}

func (IOCScreenResourceRef) TableName() string { return "ioc_screen_resource_refs" }

type IOCScreenAsset struct {
	Name       string         `gorm:"size:128" json:"name"`
	ID         uint           `gorm:"primaryKey" json:"id"`
	TenantID   uint           `gorm:"index:idx_ioc_asset_scope;not null" json:"tenantId"`
	ProjectID  uint           `gorm:"index:idx_ioc_asset_scope;not null" json:"projectId"`
	ScreenID   uint           `gorm:"index;not null" json:"screenId"`
	StorageKey string         `gorm:"size:255;not null" json:"-"`
	SHA256     string         `gorm:"size:64;not null" json:"sha256"`
	MIME       string         `gorm:"size:64;not null" json:"mime"`
	Size       int64          `json:"size"`
	Width      int            `json:"width"`
	Height     int            `json:"height"`
	Uploader   uint           `json:"uploader"`
	CreatedAt  time.Time      `json:"createdAt"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

func (IOCScreenAsset) TableName() string { return "ioc_screen_assets" }

func ScreenFind(db *gorm.DB, tenantID, projectID, id uint) (*IOCScreen, error) {
	var row IOCScreen
	if tenantID == 0 || projectID == 0 || id == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	err := db.Where("id = ? AND tenant_id = ? AND project_id = ?", id, tenantID, projectID).First(&row).Error
	return &row, err
}

func ScreenDocumentJSON(doc screen.Envelope) (string, error) {
	b, err := json.Marshal(doc)
	return string(b), err
}

func ScreenCreate(db *gorm.DB, tenantID, projectID, actor uint, name string, doc screen.Envelope, refs []screen.ResourceRef) (*IOCScreen, error) {
	name = strings.TrimSpace(name)
	if tenantID == 0 || projectID == 0 || name == "" || len([]rune(name)) > 128 {
		return nil, errors.New("a project and a name of 1–128 characters are required")
	}
	data, err := ScreenDocumentJSON(doc)
	if err != nil {
		return nil, err
	}
	row := &IOCScreen{TenantID: tenantID, ProjectID: projectID, Name: name, MetadataVersion: 1, DraftJSON: data, DraftVersion: 1, CreatedBy: actor, UpdatedBy: actor}
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(row).Error; err != nil {
			return err
		}
		return screenReplaceRefs(tx, row, "draft", row.ID, refs)
	})
	return row, err
}

func ScreenSaveDraft(db *gorm.DB, tenantID, projectID, id, actor uint, base uint64, doc screen.Envelope, refs []screen.ResourceRef) (*IOCScreen, error) {
	data, err := ScreenDocumentJSON(doc)
	if err != nil {
		return nil, err
	}
	var row *IOCScreen
	err = db.Transaction(func(tx *gorm.DB) error {
		var err error
		row, err = ScreenFind(tx, tenantID, projectID, id)
		if err != nil {
			return err
		}
		result := tx.Model(&IOCScreen{}).Where("id = ? AND tenant_id = ? AND project_id = ? AND draft_version = ?", id, tenantID, projectID, base).Updates(map[string]interface{}{"draft_json": data, "draft_version": base + 1, "updated_by": actor, "updated_at": time.Now()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrScreenConflict
		}
		row.DraftJSON = data
		row.DraftVersion = base + 1
		row.UpdatedBy = actor
		return screenReplaceRefs(tx, row, "draft", id, refs)
	})
	return row, err
}

// ScreenRefAuthorizer checks the candidate outside the write transaction: existing
// device authorization helpers may need the same DB connection. New writes then
// compare that exact candidate inside their transaction, so authorization cannot
// accidentally apply to a different draft. Completed request retries are returned
// before either candidate errors or version checks are applied.
type ScreenRefAuthorizer func(*IOCScreen, []screen.ResourceRef) error

func screenAuthorizedCandidate(db *gorm.DB, tenantID, projectID, id uint, authorize []ScreenRefAuthorizer) (*IOCScreen, screen.Envelope, []screen.ResourceRef, error) {
	row, err := ScreenFind(db, tenantID, projectID, id)
	if err != nil {
		return row, screen.Envelope{}, nil, err
	}
	doc, refs, err := screen.Validate([]byte(row.DraftJSON))
	if err == nil && len(authorize) > 0 && authorize[0] != nil {
		err = authorize[0](row, refs)
	}
	return row, doc, refs, err
}

func ScreenPublish(db *gorm.DB, tenantID, projectID, id, actor uint, version uint64, requestID string, authorize ...ScreenRefAuthorizer) (*IOCScreenRelease, error) {
	if version == 0 || !validScreenRequestID(requestID) {
		return nil, errors.New("draftVersion and a requestId of 8–128 characters are required")
	}
	hash := screenHash(fmt.Sprintf("publish:%d", version))
	candidate, doc, refs, candidateErr := screenAuthorizedCandidate(db, tenantID, projectID, id, authorize)
	var release IOCScreenRelease
	err := db.Transaction(func(tx *gorm.DB) error {
		row, err := ScreenFind(tx, tenantID, projectID, id)
		if err != nil {
			return err
		}
		err = tx.Where("screen_id = ? AND publish_request_id = ?", id, requestID).First(&release).Error
		if err == nil {
			if release.RequestHash != hash {
				return ErrScreenRequestConflict
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if row.DraftVersion != version {
			return ErrScreenConflict
		}
		if candidateErr != nil {
			return candidateErr
		}
		if candidate.DraftVersion != row.DraftVersion || candidate.DraftJSON != row.DraftJSON {
			return ErrScreenConflict
		}
		var last IOCScreenRelease
		err = tx.Where("screen_id = ?", id).Order("release_no DESC").First(&last).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		release = IOCScreenRelease{ScreenID: id, ReleaseNo: last.ReleaseNo + 1, SourceDraftVersion: version, PublishRequestID: requestID, RequestHash: hash, ContentJSON: row.DraftJSON, ContentHash: screenHash(row.DraftJSON), SchemaVersion: doc.SchemaVersion, EngineBuildID: doc.Engine.BuildID, UpstreamSHA: doc.Engine.UpstreamSHA, CreatedBy: actor}
		if err = tx.Create(&release).Error; err != nil {
			return err
		}
		if err = screenReplaceRefs(tx, row, "release", release.ID, refs); err != nil {
			return err
		}
		result := tx.Model(&IOCScreen{}).Where("id = ? AND draft_version = ?", id, version).Updates(map[string]interface{}{"published_release_id": release.ID, "updated_by": actor, "updated_at": time.Now()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrScreenConflict
		}
		return nil
	})
	return &release, err
}

func ScreenRename(db *gorm.DB, tenantID, projectID, id, actor uint, version uint64, name string) (*IOCScreen, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 128 || version == 0 {
		return nil, errors.New("name and metadata version are required")
	}
	var row *IOCScreen
	err := db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&IOCScreen{}).Where("id = ? AND tenant_id = ? AND project_id = ? AND metadata_version = ?", id, tenantID, projectID, version).
			Updates(map[string]interface{}{"name": name, "metadata_version": version + 1, "updated_by": actor, "updated_at": time.Now()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrScreenConflict
		}
		var err error
		row, err = ScreenFind(tx, tenantID, projectID, id)
		return err
	})
	return row, err
}

// Copy request identity is scoped to the source and actor. Nullable request IDs
// keep ordinary screens out of the unique index; deleted copies still reserve
// their key, preventing a retry from recreating a deliberately deleted screen.
func ScreenCopy(db *gorm.DB, tenantID, projectID, id, actor uint, version uint64, name, requestID string, authorize ...ScreenRefAuthorizer) (*IOCScreen, error) {
	name = strings.TrimSpace(name)
	if version == 0 || !validScreenRequestID(requestID) || len([]rune(name)) > 128 {
		return nil, errors.New("draftVersion, name and requestId are invalid")
	}
	parameters, _ := json.Marshal(struct {
		Version uint64
		Name    string
	}{version, name})
	hash := screenHash(string(parameters))
	findCompleted := func(tx *gorm.DB) (*IOCScreen, error) {
		var copy IOCScreen
		err := tx.Unscoped().Where("tenant_id = ? AND project_id = ? AND copy_source_id = ? AND created_by = ? AND copy_request_id = ?", tenantID, projectID, id, actor, requestID).First(&copy).Error
		if err == nil && copy.CopyRequestHash != hash {
			return nil, ErrScreenRequestConflict
		}
		if err == nil && copy.DeletedAt.Valid {
			return &copy, gorm.ErrRecordNotFound
		}
		return &copy, err
	}
	candidate, _, refs, candidateErr := screenAuthorizedCandidate(db, tenantID, projectID, id, authorize)
	var copied *IOCScreen
	createFailed := false
	err := db.Transaction(func(tx *gorm.DB) error {
		row, err := ScreenFind(tx, tenantID, projectID, id)
		if err != nil {
			return err
		}
		if existing, err := findCompleted(tx); err == nil {
			copied = existing
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		} else if existing != nil && existing.ID != 0 {
			return gorm.ErrRecordNotFound
		}
		if row.DraftVersion != version {
			return ErrScreenConflict
		}
		if candidateErr != nil {
			return candidateErr
		}
		if candidate.DraftVersion != row.DraftVersion || candidate.DraftJSON != row.DraftJSON {
			return ErrScreenConflict
		}
		copyName := name
		if copyName == "" {
			copyName = row.Name + " (Copy)"
			if runes := []rune(copyName); len(runes) > 128 {
				copyName = string(runes[:128])
			}
		}
		copied = &IOCScreen{TenantID: tenantID, ProjectID: projectID, Name: copyName, MetadataVersion: 1, DraftJSON: row.DraftJSON, DraftVersion: 1, CreatedBy: actor, UpdatedBy: actor, CopySourceID: id, CopyRequestID: &requestID, CopyRequestHash: hash}
		if err := tx.Create(copied).Error; err != nil {
			createFailed = true
			return err
		}
		return screenReplaceRefs(tx, copied, "draft", copied.ID, refs)
	})
	if err != nil && createFailed {
		// A concurrent duplicate can win the unique constraint. Its committed
		// result is the same idempotent outcome after this transaction rolls back.
		if existing, lookupErr := findCompleted(db); lookupErr == nil || errors.Is(lookupErr, ErrScreenRequestConflict) {
			return existing, lookupErr
		}
	}
	return copied, err
}

func ScreenRestore(db *gorm.DB, tenantID, projectID, id, releaseID, actor uint, base uint64, requestID string) (*IOCScreen, error) {
	if base == 0 || !validScreenRequestID(requestID) {
		return nil, errors.New("draftVersion and a requestId of 8–128 characters are required")
	}
	var row *IOCScreen
	err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		row, err = ScreenFind(tx, tenantID, projectID, id)
		if err != nil {
			return err
		}
		if row.LastRestoreRequestID == requestID {
			if row.LastRestoreReleaseID != releaseID || row.LastRestoreBaseVersion != base {
				return ErrScreenRequestConflict
			}
			if row.DraftVersion != row.LastRestoreVersion {
				return ErrScreenConflict
			}
			return nil
		}
		if row.DraftVersion != base {
			return ErrScreenConflict
		}
		var release IOCScreenRelease
		if err = tx.Where("id = ? AND screen_id = ?", releaseID, id).First(&release).Error; err != nil {
			return err
		}
		_, refs, err := screen.Validate([]byte(release.ContentJSON))
		if err != nil {
			return err
		}
		result := tx.Model(&IOCScreen{}).Where("id = ? AND draft_version = ?", id, base).Updates(map[string]interface{}{"draft_json": release.ContentJSON, "draft_version": base + 1, "updated_by": actor, "updated_at": time.Now(), "last_restore_request_id": requestID, "last_restore_release_id": releaseID, "last_restore_base_version": base, "last_restore_version": base + 1})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrScreenConflict
		}
		row.DraftJSON = release.ContentJSON
		row.DraftVersion = base + 1
		return screenReplaceRefs(tx, row, "draft", id, refs)
	})
	return row, err
}

func screenReplaceRefs(tx *gorm.DB, row *IOCScreen, kind string, ownerID uint, refs []screen.ResourceRef) error {
	if err := tx.Where("owner_kind = ? AND owner_id = ? AND tenant_id = ? AND project_id = ?", kind, ownerID, row.TenantID, row.ProjectID).Delete(&IOCScreenResourceRef{}).Error; err != nil {
		return err
	}
	for _, ref := range refs {
		if err := tx.Create(&IOCScreenResourceRef{TenantID: row.TenantID, ProjectID: row.ProjectID, OwnerKind: kind, OwnerID: ownerID, ResourceType: ref.Type, ResourceID: ref.ID, FieldKey: ref.FieldKey}).Error; err != nil {
			return err
		}
	}
	return nil
}

func screenHash(text string) string {
	hash := sha256.Sum256([]byte(text))
	return hex.EncodeToString(hash[:])
}
func validScreenRequestID(id string) bool {
	return len(id) >= 8 && len(id) <= 128 && strings.TrimSpace(id) == id
}
