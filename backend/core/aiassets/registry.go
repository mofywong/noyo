package aiassets

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
	"sync"
)

var registry = struct {
	sync.RWMutex
	providers []Provider
}{}
var identifier = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,100}$`)

func RegisterProvider(provider Provider) error {
	if err := validateProvider(provider); err != nil {
		return err
	}
	registry.Lock()
	defer registry.Unlock()
	for _, p := range registry.providers {
		if p.ID == provider.ID {
			return errCode("conflict", "provider already registered: "+provider.ID)
		}
	}
	registry.providers = append(registry.providers, cloneProvider(provider))
	return nil
}
func Providers() []Provider {
	registry.RLock()
	defer registry.RUnlock()
	result := make([]Provider, len(registry.providers))
	for i, p := range registry.providers {
		result[i] = cloneProvider(p)
	}
	return result
}
func cloneProvider(p Provider) Provider {
	if p.ReleaseAliases != nil {
		aliases := make(map[string]ReleaseAlias, len(p.ReleaseAliases))
		for id, alias := range p.ReleaseAliases {
			aliases[id] = alias
		}
		p.ReleaseAliases = aliases
	}
	p.DownloadHosts = append([]string(nil), p.DownloadHosts...)
	p.Capabilities = append([]Capability(nil), p.Capabilities...)
	for i := range p.Capabilities {
		p.Capabilities[i].Packages = append([]string(nil), p.Capabilities[i].Packages...)
		p.Capabilities[i].Dependencies = append([]string(nil), p.Capabilities[i].Dependencies...)
		p.Capabilities[i].OptionalPackages = append([]string(nil), p.Capabilities[i].OptionalPackages...)
	}
	p.Packages = append([]Package(nil), p.Packages...)
	for i := range p.Packages {
		p.Packages[i] = clonePackage(p.Packages[i])
	}
	return p
}
func clonePackage(p Package) Package {
	if p.Source == "" {
		p.Source = "builtin"
	}
	p.Files = append([]File(nil), p.Files...)
	p.Dependencies = append([]string(nil), p.Dependencies...)
	return p
}
func safeID(s string) bool {
	return identifier.MatchString(s) && s != "." && s != ".." && !strings.HasSuffix(s, ".") && !reservedName(s)
}
func reservedName(s string) bool {
	s = strings.ToUpper(strings.Split(s, ".")[0])
	if s == "CON" || s == "PRN" || s == "AUX" || s == "NUL" {
		return true
	}
	if len(s) == 4 && (strings.HasPrefix(s, "COM") || strings.HasPrefix(s, "LPT")) && s[3] >= '1' && s[3] <= '9' {
		return true
	}
	return false
}
func safeRelative(s string) bool {
	if s == "" || strings.ContainsAny(s, `\:<>"|?*`) || strings.HasPrefix(s, "/") || path.Clean(s) != s {
		return false
	}
	for _, r := range s {
		if r < 32 {
			return false
		}
	}
	for _, part := range strings.Split(s, "/") {
		if part == "." || part == ".." || part == "" || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") || reservedName(part) {
			return false
		}
	}
	return true
}
func validateProvider(p Provider) error {
	if !safeID(p.ID) {
		return errCode("invalid_request", "invalid provider ID")
	}
	for _, c := range p.Capabilities {
		if !safeID(c.ID) {
			return errCode("invalid_request", "invalid capability ID")
		}
		if c.Provider != "" && c.Provider != p.ID {
			return errCode("invalid_request", "capability provider mismatch")
		}
	}
	for _, pkg := range p.Packages {
		if !safeID(pkg.ID) || !safeID(pkg.Version) || !safeID(pkg.Kind) || pkg.Provider != p.ID || !safeRelative(pkg.Entry) || pkg.Contract == "" || len(pkg.Files) == 0 {
			return errCode("invalid_request", "invalid package descriptor: "+pkg.ID)
		}
		seen := map[string]bool{}
		entry := false
		for _, f := range pkg.Files {
			key := strings.ToLower(f.Path)
			if !safeRelative(f.Path) || seen[key] || f.Bytes <= 0 || key == "receipt.json" {
				return errCode("invalid_request", "invalid package file: "+f.Path)
			}
			seen[key] = true
			entry = entry || f.Path == pkg.Entry
			h, err := hex.DecodeString(f.SHA256)
			if err != nil || len(h) != 32 {
				return errCode("invalid_request", "invalid SHA256: "+f.Path)
			}
			if f.URL != "" {
				u, err := url.Parse(f.URL)
				if err != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
					return errCode("invalid_request", "invalid package URL")
				}
			}
			if f.Inline != "" && (len(f.Inline) > 1<<20 || int64(len(f.Inline)) != f.Bytes) {
				return errCode("invalid_request", "invalid inline resource")
			}
			if !pkg.OfflineOnly && f.URL == "" && f.Inline == "" {
				return errCode("invalid_request", "online package has no download URL")
			}
		}
		if !entry {
			return errCode("invalid_request", fmt.Sprintf("package %s entry is not declared", pkg.ID))
		}
	}
	return nil
}
