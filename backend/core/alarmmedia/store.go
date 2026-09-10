// Package alarmmedia stores immutable video evidence independently of alarm lifecycle.
package alarmmedia

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const MaxFileBytes int64 = 128 << 20
const MaxStorageBytes int64 = 10 << 30

type Record struct {
	TenantID   uint   `json:"tenant_id"`
	ProjectID  uint   `json:"project_id"`
	ID         string `json:"id"`
	DeviceCode string `json:"device_code"`
	Status     string `json:"status"`
	Reason     string `json:"reason,omitempty"`
	OccurredAt int64  `json:"occurred_at"`
	StartAt    int64  `json:"start_at"`
	EndAt      int64  `json:"end_at"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Partial    bool   `json:"partial"`
	Uploaded   bool   `json:"uploaded"`
	Remote     bool   `json:"remote"`
	UpdatedAt  int64  `json:"updated_at"`
}

var Root = filepath.Join("data", "alarm-media")
var mu sync.Mutex

func ValidID(id string) bool { u, e := uuid.Parse(id); return e == nil && u.String() == id }
func Path(id, kind string) (string, error) {
	if !ValidID(id) {
		return "", errors.New("invalid recording id")
	}
	switch kind {
	case "record.json", "source.ts", "preview.mp4", "overlay.ass", "source.ts.part", "preview.mp4.part", "upload.json":
	default:
		return "", errors.New("invalid media kind")
	}
	return filepath.Join(Root, id, kind), nil
}
func Load(id string) (Record, error) {
	p, e := Path(id, "record.json")
	if e != nil {
		return Record{}, e
	}
	b, e := os.ReadFile(p)
	if e != nil {
		return Record{}, e
	}
	var r Record
	e = json.Unmarshal(b, &r)
	return r, e
}
func Save(r Record) error {
	mu.Lock()
	defer mu.Unlock()
	return save(r)
}
func save(r Record) error {
	// Once an alarm binds this resource, background completion must preserve
	// its authoritative scope even when the job holds an older manifest copy.
	if old, e := Load(r.ID); e == nil && old.DeviceCode == r.DeviceCode && old.TenantID != 0 {
		r.TenantID = old.TenantID
		r.ProjectID = old.ProjectID
	}
	p, e := Path(r.ID, "record.json")
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		return e
	}
	r.UpdatedAt = time.Now().UnixMilli()
	b, e := json.Marshal(r)
	if e != nil {
		return e
	}
	if e = os.WriteFile(p+".tmp", b, 0600); e != nil {
		return e
	}
	return os.Rename(p+".tmp", p)
}
func Update(id string, f func(*Record)) error {
	mu.Lock()
	defer mu.Unlock()
	r, e := Load(id)
	if e != nil {
		return e
	}
	f(&r)
	return save(r)
}

// List reads only manifests; video bytes are never buffered by discovery.
func List() []Record {
	entries, _ := os.ReadDir(Root)
	out := []Record{}
	for _, d := range entries {
		if d.IsDir() && ValidID(d.Name()) {
			if r, e := Load(d.Name()); e == nil {
				out = append(out, r)
			}
		}
	}
	return out
}
func HasCapacity() bool {
	var n int64
	_ = filepath.WalkDir(Root, func(p string, d os.DirEntry, e error) error {
		if e == nil && !d.IsDir() {
			if st, e := d.Info(); e == nil {
				n += st.Size()
			}
		}
		if n > MaxStorageBytes {
			return errors.New("quota")
		}
		return nil
	})
	return n < MaxStorageBytes-MaxFileBytes
}

// Recover marks interrupted local jobs explicitly instead of showing eternal capture.
func Recover() {
	PruneOriginals()
	for _, r := range List() {
		if !r.Remote && (r.Status == "capturing" || r.Status == "rendering") {
			r.Status = "failed"
			r.Reason = "interrupted"
			_ = Save(r)
		}
	}
}

// PruneOriginals removes legacy duplicate videos only after a completed MP4
// exists. Failed recordings without a replacement keep their original evidence.
func PruneOriginals() {
	mu.Lock()
	defer mu.Unlock()
	for _, r := range List() {
		if r.Status != "ready" && r.Status != "partial" {
			continue
		}
		preview, err := Path(r.ID, "preview.mp4")
		if err != nil {
			continue
		}
		if st, err := os.Stat(preview); err != nil || !st.Mode().IsRegular() || st.Size() == 0 {
			continue
		}
		for _, kind := range []string{"source.ts", "source.ts.part", "overlay.ass"} {
			path, err := Path(r.ID, kind)
			if err == nil {
				_ = os.Remove(path)
			}
		}
	}
}
