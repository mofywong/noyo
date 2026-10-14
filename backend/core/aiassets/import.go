package aiassets

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Import stages an offline ZIP and returns a reviewable plan. It does not enable
// resources; Install applies the same integrity and provider validation pipeline.
func (s *Service) Import(ctx context.Context, reader io.Reader) (Plan, error) {
	return s.importWithMode(ctx, reader, false)
}

// ImportModel parses one self-described model release for the model-library
// update flow. Validation and preview do not change the active runtime selection.
func (s *Service) ImportModel(ctx context.Context, reader io.Reader) (Plan, error) {
	return s.importWithMode(ctx, reader, true)
}

func (s *Service) importWithMode(ctx context.Context, reader io.Reader, single bool) (Plan, error) {
	plan, err := s.importArchive(ctx, reader, single)
	if isNoSpace(err) {
		return Plan{}, errCode("disk_insufficient", "offline import exhausted disk capacity")
	}
	var pathError *os.PathError
	if errors.As(err, &pathError) {
		return Plan{}, errCode("storage_error", "offline import cannot write model resources")
	}
	return plan, err
}

func (s *Service) importArchive(ctx context.Context, reader io.Reader, single bool) (Plan, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return Plan{}, errCode("conflict", "installer is closed")
	}
	s.mu.Unlock()
	if err := s.checkSpace(0); err != nil {
		return Plan{}, err
	}
	archive, err := os.CreateTemp(filepath.Join(s.dir, "imports"), ".upload-")
	if err != nil {
		return Plan{}, errCode("storage_error", err.Error())
	}
	defer os.Remove(archive.Name())
	n, err := io.Copy(archive, &contextReader{ctx: ctx, r: io.LimitReader(reader, s.opts.MaxArchiveBytes+1)})
	closeErr := archive.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		if isNoSpace(err) {
			return Plan{}, errCode("disk_insufficient", "offline upload exhausted disk capacity")
		}
		return Plan{}, err
	}
	if n > s.opts.MaxArchiveBytes {
		return Plan{}, errCode("archive_invalid", "offline archive exceeds upload limit")
	}
	z, err := zip.OpenReader(archive.Name())
	if err != nil {
		return Plan{}, errCode("archive_invalid", "offline import requires a ZIP archive")
	}
	defer z.Close()
	if len(z.File) > 4096 {
		return Plan{}, errCode("archive_invalid", "offline archive has too many entries")
	}
	entries := map[string]*zip.File{}
	var manifestFile *zip.File
	var expanded uint64
	for _, f := range z.File {
		if err = ctx.Err(); err != nil {
			return Plan{}, err
		}
		name := f.Name
		isDir := strings.HasSuffix(name, "/")
		if isDir {
			name = strings.TrimSuffix(name, "/")
		}
		if !safeRelative(name) {
			return Plan{}, errCode("archive_invalid", "unsafe archive path")
		}
		key := strings.ToLower(name)
		if entries[key] != nil {
			return Plan{}, errCode("archive_invalid", "duplicate archive entry")
		}
		entries[key] = f
		if f.Mode()&os.ModeSymlink != 0 || (!isDir && !f.Mode().IsRegular()) {
			return Plan{}, errCode("archive_invalid", "archive links and special files are forbidden")
		}
		if f.UncompressedSize64 > uint64(s.opts.MaxExpandedBytes)-expanded {
			return Plan{}, errCode("archive_invalid", "expanded archive exceeds limit")
		}
		expanded += f.UncompressedSize64
		if name == "manifest.json" && !isDir {
			manifestFile = f
		}
	}
	if err = s.checkSpace(int64(expanded)); err != nil {
		return Plan{}, err
	}
	if manifestFile == nil {
		return Plan{}, errCode("manifest_root_missing", "manifest.json must be at the ZIP root without a wrapper directory")
	}
	if manifestFile.UncompressedSize64 > 1<<20 {
		return Plan{}, errCode("archive_invalid", "oversized manifest.json")
	}
	in, err := manifestFile.Open()
	if err != nil {
		return Plan{}, errCode("archive_invalid", err.Error())
	}
	data, err := io.ReadAll(io.LimitReader(in, 1<<20))
	_ = in.Close()
	if err != nil {
		return Plan{}, errCode("archive_invalid", "invalid offline release manifest")
	}
	s.mu.Lock()
	manifest, err := s.decodeManifest(data, entries)
	if err != nil {
		s.mu.Unlock()
		return Plan{}, err
	}
	if len(manifest.Packages) == 0 {
		s.mu.Unlock()
		return Plan{}, errCode("archive_invalid", "unsupported offline release manifest")
	}
	if single && len(manifest.Packages) != 1 {
		s.mu.Unlock()
		return Plan{}, errCode("archive_invalid", "a model update archive must contain exactly one model")
	}
	versions := map[string]string{}
	packageIDs := []string{}
	provisional := cloneReleases(s.packages)
	for _, p := range manifest.Packages {
		if err = s.packageLimit(p); err != nil {
			s.mu.Unlock()
			return Plan{}, err
		}
		if p.Source == "imported" {
			if provisional[p.ID] == nil {
				provisional[p.ID] = map[string]Package{}
			}
			provisional[p.ID][p.Version] = clonePackage(p)
		}
		if _, duplicate := versions[p.ID]; duplicate {
			s.mu.Unlock()
			return Plan{}, errCode("archive_invalid", "duplicate package in manifest")
		}
		versions[p.ID] = p.Version
		packageIDs = append(packageIDs, p.ID)
	}
	prior := s.packages
	s.packages = provisional
	plan, err := s.planLocked(PlanRequest{Packages: packageIDs, Versions: versions})
	if err == nil {
		// Mark the plan as an offline delivery before deriving usage. An import
		// takes over its resource kind, so its effect is activate_if_missing and
		// never candidate; planLocked derives usage before that is known.
		plan.Offline = true
		plan.Usage = s.planUsageLocked(plan)
	}
	s.packages = prior
	s.mu.Unlock()
	if err != nil {
		return Plan{}, err
	}
	for _, p := range plan.Packages {
		if _, included := versions[p.ID]; !included && !s.installed(p) {
			return Plan{}, errCode("missing_dependency", "offline archive omits required package: "+p.ID)
		}
	}
	stage := filepath.Join(s.dir, "imports", plan.ID)
	if err = ensureDirectory(stage); err != nil {
		return Plan{}, errCode("storage_error", err.Error())
	}
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(stage)
		}
	}()
	expected := map[string]bool{"manifest.json": true}
	directories := map[string]bool{}
	for _, p := range manifest.Packages {
		for _, f := range p.Files {
			name := p.ID + "/" + p.Version + "/" + f.Path
			expected[name] = true
			parts := strings.Split(name, "/")
			for i := 1; i < len(parts); i++ {
				directories[strings.Join(parts[:i], "/")] = true
			}
		}
	}
	for _, f := range z.File {
		name := strings.TrimSuffix(f.Name, "/")
		if strings.HasSuffix(f.Name, "/") {
			if !directories[name] {
				return Plan{}, errCode("archive_invalid", "undeclared archive directory")
			}
			continue
		}
		if !expected[name] {
			return Plan{}, errCode("archive_invalid", "undeclared archive resource: "+name)
		}
	}
	for _, p := range manifest.Packages {
		dir := filepath.Join(stage, p.ID, p.Version)
		if err = ensureDirectory(dir); err != nil {
			return Plan{}, err
		}
		for _, f := range p.Files {
			if err = ctx.Err(); err != nil {
				return Plan{}, err
			}
			name, err := fileAt(dir, f.Path)
			if err != nil {
				return Plan{}, err
			}
			if err = ensureDirectory(filepath.Dir(name)); err != nil {
				return Plan{}, err
			}
			entry := entries[strings.ToLower(p.ID+"/"+p.Version+"/"+f.Path)]
			if entry == nil {
				return Plan{}, errCode("archive_invalid", "missing offline resource: "+f.Path)
			}
			if entry.Name != p.ID+"/"+p.Version+"/"+f.Path || entry.UncompressedSize64 != uint64(f.Bytes) {
				return Plan{}, errCode("integrity", "offline resource path or size mismatch")
			}
			in, err := entry.Open()
			if err != nil {
				return Plan{}, errCode("archive_invalid", err.Error())
			}
			out, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err != nil {
				_ = in.Close()
				return Plan{}, err
			}
			_, err = io.Copy(out, &contextReader{ctx: ctx, r: io.LimitReader(in, f.Bytes+1)})
			_ = in.Close()
			if err == nil {
				err = out.Sync()
			}
			closeErr := out.Close()
			if err == nil {
				err = closeErr
			}
			if err != nil {
				if isNoSpace(err) {
					return Plan{}, err
				}
				return Plan{}, errCode("archive_invalid", err.Error())
			}
			if err = checkFile(dir, f); err != nil {
				return Plan{}, err
			}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Plan{}, errCode("conflict", "installer is closed")
	}
	if err = ctx.Err(); err != nil {
		return Plan{}, err
	}
	s.state.Plans[plan.ID] = plan
	s.state.Imports[plan.ID] = plan.ID
	priorReleases := s.packages
	priorImported := cloneReleases(s.state.Imported)
	// Another upload may have registered this identity while extraction ran.
	for _, p := range manifest.Packages {
		if existing, ok := s.packages[p.ID][p.Version]; ok && fingerprint(existing) != fingerprint(p) {
			delete(s.state.Plans, plan.ID)
			delete(s.state.Imports, plan.ID)
			return Plan{}, errCode("conflict", "immutable model release already exists")
		}
	}
	s.packages = cloneReleases(s.packages)
	for _, p := range manifest.Packages {
		if p.Source != "imported" {
			continue
		}
		if s.packages[p.ID] == nil {
			s.packages[p.ID] = map[string]Package{}
		}
		if s.state.Imported[p.ID] == nil {
			s.state.Imported[p.ID] = map[string]Package{}
		}
		s.packages[p.ID][p.Version] = clonePackage(p)
		s.state.Imported[p.ID][p.Version] = clonePackage(p)
	}
	s.refreshPackageSnapshotLocked()
	if err = s.saveLocked(); err != nil {
		s.packages = priorReleases
		s.state.Imported = priorImported
		s.refreshPackageSnapshotLocked()
		delete(s.state.Plans, plan.ID)
		delete(s.state.Imports, plan.ID)
		return Plan{}, err
	}
	published = true
	return clonePlan(plan), nil
}
