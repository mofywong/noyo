package semanticreview

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

// Exchange contains provider bodies only. Authorization headers and image
// bytes are never part of the audit record.
type Exchange struct {
	Provider   string          `json:"provider"`
	Model      string          `json:"model"`
	Status     string          `json:"status"`
	RequestID  string          `json:"request_id,omitempty"`
	Request    json.RawMessage `json:"request,omitempty"`
	Response   json.RawMessage `json:"response,omitempty"`
	HTTPStatus int             `json:"http_status,omitempty"`
	Error      string          `json:"error,omitempty"`
}

type exchangeRecorderKey struct{}

func WithExchangeRecorder(ctx context.Context, recorder func(Exchange) error) context.Context {
	return context.WithValue(ctx, exchangeRecorderKey{}, recorder)
}

func RecordExchange(ctx context.Context, exchange Exchange) error {
	if recorder, ok := ctx.Value(exchangeRecorderKey{}).(func(Exchange) error); ok {
		return recorder(exchange)
	}
	return nil
}

type ReviewRecord struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Status    string    `json:"status"`
	Request   Candidate `json:"request"`
	Result    *Result   `json:"result"`
	Exchange  *Exchange `json:"exchange,omitempty"`
}

// DeviceCodes must be resolved by the caller's device permissions. An empty
// slice grants no access, including when the caller is a system administrator.
type RecordQuery struct {
	DeviceCodes []string
	DeviceCode  string
	Status      string
	ID          string
	Page        int
	PageSize    int
}

func ListRecords(db *gorm.DB, filter RecordQuery) ([]ReviewRecord, int64, error) {
	out := []ReviewRecord{}
	if db == nil {
		return out, 0, errors.New("semantic review database unavailable")
	}
	if len(filter.DeviceCodes) == 0 || !db.Migrator().HasTable(&job{}) {
		return out, 0, nil
	}
	// Also bind historical rows to the current owner. Moving a device between
	// projects or tenants must not transfer access to its earlier review data.
	query := reviewRecordQuery(db, filter)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return out, 0, err
	}
	page, size := max(1, filter.Page), filter.PageSize
	if size <= 0 || size > 100 {
		size = 20
	}
	var jobs []job
	if err := query.Select("semantic_review_jobs.*").Order("semantic_review_jobs.created_at DESC, semantic_review_jobs.id DESC").Offset((page - 1) * size).Limit(size).Find(&jobs).Error; err != nil {
		return out, 0, err
	}
	for _, j := range jobs {
		r := ReviewRecord{ID: j.ID, CreatedAt: j.CreatedAt, Status: j.Status}
		if err := json.Unmarshal(j.Payload, &r.Request); err != nil {
			return out, 0, err
		}
		available := map[string]bool{}
		if filter.ID != "" && db.Migrator().HasTable(&recordEvidence{}) {
			var ids []string
			if err := db.Model(&recordEvidence{}).Where("job_id = ?", j.ID).Pluck("frame_id", &ids).Error; err != nil {
				return out, 0, err
			}
			for _, id := range ids {
				available[id] = true
			}
		}
		for i := range r.Request.Evidence {
			if filter.ID != "" {
				present := available[r.Request.Evidence[i].ID] || len(r.Request.Evidence[i].Data) > 0
				r.Request.Evidence[i].Available = &present
			} else {
				r.Request.Evidence[i].Available = nil
			}
			r.Request.Evidence[i].Data = nil
		}
		if len(j.Result) > 0 {
			if err := json.Unmarshal(j.Result, &r.Result); err != nil {
				return out, 0, err
			}
		}
		// Provider bodies are loaded only when a record is opened.
		if filter.ID != "" && db.Migrator().HasTable(&attempt{}) {
			var a attempt
			err := db.Where("job_id = ?", j.ID).First(&a).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return out, 0, err
			}
			if err == nil {
				r.Exchange = &Exchange{Provider: a.Provider, Model: a.Model, Status: a.Status, RequestID: a.RequestID, Request: json.RawMessage(a.Request), Response: json.RawMessage(a.Response), HTTPStatus: a.HTTPStatus, Error: a.ErrorMessage}
			}
		}
		out = append(out, r)
	}
	return out, total, nil
}
