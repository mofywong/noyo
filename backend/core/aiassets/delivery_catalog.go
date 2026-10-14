package aiassets

import "sort"

// DeliveryCatalog is a supplier input, never an import manifest. It contains
// mappings and pinned descriptors but no local paths, usage or camera references.
type DeliveryCatalog struct {
	SchemaVersion int          `json:"schemaVersion"`
	Capabilities  []Capability `json:"capabilities"`
	Packages      []Package    `json:"packages"`
}

func (s *Service) DeliveryCatalog() DeliveryCatalog {
	s.mu.Lock()
	defer s.mu.Unlock()
	catalog := DeliveryCatalog{SchemaVersion: 1, Capabilities: []Capability{}, Packages: []Package{}}
	for _, c := range s.capabilities {
		c.Packages = append([]string(nil), c.Packages...)
		c.Dependencies = append([]string(nil), c.Dependencies...)
		c.OptionalPackages = append([]string(nil), c.OptionalPackages...)
		catalog.Capabilities = append(catalog.Capabilities, c)
	}
	for _, versions := range s.packages {
		for _, p := range versions {
			catalog.Packages = append(catalog.Packages, clonePackage(p))
		}
	}
	sort.Slice(catalog.Capabilities, func(i, j int) bool { return catalog.Capabilities[i].ID < catalog.Capabilities[j].ID })
	sort.Slice(catalog.Packages, func(i, j int) bool {
		a, b := catalog.Packages[i], catalog.Packages[j]
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		return newer(b.Version, a.Version)
	})
	return catalog
}
