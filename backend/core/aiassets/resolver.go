package aiassets

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
)

// ModelRootManaged reports whether the version store owns this model root.
// Providers use it to keep retained initialization files from becoming an
// implicit runtime selection after a managed release has been removed.
// Existing unreadable state fails closed instead of permitting legacy fallback.
func ModelRootManaged(root string) bool {
	abs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	name, err := fileAt(filepath.Join(abs, ".aiassets"), "state.json")
	if err != nil {
		return !os.IsNotExist(err)
	}
	f, err := os.Open(name)
	if err != nil {
		return !os.IsNotExist(err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 32<<20 {
		return true
	}
	var marker struct {
		ManagedLayout int `json:"managedLayout"`
	}
	d := json.NewDecoder(io.LimitReader(f, 32<<20))
	if d.Decode(&marker) != nil || d.Decode(new(any)) != io.EOF {
		return true
	}
	return marker.ManagedLayout != 0
}

// ResolveRoot reads an already published snapshot without opening a service or
// acquiring the installer lock. Only registered release descriptors are trusted.
func ResolveRoot(root, kind string) (ResolvedPackage, bool) {
	return resolveRootWithProviders(root, kind, Providers())
}

func resolveRootWithProviders(root, kind string, providers []Provider) (ResolvedPackage, bool) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return ResolvedPackage{}, false
	}
	dir := filepath.Join(abs, ".aiassets")
	var active activeSnapshot
	if loadJSON(filepath.Join(dir, "active.json"), &active) != nil || active.SchemaVersion != 1 {
		return ResolvedPackage{}, false
	}
	aliases, err := releaseAliases(providers)
	if err != nil {
		return ResolvedPackage{}, false
	}
	selected := map[string]string{}
	for id, version := range active.Packages {
		if alias, ok := aliases[id]; ok {
			id, version = alias.ID, alias.VersionPrefix+version
		}
		if prior, duplicate := selected[id]; duplicate && prior != version {
			return ResolvedPackage{}, false
		}
		selected[id] = version
	}
	var state persistedState
	stateErr := loadJSON(filepath.Join(dir, "state.json"), &state)
	layout := 2 // Unreadable state fails closed instead of exposing recovery copies.
	if os.IsNotExist(stateErr) {
		layout = 0
	} else if stateErr == nil {
		layout = state.ManagedLayout
	}
	for _, provider := range providers {
		for _, p := range provider.Packages {
			if p.Kind != kind || selected[p.ID] != p.Version {
				continue
			}
			if resolved, ok := resolveStoredRelease(abs, dir, p, aliases, layout); ok {
				return resolved, true
			}
		}
	}
	if stateErr == nil {
		for _, releases := range []struct {
			packages map[string]map[string]Package
			validate func([]Provider, Package) error
		}{{state.Custom, validateCustom}, {state.Imported, validateImported}} {
			for _, versions := range releases.packages {
				for _, p := range versions {
					p = aliasedRelease(p, aliases)
					if p.Kind != kind || selected[p.ID] != p.Version || releases.validate(providers, p) != nil {
						continue
					}
					if resolved, ok := resolveStoredRelease(abs, dir, p, aliases, layout); ok {
						return resolved, true
					}
				}
			}
		}
	}
	return ResolvedPackage{}, false
}

// Before the installer starts, providers may read the previous trusted layout.
// This path does not migrate files or change the active selection.
func resolveStoredRelease(root, metadata string, p Package, aliases map[string]ReleaseAlias, layout int) (ResolvedPackage, bool) {
	sources, err := releaseDirectories(root, metadata, p, aliases)
	if err != nil {
		return ResolvedPackage{}, false
	}
	sources = append(sources[:1:1], migrationSources(root, sources, layout)...)
	for _, source := range sources {
		if validReceipt(source, p) {
			return resolvedRelease(p, source), true
		}
		name, err := fileAt(source, "receipt.json")
		if err != nil {
			continue
		}
		var r receipt
		if loadJSON(name, &r) == nil && migrationDescriptorMatches(aliasedRelease(r.Package, aliases), p) && validReceipt(source, r.Package) {
			return resolvedRelease(p, source), true
		}
	}
	return ResolvedPackage{}, false
}

func resolvedRelease(p Package, dir string) ResolvedPackage {
	return ResolvedPackage{Package: clonePackage(p), Directory: dir, EntryPath: filepath.Join(dir, filepath.FromSlash(p.Entry)), Fingerprint: fingerprint(p)}
}
