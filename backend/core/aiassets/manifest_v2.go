package aiassets

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"strings"
)

// These are the wire fields, not the application's richer catalog descriptors.
type compactManifest struct {
	SchemaVersion int              `json:"schemaVersion"`
	Packages      []compactPackage `json:"packages"`
}
type compactPackage struct {
	ID          string          `json:"id"`
	Version     string          `json:"version"`
	Contract    string          `json:"contract,omitempty"`
	Description json.RawMessage `json:"description,omitempty"`
	Files       []compactFile   `json:"files"`
}
type compactFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// importedManifest contains trusted descriptors resolved from the v2 wire
// format. These descriptors are internal and must never be published in a ZIP.
type importedManifest struct {
	Packages []Package
}

func strictManifest(data []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return errCode("archive_invalid", "invalid offline release manifest")
	}
	if d.Decode(new(any)) != io.EOF {
		return errCode("archive_invalid", "trailing import manifest data")
	}
	return nil
}

// Called with the mutation lock held. Only compiled stable package IDs may
// publish compatible releases; an archive cannot invent an algorithm mapping.
func (s *Service) decodeManifest(data []byte, entries map[string]*zip.File) (importedManifest, error) {
	var header struct {
		SchemaVersion int `json:"schemaVersion"`
	}
	if json.Unmarshal(data, &header) != nil {
		return importedManifest{}, errCode("archive_invalid", "invalid offline release manifest")
	}
	var wire compactManifest
	if header.SchemaVersion != 2 {
		return importedManifest{}, errCode("unsupported", "offline model manifests require schemaVersion 2")
	}
	if err := strictManifest(data, &wire); err != nil {
		return importedManifest{}, err
	}
	m := importedManifest{}
	for _, release := range wire.Packages {
		var p Package
		found := false
		compatibleContract := false
		for _, provider := range s.providers {
			for _, template := range provider.Packages {
				if template.ID == release.ID && (release.Contract == "" || template.Contract == release.Contract) && provider.Validate != nil {
					compatibleContract = true
					if !matchesManifestPaths(template, release.Files) {
						continue
					}
					if found && (p.Provider != template.Provider || p.Kind != template.Kind || p.Contract != template.Contract || p.Entry != template.Entry) {
						return m, errCode("unsupported", "ambiguous model contract; specify an explicit contract: "+release.ID)
					}
					p = clonePackage(template)
					found = true
				}
			}
		}
		if !found {
			if compatibleContract {
				return m, errCode("invalid_request", "offline resources do not match a complete compiled layout: "+release.ID)
			}
			return m, errCode("unsupported", "model contract requires an application update: "+release.ID)
		}
		if len(release.Description) != 0 {
			description, err := manifestDescription(release.Description)
			if err != nil {
				return m, err
			}
			p.Description = description
		}
		p.Version, p.Source, p.OfflineOnly = release.Version, "imported", true
		p.License = "" // The compiled template cannot attest to an imported weight's license.
		p.Files = nil
		for _, f := range release.Files {
			entry := entries[strings.ToLower(p.ID+"/"+p.Version+"/"+f.Path)]
			if entry == nil || entry.UncompressedSize64 > uint64(s.opts.MaxPackageBytes) {
				return m, errCode("archive_invalid", "missing or oversized model resource: "+f.Path)
			}
			p.Files = append(p.Files, File{Path: f.Path, SHA256: strings.ToLower(f.SHA256), Bytes: int64(entry.UncompressedSize64)})
		}
		if err := validateImported(s.providerList(), p); err != nil {
			return m, err
		}
		if existing, ok := s.packages[p.ID][p.Version]; ok {
			if !sameReleaseContent(existing, p) {
				return m, errCode("conflict", "immutable model release already exists: "+p.ID)
			}
			// Preserve existing receipts and trusted built-in provenance.
			p = clonePackage(existing)
		}
		m.Packages = append(m.Packages, p)
	}
	return m, nil
}

// A model family can offer several compiled resource layouts under one
// contract. The ZIP must identify one complete layout by its resource paths.
func matchesManifestPaths(template Package, files []compactFile) bool {
	if len(template.Files) != len(files) {
		return false
	}
	paths := make(map[string]bool, len(template.Files))
	for _, file := range template.Files {
		paths[file.Path] = true
	}
	for _, file := range files {
		if !paths[file.Path] {
			return false
		}
		delete(paths, file.Path)
	}
	return len(paths) == 0
}

func sameReleaseContent(a, b Package) bool {
	if a.ID != b.ID || a.Version != b.Version || a.Contract != b.Contract || len(a.Files) != len(b.Files) {
		return false
	}
	files := map[string]File{}
	for _, f := range a.Files {
		files[f.Path] = f
	}
	for _, f := range b.Files {
		old, ok := files[f.Path]
		if !ok || old.Bytes != f.Bytes || !strings.EqualFold(old.SHA256, f.SHA256) {
			return false
		}
	}
	return true
}

func validateImported(providers []Provider, p Package) error {
	if p.Source != "imported" || !p.OfflineOnly {
		return errCode("unsupported", "invalid imported provenance")
	}
	if err := validateProvider(Provider{ID: p.Provider, Packages: []Package{p}}); err != nil {
		return err
	}
	for _, f := range p.Files {
		if f.URL != "" || f.Inline != "" {
			return errCode("unsupported", "imported releases cannot supply download sources")
		}
	}
	for _, provider := range providers {
		if provider.ID != p.Provider || provider.Validate == nil {
			continue
		}
		for _, t := range provider.Packages {
			if t.ID != p.ID || t.Kind != p.Kind || t.Contract != p.Contract || t.Entry != p.Entry || len(t.Files) != len(p.Files) || len(t.Dependencies) != len(p.Dependencies) {
				continue
			}
			paths, deps := map[string]bool{}, map[string]bool{}
			for _, f := range t.Files {
				paths[f.Path] = true
			}
			for _, d := range t.Dependencies {
				deps[d] = true
			}
			match := true
			for _, f := range p.Files {
				match = match && paths[f.Path]
			}
			for _, d := range p.Dependencies {
				match = match && deps[d]
			}
			if match {
				return nil
			}
		}
	}
	return errCode("unsupported", "imported model does not match compiled contract resources")
}

// Descriptions are user metadata. A single string is kept in both locales; an
// object may supply independent translations, with a missing locale falling back.
func manifestDescription(data []byte) (Text, error) {
	var value string
	if json.Unmarshal(data, &value) == nil && !bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return Text{ZH: value, EN: value}, nil
	}
	var description Text
	if len(bytes.TrimSpace(data)) == 0 || bytes.TrimSpace(data)[0] != '{' {
		return Text{}, errCode("archive_invalid", "model description requires a string or zh/en object")
	}
	if err := strictManifest(data, &description); err != nil {
		return Text{}, err
	}
	if description.ZH == "" {
		description.ZH = description.EN
	}
	if description.EN == "" {
		description.EN = description.ZH
	}
	return description, nil
}
