package aiassets

import "strings"

func validateCustom(providers []Provider, p Package) error {
	if p.Source != "custom" || !strings.HasPrefix(p.ID, "custom-") || !p.OfflineOnly {
		return errCode("unsupported", "custom packages require custom- IDs and offline provenance")
	}
	for _, f := range p.Files {
		if f.URL != "" || f.Inline != "" {
			return errCode("unsupported", "custom packages cannot supply download URLs or inline resources")
		}
	}
	if err := validateProvider(Provider{ID: p.Provider, Packages: []Package{p}}); err != nil {
		return err
	}
	for _, provider := range providers {
		if provider.ID != p.Provider || provider.Validate == nil {
			continue
		}
		for _, template := range provider.Packages {
			if template.Kind != p.Kind || template.Contract != p.Contract || template.Entry != p.Entry {
				continue
			}
			if len(template.Files) != len(p.Files) || len(template.Dependencies) != len(p.Dependencies) {
				continue
			}
			paths := map[string]bool{}
			for _, f := range template.Files {
				paths[f.Path] = true
			}
			match := true
			for _, f := range p.Files {
				if !paths[f.Path] {
					match = false
				}
			}
			deps := map[string]bool{}
			for _, d := range template.Dependencies {
				deps[d] = true
			}
			for _, d := range p.Dependencies {
				if !deps[d] {
					match = false
				}
			}
			if match {
				return nil
			}
		}
	}
	return errCode("unsupported", "custom package does not match a compiled model contract and required resources")
}
func (s *Service) refreshPackageSnapshotLocked() {
	s.packageSnapshot.Store(cloneReleases(s.packages))
}
func cloneReleases(packages map[string]map[string]Package) map[string]map[string]Package {
	copy := map[string]map[string]Package{}
	for id, versions := range packages {
		copy[id] = map[string]Package{}
		for version, p := range versions {
			copy[id][version] = clonePackage(p)
		}
	}
	return copy
}
func (s *Service) releaseSnapshot() map[string]map[string]Package {
	if v := s.packageSnapshot.Load(); v != nil {
		return v.(map[string]map[string]Package)
	}
	return s.packages
}
func (s *Service) providerList() []Provider {
	result := make([]Provider, 0, len(s.providers))
	for _, p := range s.providers {
		result = append(result, p)
	}
	return result
}
func (s *Service) packageLimit(p Package) error {
	var total int64
	for _, f := range p.Files {
		if f.Bytes > s.opts.MaxPackageBytes-total {
			return errCode("archive_invalid", "package exceeds configured resource limit")
		}
		total += f.Bytes
	}
	return nil
}
func compatibleRelease(a, b Package) bool {
	return a.Kind == b.Kind && a.Contract == b.Contract && a.Provider == b.Provider
}
func (s *Service) alternativeActiveLocked(id string) string {
	for _, expected := range s.packages[id] {
		for activeID, version := range s.active.Packages {
			p, ok := s.packages[activeID][version]
			if ok && activeID != id && compatibleRelease(expected, p) && s.installed(p) {
				return activeID
			}
		}
	}
	return ""
}
func (s *Service) dependencySatisfiedLocked(next map[string]string, id string) bool {
	if _, ok := next[id]; ok {
		return true
	}
	for _, expected := range s.packages[id] {
		if legacy := s.providers[expected.Provider].Legacy; legacy != nil && legacy(s.root, expected.Kind) {
			return true
		}
		for activeID, version := range next {
			if p, ok := s.packages[activeID][version]; ok && compatibleRelease(expected, p) {
				return true
			}
		}
	}
	return false
}
