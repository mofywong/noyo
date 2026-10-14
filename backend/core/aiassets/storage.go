package aiassets

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type receipt struct {
	Package     Package              `json:"package"`
	Fingerprint string               `json:"fingerprint"`
	Files       map[string]fileStamp `json:"files"`
	InstalledAt time.Time            `json:"installedAt"`
}
type fileStamp struct {
	Bytes    int64 `json:"bytes"`
	Modified int64 `json:"modified"`
}

func makeReceipt(dir string, p Package) (receipt, error) {
	r := receipt{Package: p, Fingerprint: fingerprint(p), Files: map[string]fileStamp{}, InstalledAt: time.Now().UTC()}
	var previous receipt
	if loadJSON(filepath.Join(dir, "receipt.json"), &previous) == nil && sameReleaseContent(previous.Package, p) {
		if installedAt := receiptInstalledAt(dir); installedAt != nil {
			r.InstalledAt = *installedAt
		}
	}
	for _, f := range p.Files {
		name, err := fileAt(dir, f.Path)
		if err != nil {
			return receipt{}, err
		}
		info, err := os.Stat(name)
		if err != nil {
			return receipt{}, err
		}
		r.Files[f.Path] = fileStamp{Bytes: info.Size(), Modified: info.ModTime().UnixNano()}
	}
	return r, nil
}

// Older receipts have no explicit timestamp. Retain their filesystem timestamp
// when upgrading, so switching and receipt rewrites never change install time.
func receiptInstalledAt(dir string) *time.Time {
	name, err := fileAt(dir, "receipt.json")
	if err != nil {
		return nil
	}
	var r receipt
	if loadJSON(name, &r) != nil {
		return nil
	}
	installedAt := r.InstalledAt.UTC()
	if installedAt.IsZero() {
		info, err := os.Stat(name)
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		installedAt = info.ModTime().UTC()
	}
	return &installedAt
}

// packageInstalledAt never acquires the mutation lock; audit publication may
// call it with the lock held. Only valid managed receipts supply a timestamp.
func (s *Service) packageInstalledAt(p Package) *time.Time {
	if !s.installed(p) {
		return nil
	}
	return receiptInstalledAt(s.packageDir(p))
}

func (s *Service) InstalledAt(id, version string) *time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.packages[id][version]
	if !ok {
		return nil
	}
	return s.packageInstalledAt(p)
}

func validReceipt(dir string, p Package) bool {
	var r receipt
	if loadJSON(filepath.Join(dir, "receipt.json"), &r) != nil || r.Fingerprint != fingerprint(p) || fingerprint(r.Package) != fingerprint(p) || len(r.Files) != len(p.Files) {
		return false
	}
	for _, f := range p.Files {
		stamp, ok := r.Files[f.Path]
		if !ok {
			return false
		}
		name, err := fileAt(dir, f.Path)
		if err != nil {
			return false
		}
		info, err := os.Stat(name)
		if err != nil || !info.Mode().IsRegular() || info.Size() != f.Bytes || info.Size() != stamp.Bytes || info.ModTime().UnixNano() != stamp.Modified {
			return false
		}
	}
	return true
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func fingerprint(p Package) string {
	b, _ := json.Marshal(clonePackage(p))
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func atomicJSON(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".atomic-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return replaceFile(name, path)
}
func loadJSON(path string, value any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return err
	}
	if stat.Size() > 32<<20 {
		return fmt.Errorf("state file exceeds limit")
	}
	d := json.NewDecoder(io.LimitReader(f, 32<<20))
	d.DisallowUnknownFields()
	if err = d.Decode(value); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return fmt.Errorf("unexpected trailing state data")
	}
	return nil
}
func ensureDirectory(path string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	return checkDirectory(path)
}
func checkDirectory(path string) error {
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("managed directory must not be a symbolic link: %s", current)
		}
		if parent := filepath.Dir(current); parent == current {
			break
		}
	}
	return nil
}
func fileAt(dir, relative string) (string, error) {
	if !safeRelative(relative) {
		return "", errCode("invalid_request", "unsafe resource path")
	}
	name := filepath.Join(dir, filepath.FromSlash(relative))
	if err := checkDirectory(dir); err != nil {
		return "", err
	}
	current := dir
	for _, part := range strings.Split(relative, "/") {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", errCode("integrity", "symbolic links are not model resources")
		}
	}
	return name, nil
}
func checkFile(dir string, f File) error {
	name, err := fileAt(dir, f.Path)
	if err != nil {
		return err
	}
	file, err := os.Open(name)
	if err != nil {
		return errCode("integrity", "missing resource: "+f.Path)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != f.Bytes {
		return errCode("integrity", "resource size mismatch: "+f.Path)
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(file, f.Bytes+1))
	if err != nil || n != f.Bytes || !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), f.SHA256) {
		return errCode("integrity", "resource SHA256 mismatch: "+f.Path)
	}
	return nil
}
func validateFiles(dir string, p Package) error {
	declared := map[string]bool{}
	for _, f := range p.Files {
		declared[f.Path] = true
		if err := checkFile(dir, f); err != nil {
			return err
		}
	}
	return filepath.WalkDir(dir, func(name string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errCode("integrity", "symbolic links are forbidden")
		}
		if entry.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, name)
		rel = filepath.ToSlash(rel)
		if rel != "receipt.json" && !declared[rel] {
			return errCode("integrity", "undeclared resource: "+rel)
		}
		return nil
	})
}
