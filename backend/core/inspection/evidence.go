package inspection

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrEvidenceExpired = errors.New("inspection_evidence_expired")

const EvidenceRetention = 30 * 24 * time.Hour
const ReportRetention = 365 * 24 * time.Hour
const PDFRetention = 365 * 24 * time.Hour

type EvidenceAsset struct {
	ID        string `gorm:"primaryKey;size:36" json:"id"`
	Scope     `gorm:"embedded"`
	RunID     string    `gorm:"index" json:"run_id"`
	ItemID    string    `gorm:"index" json:"item_id"`
	Hash      string    `gorm:"index" json:"sha256"`
	MIME      string    `json:"mime"`
	Data      []byte    `gorm:"type:blob" json:"-"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

func (EvidenceAsset) TableName() string { return "inspection_evidence_assets" }

type evidenceReference struct {
	ReportID string `gorm:"primaryKey;size:36"`
	AssetID  string `gorm:"primaryKey;size:36;index"`
}

func (evidenceReference) TableName() string { return "inspection_evidence_references" }

func (s *Service) storeEvidence(tx *gorm.DB, r Run, i *Item) error {
	var value map[string]any
	if i.EvidenceJSON == "" || i.EvidenceJSON == "null" {
		return nil
	}
	if err := json.Unmarshal([]byte(i.EvidenceJSON), &value); err != nil {
		return err
	}
	if value == nil {
		return nil
	}
	if frames, ok := value["keyframes"].([]any); ok {
		for n, frame := range frames {
			child := Item{ID: i.ID, EvidenceJSON: encoded(frame)}
			if err := s.storeEvidence(tx, r, &child); err != nil {
				return err
			}
			var transformed any
			if err := json.Unmarshal([]byte(child.EvidenceJSON), &transformed); err != nil {
				return err
			}
			frames[n] = transformed
		}
		value["keyframes"] = frames
	}
	jpeg, ok := value["jpeg_base64"].(string)
	if !ok || jpeg == "" {
		delete(value, "jpeg_base64")
		i.EvidenceJSON = encoded(value)
		return nil
	}
	if len(jpeg) > 4*1024*1024 {
		return ErrInvalid
	}
	data, err := base64.StdEncoding.DecodeString(jpeg)
	if err != nil {
		return ErrInvalid
	}
	if len(data) < 2 || data[0] != 0xff || data[1] != 0xd8 {
		return ErrInvalid
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	var a EvidenceAsset
	err = scoped(tx, r.Scope).Omit("data").First(&a, "item_id = ? AND hash = ?", i.ID, hash).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		a = EvidenceAsset{ID: uuid.NewString(), Scope: r.Scope, RunID: r.ID, ItemID: i.ID, Hash: hash, MIME: "image/jpeg", Data: data, CreatedAt: s.Now()}
		if err = tx.Create(&a).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	delete(value, "jpeg_base64")
	value["asset_id"] = a.ID
	value["sha256"] = hash
	value["mime"] = a.MIME
	i.EvidenceJSON = encoded(value)
	return nil
}
func (s *Service) EvidenceAsset(scope Scope, id string) (EvidenceAsset, error) {
	var a EvidenceAsset
	err := scoped(s.DB, scope).First(&a, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return a, ErrEvidenceExpired
	}
	return a, err
}
func (s *Service) Evidence(scope Scope, runID, itemID string) (EvidenceAsset, error) {
	var r Run
	var i Item
	if err := scoped(s.DB, scope).First(&r, "id = ?", runID).Error; err != nil {
		return EvidenceAsset{}, err
	}
	if err := s.DB.First(&i, "id = ? AND run_id = ?", itemID, runID).Error; err != nil {
		return EvidenceAsset{}, err
	}
	var e map[string]any
	_ = json.Unmarshal([]byte(i.EvidenceJSON), &e)
	id, _ := e["asset_id"].(string)
	if id == "" {
		return EvidenceAsset{}, ErrEvidenceExpired
	}
	a, err := s.EvidenceAsset(scope, id)
	if err == nil && (a.RunID != runID || a.ItemID != itemID) {
		return EvidenceAsset{}, gorm.ErrRecordNotFound
	}
	return a, err
}
func (s *Service) pinEvidence(tx *gorm.DB, reportID string, items []Item) error {
	for _, item := range items {
		var e map[string]any
		_ = json.Unmarshal([]byte(item.EvidenceJSON), &e)
		ids := evidenceIDs(e)
		for _, id := range ids {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&evidenceReference{ReportID: reportID, AssetID: id}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func evidenceIDs(value any) []string {
	var ids []string
	switch v := value.(type) {
	case map[string]any:
		if id, ok := v["asset_id"].(string); ok && id != "" {
			ids = append(ids, id)
		}
		for key, child := range v {
			if key != "asset_id" {
				ids = append(ids, evidenceIDs(child)...)
			}
		}
	case []any:
		for _, child := range v {
			ids = append(ids, evidenceIDs(child)...)
		}
	}
	return ids
}
func (s *Service) migrateEvidence() error {
	for {
		var rows []Item
		if err := s.DB.Where("evidence_json LIKE ?", "%jpeg_base64%").Limit(50).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			break
		}
		if err := s.DB.Transaction(func(tx *gorm.DB) error {
			for _, i := range rows {
				var r Run
				if err := tx.First(&r, "id = ?", i.RunID).Error; err != nil {
					return err
				}
				if err := s.storeEvidence(tx, r, &i); err != nil {
					return err
				}
				if err := tx.Model(&Item{}).Where("id = ?", i.ID).Update("evidence_json", i.EvidenceJSON).Error; err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	// Pin legacy archives without rewriting their immutable snapshots or hashes.
	var versions []ReportVersion
	if err := s.DB.Select("id", "snapshot").Where("created_at >= ? AND NOT EXISTS (SELECT 1 FROM inspection_evidence_references er WHERE er.report_id = inspection_report_versions.id)", s.Now().Add(-ReportRetention)).Find(&versions).Error; err != nil {
		return err
	}
	for _, v := range versions {
		var report Report
		if err := json.Unmarshal([]byte(v.Snapshot), &report); err != nil {
			return err
		}
		if err := s.normalizeArchivedEvidence(v.ID, &report); err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) normalizeArchivedEvidence(id string, report *Report) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		for n := range report.Items {
			if err := s.storeEvidence(tx, report.Run, &report.Items[n]); err != nil {
				return err
			}
		}
		return s.pinEvidence(tx, id, report.Items)
	})
}

// Archives pin their exact assets for the report retention period. PDF and
// snapshot expiry are separate from raw image expiry; metadata remains usable.
func (s *Service) CleanupEvidence() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.DB.Transaction(func(tx *gorm.DB) error {
		now := s.Now()
		if err := tx.Where("created_at < ?", now.Add(-PDFRetention)).Delete(&ReportPDF{}).Error; err != nil {
			return err
		}
		if err := tx.Where("report_id IN (SELECT id FROM inspection_report_versions WHERE created_at < ?)", now.Add(-ReportRetention)).Delete(&evidenceReference{}).Error; err != nil {
			return err
		}
		if err := tx.Where("created_at < ?", now.Add(-ReportRetention)).Delete(&ReportVersion{}).Error; err != nil {
			return err
		}
		return tx.Where("created_at < ? AND NOT EXISTS (SELECT 1 FROM inspection_evidence_references er WHERE er.asset_id = inspection_evidence_assets.id)", now.Add(-EvidenceRetention)).Delete(&EvidenceAsset{}).Error
	})
}
