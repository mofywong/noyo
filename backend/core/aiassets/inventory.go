package aiassets

import "time"

// ModelInventory is a local inventory, not an import manifest or release catalog.
// Version is empty for pre-existing unmanaged resources because their directory
// presence cannot establish the version of a compiled release template.
type ModelInventory struct {
	SchemaVersion int           `json:"schemaVersion"`
	ExportedAt    time.Time     `json:"exportedAt"`
	Generation    uint64        `json:"generation"`
	Models        []ModelRecord `json:"models"`
}

type ModelRecord struct {
	ID          string     `json:"id"`
	Provider    string     `json:"provider"`
	Kind        string     `json:"kind"`
	Title       Text       `json:"title"`
	Description Text       `json:"description"`
	Version     string     `json:"version"`
	State       string     `json:"state"`
	Installed   bool       `json:"installed"`
	InstalledAt *time.Time `json:"installedAt,omitempty"`
	Active      bool       `json:"active"`
	Source      string     `json:"source"`
	Contract    string     `json:"contract"`
}

func (s *Service) Inventory() ModelInventory {
	catalog := s.Catalog()
	result := ModelInventory{SchemaVersion: 1, ExportedAt: time.Now().UTC(), Generation: catalog.Generation, Models: []ModelRecord{}}
	legacySeen := map[string]bool{}
	for _, status := range catalog.Packages {
		p := status.Package
		model := ModelRecord{ID: p.ID, Provider: p.Provider, Kind: p.Kind, Title: p.Title, Description: p.Description,
			Version: p.Version, State: "not_installed", Installed: status.Installed, InstalledAt: status.InstalledAt, Active: status.Active, Source: p.Source, Contract: p.Contract}
		if status.State == "legacy" {
			if legacySeen[p.ID] {
				continue
			}
			legacySeen[p.ID] = true
			model.Version, model.Source = "", "unmanaged"
			model.Installed, model.Active = true, true
		}
		if model.Installed {
			model.State = "installed"
		}
		result.Models = append(result.Models, model)
	}
	return result
}
