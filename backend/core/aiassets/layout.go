package aiassets

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Release aliases are compiled migration rules, never archive metadata. They
// keep historical releases distinct when two former IDs become one model.
func releaseAliases(providers []Provider) (map[string]ReleaseAlias, error) {
	aliases := map[string]ReleaseAlias{}
	for _, provider := range providers {
		for oldID, alias := range provider.ReleaseAliases {
			if !safeID(oldID) || !safeID(alias.ID) || oldID == alias.ID || alias.VersionPrefix == "" || !safeID(alias.VersionPrefix+"1") {
				return nil, errCode("storage_error", "invalid compiled release alias")
			}
			found := false
			for _, p := range provider.Packages {
				found = found || p.ID == alias.ID
			}
			if _, duplicate := aliases[oldID]; duplicate || !found {
				return nil, errCode("storage_error", "conflicting compiled release alias")
			}
			aliases[oldID] = alias
		}
	}
	return aliases, nil
}

func aliasedRelease(p Package, aliases map[string]ReleaseAlias) Package {
	p = clonePackage(p)
	if alias, ok := aliases[p.ID]; ok {
		p.ID, p.Version = alias.ID, alias.VersionPrefix+p.Version
	}
	for i, id := range p.Dependencies {
		if alias, ok := aliases[id]; ok {
			p.Dependencies[i] = alias.ID
		}
	}
	return p
}

func (s *Service) normalizeReleaseAliases(providers []Provider) (bool, error) {
	aliases, err := releaseAliases(providers)
	if err != nil {
		return false, err
	}
	for _, releases := range []*map[string]map[string]Package{&s.state.Imported, &s.state.Custom} {
		next := map[string]map[string]Package{}
		for id, versions := range *releases {
			for version, original := range versions {
				if original.ID != id || original.Version != version {
					return false, errCode("storage_error", "persisted release identity mismatch")
				}
				p := aliasedRelease(original, aliases)
				if next[p.ID] == nil {
					next[p.ID] = map[string]Package{}
				}
				if prior, exists := next[p.ID][p.Version]; exists && !migrationDescriptorMatches(prior, p) {
					return false, errCode("storage_error", "release aliases collide with existing model versions")
				}
				next[p.ID][p.Version] = p
			}
		}
		*releases = next
	}
	initialization := map[string]map[string]string{}
	for id, versions := range s.state.Initialization {
		for version, status := range versions {
			if !safeID(id) || !safeID(version) || (status != "adopted" && status != "deleted") {
				return false, errCode("storage_error", "invalid initialization release record")
			}
			p := aliasedRelease(Package{ID: id, Version: version}, aliases)
			if initialization[p.ID] == nil {
				initialization[p.ID] = map[string]string{}
			}
			if initialization[p.ID][p.Version] != "deleted" {
				initialization[p.ID][p.Version] = status
			}
		}
	}
	s.state.Initialization = initialization
	changed := false
	active := map[string]string{}
	for id, version := range s.active.Packages {
		if alias, ok := aliases[id]; ok {
			id, version = alias.ID, alias.VersionPrefix+version
			changed = true
		}
		if prior, exists := active[id]; exists && prior != version {
			return false, errCode("storage_error", "multiple historical versions were active for one model")
		}
		active[id] = version
	}
	s.active.Packages = active
	for id, job := range s.state.Jobs {
		if alias, ok := aliases[job.PackageID]; ok {
			job.PackageID = alias.ID
		}
		for i, packageID := range job.Activated {
			if alias, ok := aliases[packageID]; ok {
				job.Activated[i] = alias.ID
			}
		}
		for i, pending := range job.Pending {
			if alias, ok := aliases[pending.PackageID]; ok {
				job.Pending[i].PackageID = alias.ID
				job.Pending[i].Version = alias.VersionPrefix + pending.Version
			}
		}
		s.state.Jobs[id] = job
	}
	return changed, nil
}

func migrationDescriptorMatches(a, b Package) bool {
	if a.Provider != b.Provider || a.Kind != b.Kind || a.Entry != b.Entry || len(a.Dependencies) != len(b.Dependencies) || !sameReleaseContent(a, b) {
		return false
	}
	deps := map[string]bool{}
	for _, id := range a.Dependencies {
		deps[id] = true
	}
	for _, id := range b.Dependencies {
		if !deps[id] {
			return false
		}
	}
	return true
}

func (s *Service) initializeManagedLayout(providers []Provider, aliasesChanged bool) error {
	if s.state.ManagedLayout < 0 || s.state.ManagedLayout > 2 {
		return errCode("storage_error", "unsupported managed model layout")
	}
	aliases, err := releaseAliases(providers)
	if err != nil {
		return err
	}
	// Preserve the previous layout as a recovery copy. Only checked receipts and
	// SHA-256 verified files are copied into the current immutable version store.
	for _, id := range sortedKeys(s.packages) {
		for _, version := range sortedKeys(s.packages[id]) {
			p := s.packages[id][version]
			if err := s.migrateReleaseDirectory(p, aliases); err != nil {
				return err
			}
		}
	}
	if aliasesChanged {
		s.active.Generation++
		if err := atomicJSON(filepath.Join(s.dir, "active.json"), s.active); err != nil {
			return errCode("storage_error", err.Error())
		}
	}
	// Each compiled release is adopted independently. An empty first start must
	// not prevent a later initialization script from supplying a new flat model.
	// Successful adoption and explicit deletion both prevent later resurrection.
	candidates := []Package{}
	for _, provider := range providers {
		for _, compiled := range provider.Packages {
			p := s.packages[compiled.ID][compiled.Version]
			if s.state.Initialization[p.ID][p.Version] != "" {
				continue
			}
			if !s.installed(p) && fingerprint(p) == fingerprint(compiled) {
				if _, err := s.publishCheckedCopy(s.root, p, true); err != nil {
					return err
				}
			}
			if s.installed(p) {
				s.recordInitialization(p, "adopted")
				candidates = append(candidates, p)
			}
		}
	}
	// Provider catalog order preserves its default model selection. Dependencies
	// are adopted first; an already selected kind is never replaced at startup.
	for progress := true; progress; {
		progress = false
		for _, p := range candidates {
			if !s.installed(p) || s.hasActiveKind(p.Kind) || !s.managedInitializationDependenciesReady(p) {
				continue
			}
			next := make(map[string]string, len(s.active.Packages)+1)
			for id, version := range s.active.Packages {
				next[id] = version
			}
			next[p.ID] = p.Version
			if s.validateActive(next) != nil {
				continue
			}
			err := s.mutateGuarded([]string{p.Kind}, func() error {
				snapshot := activeSnapshot{SchemaVersion: 1, Generation: s.active.Generation + 1, Packages: next}
				if err := atomicJSON(filepath.Join(s.dir, "active.json"), snapshot); err != nil {
					return errCode("storage_error", err.Error())
				}
				s.active = snapshot
				return nil
			})
			if err != nil {
				if failureCode(err) == "in_use" {
					continue
				}
				return err
			}
			progress = true
		}
	}
	s.state.ManagedLayout = 2
	return nil
}

func (s *Service) recordInitialization(p Package, status string) {
	if s.state.Initialization[p.ID] == nil {
		s.state.Initialization[p.ID] = map[string]string{}
	}
	s.state.Initialization[p.ID][p.Version] = status
}

func (s *Service) managedInitializationDependenciesReady(p Package) bool {
	for _, id := range p.Dependencies {
		if _, active := s.active.Packages[id]; !active && s.alternativeActiveLocked(id) == "" {
			return false
		}
	}
	return true
}

func (s *Service) hasActiveKind(kind string) bool {
	for id, version := range s.active.Packages {
		if p, ok := s.packages[id][version]; ok && p.Kind == kind {
			return true
		}
	}
	return false
}

func (s *Service) migrateReleaseDirectory(p Package, aliases map[string]ReleaseAlias) error {
	target := s.packageDir(p)
	if _, err := os.Lstat(target); err == nil {
		if err := checkDirectory(target); err != nil {
			return errCode("storage_error", err.Error())
		}
		if validReceipt(target, p) {
			return nil
		}
		// A current-layout receipt can still contain the previous dependency IDs.
		if ok, err := s.trustedMigrationSource(target, p, aliases); err != nil {
			return err
		} else if ok {
			r, err := makeReceipt(target, p)
			if err != nil {
				return err
			}
			if installedAt := receiptInstalledAt(target); installedAt != nil {
				r.InstalledAt = *installedAt
			}
			return atomicJSON(filepath.Join(target, "receipt.json"), r)
		}
		return nil // An incomplete immutable version must be explicitly repaired.
	} else if !os.IsNotExist(err) {
		return err
	}
	if s.state.ManagedLayout == 2 || s.state.Initialization[p.ID][p.Version] == "deleted" {
		return nil // Retained recovery copies must never resurrect deleted versions.
	}
	sources, err := releaseDirectories(s.root, s.dir, p, aliases)
	if err != nil {
		return err
	}
	for _, source := range migrationSources(s.root, sources, s.state.ManagedLayout) {
		ok, err := s.trustedMigrationSource(source, p, aliases)
		if err != nil {
			return err
		}
		if ok {
			_, err = s.publishCheckedCopy(source, p, false)
			return err
		}
	}
	return nil
}

func releaseDirectories(root, metadata string, p Package, aliases map[string]ReleaseAlias) ([]string, error) {
	if !safeID(p.ID) || !safeID(p.Version) {
		return nil, errCode("storage_error", "invalid model directory identity")
	}
	sources := []string{filepath.Join(root, p.ID, p.Version), filepath.Join(root, "versions", p.ID, p.Version), filepath.Join(metadata, "versions", p.ID, p.Version)}
	for oldID, alias := range aliases {
		if alias.ID == p.ID && strings.HasPrefix(p.Version, alias.VersionPrefix) {
			oldVersion := strings.TrimPrefix(p.Version, alias.VersionPrefix)
			if !safeID(oldVersion) {
				return nil, errCode("storage_error", "invalid historical model version")
			}
			sources = append(sources, filepath.Join(root, oldID, oldVersion), filepath.Join(root, "versions", oldID, oldVersion), filepath.Join(metadata, "versions", oldID, oldVersion))
		}
	}
	return sources, nil
}

// Layout 1 has already retired .aiassets/versions. Only its live root/versions
// store may migrate into layout 2. Layout 2 never reopens any recovery directory.
func migrationSources(root string, sources []string, layout int) []string {
	if layout < 0 || layout >= 2 {
		return nil
	}
	if layout == 0 {
		return sources[1:]
	}
	allowed := []string{}
	previous := filepath.Join(root, "versions")
	for _, source := range sources[1:] {
		relative, err := filepath.Rel(previous, source)
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			allowed = append(allowed, source)
		}
	}
	return allowed
}

func (s *Service) trustedMigrationSource(source string, p Package, aliases map[string]ReleaseAlias) (bool, error) {
	if _, err := os.Lstat(source); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if err := checkDirectory(source); err != nil {
		return false, errCode("storage_error", err.Error())
	}
	name, err := fileAt(source, "receipt.json")
	if err != nil {
		return false, err
	}
	var r receipt
	if loadJSON(name, &r) != nil || !migrationDescriptorMatches(aliasedRelease(r.Package, aliases), p) || !validReceipt(source, r.Package) {
		return false, nil
	}
	if err := validateFiles(source, r.Package); err != nil {
		return false, nil
	}
	return true, nil
}

// publishCheckedCopy never modifies source resources or an existing version.
// Missing inline resources may be supplied by the trusted compiled descriptor;
// existing files with different content are never adopted as that descriptor.
func (s *Service) publishCheckedCopy(source string, p Package, allowInline bool) (bool, error) {
	if _, err := os.Lstat(s.packageDir(p)); err == nil {
		return s.installed(p), nil
	} else if !os.IsNotExist(err) {
		return false, err
	}
	var bytes int64
	inline := map[string]bool{}
	for _, f := range p.Files {
		name, err := fileAt(source, f.Path)
		if err != nil {
			return false, nil // Flat resources containing links remain unmanaged.
		}
		if _, err := os.Lstat(name); os.IsNotExist(err) && allowInline && f.Inline != "" {
			inline[f.Path] = true
		} else if checkFile(source, f) != nil {
			return false, nil
		}
		bytes += f.Bytes
	}
	if err := s.checkSpace(bytes); err != nil {
		return false, err
	}
	stage, err := os.MkdirTemp(filepath.Join(s.dir, "staging"), "layout-")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(stage)
	for _, f := range p.Files {
		name := filepath.Join(stage, filepath.FromSlash(f.Path))
		if err := ensureDirectory(filepath.Dir(name)); err != nil {
			return false, err
		}
		out, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return false, err
		}
		if inline[f.Path] {
			_, err = io.Copy(out, strings.NewReader(f.Inline))
		} else {
			var in *os.File
			var sourceName string
			sourceName, err = fileAt(source, f.Path)
			if err == nil {
				in, err = os.Open(sourceName)
			}
			if err == nil {
				_, err = io.Copy(out, io.LimitReader(in, f.Bytes+1))
				err = errors.Join(err, in.Close())
			}
		}
		if err == nil {
			err = out.Sync()
		}
		err = errors.Join(err, out.Close())
		if err != nil {
			return false, err
		}
	}
	if err := validateFiles(stage, p); err != nil {
		return false, err
	}
	r, err := makeReceipt(stage, p)
	if err != nil {
		return false, err
	}
	if !allowInline {
		if installedAt := receiptInstalledAt(source); installedAt != nil {
			r.InstalledAt = *installedAt
		}
	}
	if err = atomicJSON(filepath.Join(stage, "receipt.json"), r); err != nil {
		return false, err
	}
	if err = ensureDirectory(filepath.Dir(s.packageDir(p))); err != nil {
		return false, err
	}
	if err = os.Rename(stage, s.packageDir(p)); err != nil {
		return false, errCode("storage_error", err.Error())
	}
	return true, nil
}
