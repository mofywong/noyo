package aiassets

import (
	"os"
	"sort"
	"time"
)

// HistoryActor is supplied by trusted API authentication, never archive content.
// It travels with a persisted job so asynchronous installation retains its user.
type HistoryActor struct {
	UserID   uint   `json:"userId,omitempty"`
	TenantID uint   `json:"tenantId,omitempty"`
	Username string `json:"username,omitempty"`
	Name     string `json:"name,omitempty"`
	IP       string `json:"ip,omitempty"`
}

type HistoryEvent struct {
	ID              string       `json:"id"`
	PackageID       string       `json:"packageId"`
	Version         string       `json:"version"`
	PreviousVersion string       `json:"previousVersion,omitempty"`
	Action          string       `json:"action"`
	Actor           HistoryActor `json:"actor"`
	CreatedAt       time.Time    `json:"createdAt"`
}

func operationActor(actors []HistoryActor) HistoryActor {
	if len(actors) == 0 {
		return HistoryActor{Name: "system"}
	}
	actor := actors[0]
	if actor.Name == "" {
		actor.Name = actor.Username
	}
	if actor.Name == "" {
		actor.Name = "system"
	}
	return actor
}

func cloneActivePackages(active map[string]string) map[string]string {
	next := make(map[string]string, len(active))
	for id, version := range active {
		next[id] = version
	}
	return next
}

func (s *Service) switchHistoryLocked(next map[string]string, actor HistoryActor) []HistoryEvent {
	return switchHistory(s.active.Packages, next, actor)
}

func switchHistory(before, next map[string]string, actor HistoryActor) []HistoryEvent {
	ids := map[string]bool{}
	for id := range before {
		ids[id] = true
	}
	for id := range next {
		ids[id] = true
	}
	keys := make([]string, 0, len(ids))
	for id := range ids {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	events := []HistoryEvent{}
	for _, id := range keys {
		if before[id] != next[id] {
			events = append(events, HistoryEvent{PackageID: id, Version: next[id], PreviousVersion: before[id], Action: "switch", Actor: actor})
		}
	}
	return events
}

// The persisted intent precedes the filesystem/snapshot change. Restart checks
// what was actually published before delivering it to the idempotent SQL writer.
func (s *Service) mutateWithHistoryLocked(events []HistoryEvent, change func() error) error {
	if s.opts.RecordHistory == nil || len(events) == 0 {
		return change()
	}
	if err := s.flushHistoryLocked(); err != nil {
		return err
	}
	for i := range events {
		events[i].ID = newID()
		events[i].CreatedAt = time.Now().UTC()
		events[i].Actor = operationActor([]HistoryActor{events[i].Actor})
	}
	s.state.HistoryPending = append(s.state.HistoryPending, events...)
	if err := s.saveLocked(); err != nil {
		s.state.HistoryPending = s.state.HistoryPending[:len(s.state.HistoryPending)-len(events)]
		return err
	}
	changeErr := change()
	historyErr := s.flushHistoryLocked()
	if changeErr != nil {
		return changeErr
	}
	return historyErr
}

func (s *Service) historyAppliedLocked(event HistoryEvent) bool {
	switch event.Action {
	case "switch":
		return s.active.Packages[event.PackageID] == event.Version
	case "delete":
		_, err := os.Lstat(s.packageDir(Package{ID: event.PackageID, Version: event.Version}))
		return os.IsNotExist(err)
	case "add":
		p, ok := s.packages[event.PackageID][event.Version]
		return ok && s.installed(p)
	}
	return false
}

func (s *Service) flushHistoryLocked() error {
	if s.opts.RecordHistory == nil || len(s.state.HistoryPending) == 0 {
		return nil
	}
	for _, event := range s.state.HistoryPending {
		if !safeID(event.PackageID) || (event.Version != "" && !safeID(event.Version)) {
			return errCode("storage_error", "invalid pending model history identity")
		}
		if !s.historyAppliedLocked(event) {
			continue
		}
		if err := s.opts.RecordHistory(event); err != nil {
			return errCode("history_unavailable", "cannot persist model operation history: "+err.Error())
		}
		if event.Action == "add" {
			p := s.packages[event.PackageID][event.Version]
			if installedAt := s.packageInstalledAt(p); installedAt != nil {
				s.state.HistoryObserved[p.ID+"/"+p.Version] = installedAt.UTC().Format(time.RFC3339Nano)
			}
		}
		if event.Action == "delete" {
			delete(s.state.HistoryObserved, event.PackageID+"/"+event.Version)
		}
	}
	pending := s.state.HistoryPending
	s.state.HistoryPending = nil
	if err := s.saveLocked(); err != nil {
		s.state.HistoryPending = pending
		return err
	}
	return nil
}

func (s *Service) bootstrapHistoryLocked(before map[string]string) error {
	if s.opts.RecordHistory == nil {
		return nil
	}
	events := []HistoryEvent{}
	for _, id := range sortedKeys(s.packages) {
		for _, version := range sortedKeys(s.packages[id]) {
			p := s.packages[id][version]
			installedAt := s.packageInstalledAt(p)
			if installedAt != nil && s.state.HistoryObserved[id+"/"+version] != installedAt.UTC().Format(time.RFC3339Nano) {
				events = append(events, HistoryEvent{PackageID: id, Version: version, Action: "add", Actor: operationActor(nil)})
			}
		}
	}
	events = append(events, switchHistory(before, s.active.Packages, operationActor(nil))...)
	return s.mutateWithHistoryLocked(events, func() error { return nil })
}

// FlushHistory retries pending SQL writes without changing the model selection.
func (s *Service) FlushHistory() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.flushHistoryLocked()
}
