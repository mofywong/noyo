package semanticreview

import (
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

// Images are kept separately from queue payloads and provider audit bodies,
// so completed records retain exactly the evidence submitted for review.
type recordEvidence struct {
	ID        string `gorm:"primaryKey;size:64"`
	JobID     string `gorm:"index;size:64"`
	FrameID   string
	MIME      string
	Data      []byte
	CreatedAt time.Time
}

func (recordEvidence) TableName() string { return "semantic_review_evidence" }

func archiveRecordEvidence(db *gorm.DB, jobID string, at time.Time, frames []Frame) error {
	for _, frame := range frames {
		if len(frame.Data) == 0 {
			continue
		}
		row := recordEvidence{ID: digest([]string{jobID, frame.ID}), JobID: jobID, FrameID: frame.ID, MIME: frame.MIME, Data: frame.Data, CreatedAt: at}
		if err := db.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func reviewRecordQuery(db *gorm.DB, filter RecordQuery) *gorm.DB {
	query := db.Model(&job{}).Joins("JOIN devices ON devices.code = semantic_review_jobs.device_code AND devices.tenant_id = semantic_review_jobs.tenant_id AND devices.project_id = semantic_review_jobs.project_id").Where("semantic_review_jobs.device_code IN ?", filter.DeviceCodes)
	if filter.DeviceCode != "" {
		query = query.Where("semantic_review_jobs.device_code = ?", filter.DeviceCode)
	}
	if filter.Status != "" {
		query = query.Where("semantic_review_jobs.status = ?", filter.Status)
	}
	if filter.ID != "" {
		query = query.Where("semantic_review_jobs.id = ?", filter.ID)
	}
	return query
}

// RecordEvidence applies the same current-device and historical-owner scope as
// ListRecords. An ID or frame ID never grants access on its own.
func RecordEvidence(db *gorm.DB, filter RecordQuery, frameID string) (Frame, error) {
	if db == nil {
		return Frame{}, errors.New("semantic review database unavailable")
	}
	if filter.ID == "" || frameID == "" || len(filter.DeviceCodes) == 0 || !db.Migrator().HasTable(&job{}) {
		return Frame{}, gorm.ErrRecordNotFound
	}
	var j job
	if err := reviewRecordQuery(db, filter).Select("semantic_review_jobs.*").First(&j).Error; err != nil {
		return Frame{}, err
	}
	if db.Migrator().HasTable(&recordEvidence{}) {
		var row recordEvidence
		err := db.Where("job_id = ? AND frame_id = ?", j.ID, frameID).First(&row).Error
		if err == nil && len(row.Data) > 0 {
			return Frame{ID: row.FrameID, MIME: row.MIME, Data: row.Data}, nil
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return Frame{}, err
		}
	}
	// Pre-upgrade queued records may still contain their original image data.
	var candidate Candidate
	if err := json.Unmarshal(j.Payload, &candidate); err != nil {
		return Frame{}, err
	}
	for _, frame := range candidate.Evidence {
		if frame.ID == frameID && len(frame.Data) > 0 {
			return frame, nil
		}
	}
	return Frame{}, gorm.ErrRecordNotFound
}
