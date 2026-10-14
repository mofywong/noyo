// Package aiassets manages trusted, versioned inference resources. It never loads
// executable code from downloaded or imported packages.
package aiassets

import (
	"context"
	"net/http"
	"time"
)

type Text struct {
	ZH string `json:"zh"`
	EN string `json:"en"`
}
type File struct {
	Path   string `json:"path"`
	URL    string `json:"url,omitempty"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
	Inline string `json:"inline,omitempty"`
}
type Package struct {
	Source       string   `json:"source"`
	ID           string   `json:"id"`
	Provider     string   `json:"provider"`
	Version      string   `json:"version"`
	Kind         string   `json:"kind"`
	Title        Text     `json:"title"`
	Description  Text     `json:"description"`
	License      string   `json:"license,omitempty"`
	Contract     string   `json:"contract"`
	Entry        string   `json:"entry"`
	Files        []File   `json:"files"`
	Dependencies []string `json:"dependencies,omitempty"`
	OfflineOnly  bool     `json:"offlineOnly,omitempty"`
}
type Capability struct {
	ID                string   `json:"id"`
	Provider          string   `json:"provider"`
	SceneType         string   `json:"sceneType,omitempty"`
	Category          string   `json:"category,omitempty"`
	Icon              string   `json:"icon,omitempty"`
	Title             Text     `json:"title"`
	Description       Text     `json:"description"`
	Packages          []string `json:"packages"`
	Dependencies      []string `json:"dependencies,omitempty"`
	OptionalPackages  []string `json:"optionalPackages,omitempty"`
	UnsupportedReason string   `json:"unsupportedReason,omitempty"`
}

// Provider callbacks must be concurrency safe. InUse and References are checked
// while the service mutation lock is held. Changed runs after publication.
type ReleaseAlias struct {
	ID            string
	VersionPrefix string
}

type Provider struct {
	ID               string
	Capabilities     []Capability
	Packages         []Package
	ReleaseAliases   map[string]ReleaseAlias
	DownloadHosts    []string
	Validate         func(context.Context, Package, string) error
	Legacy           func(root, kind string) bool
	CapabilityReason func(root string, capability Capability) string
	InUse            func(kind string) bool
	References       func(root, kind string) []string
	RequiredKinds    func(configJSON string) []string
	ValidateConfig   func(root, before, after string) error
	Guard            func(kinds []string, change func() error) error
	Changed          func(kinds []string)
}
type Options struct {
	RecordHistory    func(HistoryEvent) error
	Providers        []Provider
	Client           *http.Client
	AllowedHosts     []string
	AllowHTTP        bool // Reserved for injected local test transports.
	MaxArchiveBytes  int64
	MaxExpandedBytes int64
	MaxPackageBytes  int64
	MinFreeBytes     uint64
	AvailableSpace   func(path string) (uint64, error)
}
type PlanRequest struct {
	Capabilities []string          `json:"capabilities"`
	Packages     []string          `json:"packages,omitempty"`
	Versions     map[string]string `json:"versions,omitempty"`
}
type Plan struct {
	RequiredDiskBytes uint64         `json:"requiredDiskBytes"`
	Generation        uint64         `json:"generation"`
	LegacyKinds       []string       `json:"legacyKinds,omitempty"`
	ID                string         `json:"id"`
	Capabilities      []string       `json:"capabilities"`
	Packages          []Package      `json:"packages"`
	Usage             []PackageUsage `json:"usage"`
	DownloadBytes     int64          `json:"downloadBytes"`
	Offline           bool           `json:"offline"`
	CreatedAt         time.Time      `json:"createdAt"`
}

// PackageUsage is derived from compiled capabilities, never from an uploaded
// capability claim. It is not part of a release's immutable fingerprint.
type PackageUsage struct {
	PackageID    string   `json:"packageId"`
	Version      string   `json:"version"`
	Capabilities []string `json:"capabilities"`
	RequiredBy   []string `json:"requiredBy"`
	Effect       string   `json:"effect,omitempty"`
}
type Job struct {
	Actor          HistoryActor        `json:"actor,omitempty"`
	ID             string              `json:"id"`
	PlanID         string              `json:"planId"`
	IdempotencyKey string              `json:"idempotencyKey,omitempty"`
	State          string              `json:"state"`
	PackageID      string              `json:"packageId,omitempty"`
	File           string              `json:"file,omitempty"`
	CompletedBytes int64               `json:"completedBytes"`
	TotalBytes     int64               `json:"totalBytes"`
	ErrorCode      string              `json:"errorCode,omitempty"`
	Error          string              `json:"error,omitempty"`
	Attempt        int                 `json:"attempt"`
	Activated      []string            `json:"activated,omitempty"`
	Pending        []PendingActivation `json:"pending,omitempty"`
	CreatedAt      time.Time           `json:"createdAt"`
	UpdatedAt      time.Time           `json:"updatedAt"`
}

// PendingActivation reports a package that installed successfully but did not
// take over its resource kind in the same job. A hand-built offline archive may
// omit a dependency, or the runtime may still be serving the previous version.
// Reason carries the error code: `missing_dependency` needs another import,
// `in_use` clears by itself once the runtime releases the kind.
type PendingActivation struct {
	PackageID string `json:"packageId"`
	Version   string `json:"version"`
	Kind      string `json:"kind"`
	Reason    string `json:"reason"`
}
type PackageStatus struct {
	Package     Package      `json:"package"`
	Usage       PackageUsage `json:"usage"`
	State       string       `json:"state"`
	Active      bool         `json:"active"`
	Installed   bool         `json:"installed"`
	InstalledAt *time.Time   `json:"installedAt,omitempty"`
	InUse       bool         `json:"inUse"`
	References  []string     `json:"references,omitempty"`
	ReasonCode  string       `json:"reasonCode,omitempty"`
}
type CapabilityStatus struct {
	Capability Capability `json:"capability"`
	State      string     `json:"state"`
	// Missing lists required packages that are not serving and not installed, so
	// the capability genuinely needs a delivery. A `candidate` capability has
	// everything on disk and therefore reports no Missing packages.
	Missing []string `json:"missing,omitempty"`
	// Unactivated lists required packages that are installed on this machine but
	// do not own their resource kind, so the capability is not serving yet. It is
	// non-empty exactly when State is `candidate`, and lets the UI point at the
	// package to activate. Together: candidate ⇔ Missing empty ∧ Unactivated set.
	Unactivated []string `json:"unactivated,omitempty"`
	ReasonCode  string   `json:"reasonCode,omitempty"`
}
type Catalog struct {
	Generation   uint64             `json:"generation"`
	Capabilities []CapabilityStatus `json:"capabilities"`
	Packages     []PackageStatus    `json:"packages"`
	Jobs         []Job              `json:"jobs"`
}
type ActivateRequest struct {
	Packages           map[string]string `json:"packages"`
	ExpectedGeneration *uint64           `json:"expectedGeneration,omitempty"`
}
type ResolvedPackage struct {
	Package     Package `json:"package"`
	Directory   string  `json:"directory"`
	EntryPath   string  `json:"entryPath"`
	Fingerprint string  `json:"fingerprint"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string           { return e.Message }
func errCode(code, message string) error { return &Error{Code: code, Message: message} }
