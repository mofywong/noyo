package aiassets

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

const importPreviewLifetime = 24 * time.Hour

// DiscardImport releases an unsubmitted preview. Job-owned files remain available
// for installation and retry, including after cancellation or a failed attempt.
func (s *Service) DiscardImport(id string) error {
	if !safeID(id) {
		return errCode("invalid_request", "invalid import identifier")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errCode("conflict", "installer is closed")
	}
	if _, ok := s.state.Imports[id]; !ok {
		return nil
	}
	for _, job := range s.state.Jobs {
		if job.PlanID == id {
			return errCode("conflict", "import belongs to an installation job")
		}
	}
	return s.removeImportPreviewsLocked([]string{id})
}

func (s *Service) expireImportPreviewsLocked(now time.Time) error {
	owned := make(map[string]bool, len(s.state.Jobs))
	for _, job := range s.state.Jobs {
		owned[job.PlanID] = true
	}
	var expired []string
	for id := range s.state.Imports {
		plan, ok := s.state.Plans[id]
		if !owned[id] && (!ok || !now.Before(plan.CreatedAt.Add(importPreviewLifetime))) {
			expired = append(expired, id)
		}
	}
	return s.removeImportPreviewsLocked(expired)
}

func (s *Service) removeImportPreviewsLocked(ids []string) error {
	removed := make(map[string]Plan, len(ids))
	var firstErr error
	for _, id := range ids {
		if err := os.RemoveAll(filepath.Join(s.dir, "imports", id)); err != nil {
			if firstErr == nil {
				firstErr = errCode("storage_error", "cannot remove import preview")
			}
			continue // Retain metadata so the next sweep retries failed removals.
		}
		removed[id] = s.state.Plans[id]
		delete(s.state.Imports, id)
		delete(s.state.Plans, id)
	}
	if len(removed) != 0 {
		if err := s.saveLocked(); err != nil {
			for id, plan := range removed {
				s.state.Imports[id] = id
				if plan.ID != "" {
					s.state.Plans[id] = plan
				}
			}
			return err
		}
	}
	return firstErr
}

// Only run during startup: a live upload has not yet registered its directory.
func (s *Service) cleanOrphanImportsLocked() {
	entries, err := os.ReadDir(filepath.Join(s.dir, "imports"))
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if _, ok := s.state.Imports[name]; ok {
			continue
		}
		if safeID(name) || strings.HasPrefix(name, ".upload-") {
			_ = os.RemoveAll(filepath.Join(s.dir, "imports", name))
		}
	}
}
