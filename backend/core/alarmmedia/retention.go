package alarmmedia

import (
	"errors"
	"io/fs"
	"noyo/core/config"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

var storagePolicy atomic.Pointer[config.AlarmMediaConfig]
var pins = make(map[string]int)

// Configure applies a persisted policy without restarting recording workers.
// It does not delete evidence; reclamation only runs under storage pressure.
func Configure(policy config.AlarmMediaConfig) error {
	if err := policy.Validate(); err != nil {
		return err
	}
	storagePolicy.Store(&policy)
	return nil
}

func currentPolicy() config.AlarmMediaConfig {
	if policy := storagePolicy.Load(); policy != nil {
		return *policy
	}
	return config.DefaultAlarmMediaConfig()
}

// Pin protects evidence while an upload or a playback request is using it.
func Pin(id string) (func(), bool) {
	mu.Lock()
	defer mu.Unlock()
	r, err := Load(id)
	if err != nil || r.Status == "expired" {
		return nil, false
	}
	pins[id]++
	var once sync.Once
	return func() {
		once.Do(func() {
			mu.Lock()
			defer mu.Unlock()
			pins[id]--
			if pins[id] == 0 {
				delete(pins, id)
			}
		})
	}, true
}

func storageUsage() (int64, error) {
	var used int64
	err := filepath.WalkDir(Root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			used += info.Size()
		}
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		if _, rootErr := os.Stat(Root); errors.Is(rootErr, os.ErrNotExist) {
			return 0, nil
		}
	}
	return used, err
}

func hasCapacityAt(now time.Time) bool {
	policy := currentPolicy()
	return hasCapacityWithinLimit(now, policy.MaxStorageGiB<<30, policy.RetentionDays)
}

func hasCapacityWithinLimit(now time.Time, maxBytes int64, retentionDays int) bool {
	mu.Lock()
	defer mu.Unlock()
	threshold := maxBytes - MaxFileBytes
	used, err := storageUsage()
	if err != nil {
		return false
	}
	if used < threshold {
		return true
	}
	cutoff := now.AddDate(0, 0, -retentionDays).UnixMilli()
	var candidates []Record
	entries, err := os.ReadDir(Root)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !ValidID(entry.Name()) {
			continue
		}
		r, err := Load(entry.Name())
		if err != nil || r.ID != entry.Name() || r.OccurredAt <= 0 || r.OccurredAt >= cutoff || pins[r.ID] > 0 {
			continue
		}
		switch r.Status {
		case "ready", "partial", "failed", "expired":
			candidates = append(candidates, r)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].OccurredAt == candidates[j].OccurredAt {
			return candidates[i].ID < candidates[j].ID
		}
		return candidates[i].OccurredAt < candidates[j].OccurredAt
	})
	for _, r := range candidates {
		freed, err := expireRecording(r)
		if err != nil {
			return false
		}
		used -= freed
		if used < threshold {
			return true
		}
	}
	return false
}

// expireRecording keeps the manifest so an alarm still explains why its video
// is unavailable. Only allowlisted media files inside the recording are removed.
// Callers hold mu and have excluded active jobs and pinned readers.
func expireRecording(r Record) (int64, error) {
	root, err := filepath.Abs(Root)
	if err != nil {
		return 0, err
	}
	dir := filepath.Join(root, r.ID)
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel != r.ID || !ValidID(r.ID) {
		return 0, errors.New("invalid recording directory")
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return 0, errors.New("unsafe recording directory")
	}
	var files []struct {
		path string
		size int64
	}
	for _, kind := range []string{"preview.mp4", "source.ts", "preview.mp4.part", "source.ts.part", "overlay.ass", "upload.json"} {
		path := filepath.Join(dir, kind)
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return 0, err
		}
		if info.Mode().IsRegular() {
			files = append(files, struct {
				path string
				size int64
			}{path, info.Size()})
		}
	}
	if len(files) == 0 {
		return 0, nil // Preserve the original reason when no video was ever captured.
	}
	// Persist expiration before removing files. A manifest write failure must
	// never leave an apparently playable recording whose video was deleted.
	r.Status = "expired"
	r.Reason = "retention_expired"
	if err := save(r); err != nil {
		return 0, err
	}
	var freed int64
	for _, file := range files {
		if err := os.Remove(file.path); err != nil {
			return freed, err
		}
		freed += file.size
	}
	return freed, nil
}
