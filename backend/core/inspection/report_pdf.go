package inspection

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ReportPDF caches a presentation of an immutable snapshot in one language.
// Legacy bilingual ReportVersion.PDF bytes are never reused for these exports.
type ReportPDF struct {
	ReportID  string `gorm:"primaryKey;size:36"`
	Language  string `gorm:"primaryKey;size:2"`
	Scope     `gorm:"embedded"`
	PDF       []byte `gorm:"type:blob"`
	Hash      string
	CreatedAt time.Time
}

func (ReportPDF) TableName() string { return "inspection_report_pdfs" }

func (s *Service) LocalizedPDF(scope Scope, id, language string) ([]byte, error) {
	if language != "zh" && language != "en" {
		return nil, ErrInvalid
	}
	var row ReportPDF
	err := scoped(s.DB, scope).Select("pdf").First(&row, "report_id = ? AND language = ?", id, language).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return row.PDF, err
}

func (s *Service) ArchiveLocalizedPDF(scope Scope, id, language string, pdf []byte) ([]byte, error) {
	if (language != "zh" && language != "en") || len(pdf) < 5 || len(pdf) > 20*1024*1024 || string(pdf[:5]) != "%PDF-" {
		return nil, ErrInvalid
	}
	var data []byte
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		var version ReportVersion
		if err := scoped(tx, scope).Select("id").First(&version, "id = ?", id).Error; err != nil {
			return err
		}
		sum := sha256.Sum256(pdf)
		row := ReportPDF{ReportID: id, Language: language, Scope: scope, PDF: pdf, Hash: hex.EncodeToString(sum[:]), CreatedAt: s.Now()}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
			return err
		}
		var stored ReportPDF
		if err := scoped(tx, scope).Select("pdf").First(&stored, "report_id = ? AND language = ?", id, language).Error; err != nil {
			return err
		}
		data = stored.PDF
		return nil
	})
	return data, err
}
