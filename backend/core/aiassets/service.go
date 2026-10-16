package aiassets

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type persistedState struct {
	HistoryPending  []HistoryEvent                `json:"historyPending,omitempty"`
	HistoryObserved map[string]string             `json:"historyObserved,omitempty"`
	ManagedLayout   int                           `json:"managedLayout,omitempty"`
	Initialization  map[string]map[string]string  `json:"initialization,omitempty"`
	Imported        map[string]map[string]Package `json:"imported,omitempty"`
	Custom          map[string]map[string]Package `json:"custom,omitempty"`
	Plans           map[string]Plan               `json:"plans"`
	Jobs            map[string]Job                `json:"jobs"`
	Imports         map[string]string             `json:"imports"`
}
type activeSnapshot struct {
	SchemaVersion int               `json:"schemaVersion"`
	Generation    uint64            `json:"generation"`
	Packages      map[string]string `json:"packages"`
}
type Service struct {
	mu              sync.Mutex
	root            string
	dir             string
	opts            Options
	providers       map[string]Provider
	packages        map[string]map[string]Package
	capabilities    map[string]Capability
	state           persistedState
	active          activeSnapshot
	snapshot        atomic.Value // immutable map[string]ResolvedPackage; Resolve never takes mu.
	packageSnapshot atomic.Value // immutable map of supported and accepted custom releases.
	unlock          func() error
	ctx             context.Context
	cancel          context.CancelFunc
	wake            chan struct{}
	done            chan struct{}
	runningID       string
	runningCancel   context.CancelFunc
	closed          bool
	lastProgress    time.Time
}

func NewService(root string, options Options) (*Service, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err = ensureDirectory(abs); err != nil {
		return nil, err
	}
	if options.MaxArchiveBytes <= 0 {
		options.MaxArchiveBytes = 8 << 30
	}
	if options.MaxExpandedBytes <= 0 {
		options.MaxExpandedBytes = 16 << 30
	}
	if options.MaxPackageBytes <= 0 {
		options.MaxPackageBytes = 8 << 30
	}
	if options.MinFreeBytes == 0 {
		options.MinFreeBytes = 64 << 20
	}
	if options.AvailableSpace == nil {
		options.AvailableSpace = availableSpace
	}
	providers := options.Providers
	if providers == nil {
		providers = Providers()
	}
	s := &Service{root: abs, dir: filepath.Join(abs, ".aiassets"), opts: options, providers: map[string]Provider{}, packages: map[string]map[string]Package{}, capabilities: map[string]Capability{}, wake: make(chan struct{}, 1), done: make(chan struct{})}
	for _, provider := range providers {
		provider = cloneProvider(provider)
		if err = validateProvider(provider); err != nil {
			return nil, err
		}
		if _, exists := s.providers[provider.ID]; exists {
			return nil, errCode("conflict", "duplicate provider")
		}
		s.providers[provider.ID] = provider
		for _, c := range provider.Capabilities {
			if _, ok := s.capabilities[c.ID]; ok {
				return nil, errCode("conflict", "duplicate capability")
			}
			c.Provider = provider.ID
			s.capabilities[c.ID] = c
		}
		for _, p := range provider.Packages {
			var bytes int64
			for _, f := range p.Files {
				if f.Bytes > options.MaxPackageBytes-bytes {
					return nil, errCode("invalid_request", "package size exceeds configured limit")
				}
				bytes += f.Bytes
			}
			if s.packages[p.ID] == nil {
				s.packages[p.ID] = map[string]Package{}
			}
			if _, ok := s.packages[p.ID][p.Version]; ok {
				return nil, errCode("conflict", "duplicate package version")
			}
			for _, prior := range s.packages[p.ID] {
				if prior.Kind != p.Kind || prior.Provider != p.Provider {
					return nil, errCode("conflict", "package identity changed across versions")
				}
			}
			s.packages[p.ID][p.Version] = p
		}
	}
	for _, sub := range []string{"", "staging", "imports"} {
		if err = ensureDirectory(filepath.Join(s.dir, sub)); err != nil {
			return nil, err
		}
	}
	s.unlock, err = acquireLock(filepath.Join(s.dir, "installer.lock"))
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			_ = s.unlock()
		}
	}()
	s.state = persistedState{Plans: map[string]Plan{}, Jobs: map[string]Job{}, Imports: map[string]string{}}
	if err = loadJSON(filepath.Join(s.dir, "state.json"), &s.state); err != nil && !os.IsNotExist(err) {
		return nil, errCode("storage_error", "cannot read model installer state: "+err.Error())
	}
	if s.state.Plans == nil {
		s.state.Plans = map[string]Plan{}
	}
	if s.state.Jobs == nil {
		s.state.Jobs = map[string]Job{}
	}
	if s.state.Imports == nil {
		s.state.Imports = map[string]string{}
	}
	if s.state.Custom == nil {
		s.state.Custom = map[string]map[string]Package{}
	}
	if s.state.Imported == nil {
		s.state.Imported = map[string]map[string]Package{}
	}
	if s.state.Initialization == nil {
		s.state.Initialization = map[string]map[string]string{}
	}
	s.active = activeSnapshot{SchemaVersion: 1, Packages: map[string]string{}}
	if err = loadJSON(filepath.Join(s.dir, "active.json"), &s.active); err != nil && !os.IsNotExist(err) {
		return nil, errCode("storage_error", "cannot read active model snapshot: "+err.Error())
	}
	if s.active.SchemaVersion != 1 {
		return nil, errCode("storage_error", "unsupported active snapshot version")
	}
	if s.active.Packages == nil {
		s.active.Packages = map[string]string{}
	}
	aliasesChanged, err := s.normalizeReleaseAliases(providers)
	if err != nil {
		return nil, err
	}
	for _, versions := range s.state.Imported {
		for _, p := range versions {
			if err = s.packageLimit(p); err != nil {
				return nil, err
			}
			if err = validateImported(providers, p); err != nil {
				return nil, errCode("storage_error", "imported release is invalid or unsupported by this build: "+p.ID)
			}
			if existing, ok := s.packages[p.ID][p.Version]; ok && !sameReleaseContent(existing, p) {
				return nil, errCode("storage_error", "imported release conflicts with compiled identity")
			}
			if s.packages[p.ID] == nil {
				s.packages[p.ID] = map[string]Package{}
			}
			s.packages[p.ID][p.Version] = clonePackage(p)
		}
	}
	for _, versions := range s.state.Custom {
		for _, p := range versions {
			if err = s.packageLimit(p); err != nil {
				return nil, err
			}
			if err = validateCustom(providers, p); err != nil {
				return nil, errCode("storage_error", "custom release is invalid or unsupported by this build: "+p.ID)
			}
			if s.packages[p.ID] == nil {
				s.packages[p.ID] = map[string]Package{}
			}
			s.packages[p.ID][p.Version] = clonePackage(p)
		}
	}
	s.refreshPackageSnapshotLocked()
	if s.state.HistoryObserved == nil {
		s.state.HistoryObserved = map[string]string{}
	}
	if err = s.flushHistoryLocked(); err != nil {
		return nil, err
	}
	historyBefore := cloneActivePackages(s.active.Packages)
	if err = s.initializeManagedLayout(providers, aliasesChanged); err != nil {
		return nil, err
	}
	if err = s.bootstrapHistoryLocked(historyBefore); err != nil {
		return nil, err
	}
	for id, job := range s.state.Jobs {
		if !safeID(id) || !safeID(job.PlanID) {
			return nil, errCode("storage_error", "invalid persisted job identifier")
		}
		switch job.State {
		case "downloading", "verifying", "publishing":
			job.State = "interrupted"
			job.ErrorCode = "interrupted"
			job.Error = "installation interrupted; retry to continue"
			job.UpdatedAt = time.Now().UTC()
			s.state.Jobs[id] = job
		}
	}
	for id, plan := range s.state.Plans {
		if id != plan.ID || !safeID(id) {
			return nil, errCode("storage_error", "invalid persisted plan")
		}
		if err = s.checkPlanDescriptors(plan); err != nil {
			for jid, j := range s.state.Jobs {
				if j.PlanID == id && j.State == "queued" {
					j.State = "failed"
					j.ErrorCode = "unsupported"
					j.Error = "release is unavailable in this build"
					s.state.Jobs[jid] = j
				}
			}
		}
	}
	for id, path := range s.state.Imports {
		if !safeID(id) || path != id {
			return nil, errCode("storage_error", "invalid persisted import")
		}
	}
	s.cleanOrphanImportsLocked()
	_ = s.expireImportPreviewsLocked(time.Now().UTC())
	if err = s.refreshSnapshotLocked(); err != nil {
		return nil, err
	}
	if err = s.saveLocked(); err != nil {
		return nil, err
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.configureClient()
	success = true
	go s.worker()
	s.signal()
	return s, nil
}
func (s *Service) configureClient() {
	base := s.opts.Client
	if base == nil {
		base = &http.Client{Timeout: 30 * time.Minute}
	}
	c := *base
	prior := c.CheckRedirect
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 8 {
			return errCode("download_failed", "too many download redirects")
		}
		if err := s.allowedURL(req.URL); err != nil {
			return err
		}
		if prior != nil {
			return prior(req, via)
		}
		return nil
	}
	s.opts.Client = &c
}
func (s *Service) allowedURL(u *url.URL) error {
	if u == nil || u.User != nil || u.Hostname() == "" || (u.Scheme != "https" && !(s.opts.AllowHTTP && u.Scheme == "http")) {
		return errCode("download_failed", "download URL must use trusted HTTPS")
	}
	allowed := s.opts.AllowedHosts
	if allowed == nil {
		for _, provider := range s.providers {
			allowed = append(allowed, provider.DownloadHosts...)
		}
		for _, versions := range s.releaseSnapshot() {
			for _, p := range versions {
				for _, f := range p.Files {
					parsed, _ := url.Parse(f.URL)
					if parsed != nil && parsed.Host != "" {
						allowed = append(allowed, parsed.Host)
					}
				}
			}
		}
	}
	for _, host := range allowed {
		if strings.EqualFold(host, u.Host) {
			return nil
		}
	}
	return errCode("download_failed", "download host is not allowed")
}
func (s *Service) Root() string { return s.root }
func (s *Service) saveLocked() error {
	if err := atomicJSON(filepath.Join(s.dir, "state.json"), s.state); err != nil {
		return errCode("storage_error", err.Error())
	}
	return nil
}
func (s *Service) signal() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}
func (s *Service) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.cancel()
	if s.runningCancel != nil {
		s.runningCancel()
	}
	s.mu.Unlock()
	<-s.done
	return s.unlock()
}
func (s *Service) packageDir(p Package) string {
	return filepath.Join(s.root, p.ID, p.Version)
}
func (s *Service) installed(p Package) bool {
	return validReceipt(s.packageDir(p), p)
}
func (s *Service) refreshSnapshotLocked() error {
	next := map[string]ResolvedPackage{}
	for id, version := range s.active.Packages {
		p, ok := s.packages[id][version]
		if !ok || !s.installed(p) {
			continue
		}
		if _, duplicate := next[p.Kind]; duplicate {
			return errCode("storage_error", "multiple active packages for resource kind")
		}
		next[p.Kind] = ResolvedPackage{Package: clonePackage(p), Directory: s.packageDir(p), EntryPath: filepath.Join(s.packageDir(p), filepath.FromSlash(p.Entry)), Fingerprint: fingerprint(p)}
	}
	s.snapshot.Store(next)
	return nil
}
func (s *Service) Resolve(kind string) (ResolvedPackage, bool) {
	v := s.snapshot.Load()
	if v == nil {
		return ResolvedPackage{}, false
	}
	p, ok := v.(map[string]ResolvedPackage)[kind]
	if ok && !s.installed(p.Package) {
		return ResolvedPackage{}, false
	}
	p.Package = clonePackage(p.Package)
	return p, ok
}
func (s *Service) Catalog() Catalog {
	s.mu.Lock()
	active := map[string]string{}
	for id, v := range s.active.Packages {
		active[id] = v
	}
	jobs := s.jobsLocked()
	generation := s.active.Generation
	packages := cloneReleases(s.packages)
	s.mu.Unlock()
	result := Catalog{Generation: generation, Capabilities: []CapabilityStatus{}, Packages: []PackageStatus{}, Jobs: jobs}
	ready := map[string]bool{}
	// installedOnly covers releases that are on disk but do not own their
	// resource kind. A capability whose only gap is such a package is already
	// delivered — it just needs activation, not another download.
	installedOnly := map[string]bool{}
	// kindsInUse remembers which resource kinds the runtime is currently using,
	// so an unactivated package can explain itself as `in_use` instead of a
	// generic pending state.
	kindsInUse := map[string]bool{}
	ids := sortedKeys(packages)
	for _, id := range ids {
		versions := sortedKeys(packages[id])
		for _, version := range versions {
			p := packages[id][version]
			provider := s.providers[p.Provider]
			installed := s.installed(p)
			status := PackageStatus{Package: clonePackage(p), Usage: s.packageUsage(p, packages), State: "missing", Installed: installed, InstalledAt: s.packageInstalledAt(p), Active: installed && active[id] == version}
			if status.Active {
				status.State = "active"
				ready[id] = true
			} else if installed {
				status.State = "candidate"
				installedOnly[id] = true
			} else if p.OfflineOnly {
				status.State = "offline_only"
			}
			hasActiveCompatible := false
			for activeID, activeVersion := range active {
				if installedActive, ok := packages[activeID][activeVersion]; ok && s.installed(installedActive) && compatibleRelease(p, installedActive) {
					ready[id] = true
					hasActiveCompatible = true
				}
			}
			if provider.Legacy != nil && provider.Legacy(s.root, p.Kind) {
				ready[id] = true
				if !installed && !hasActiveCompatible {
					status.State = "legacy"
				}
			}
			if provider.InUse != nil {
				status.InUse = provider.InUse(p.Kind)
				if status.InUse {
					kindsInUse[p.Kind] = true
				}
			}
			if provider.References != nil {
				status.References = provider.References(s.root, p.Kind)
			}
			if !installed {
				if info, err := os.Lstat(s.packageDir(p)); err == nil && info.IsDir() {
					status.State = "incomplete"
					status.ReasonCode = "integrity"
				}
				for _, j := range jobs {
					if j.PackageID == id && j.State == "failed" {
						status.ReasonCode = j.ErrorCode
					}
				}
			}
			result.Packages = append(result.Packages, status)
		}
	}
	for _, id := range sortedKeys(s.capabilities) {
		c := s.capabilities[id]
		status := CapabilityStatus{Capability: c, State: "ready"}
		if c.UnsupportedReason != "" {
			status.State = "unsupported"
			status.ReasonCode = c.UnsupportedReason
		} else {
			seen := map[string]bool{}
			var visit func(string)
			visit = func(cid string) {
				if seen[cid] {
					return
				}
				seen[cid] = true
				dep, ok := s.capabilities[cid]
				if !ok || dep.UnsupportedReason != "" {
					status.State = "unsupported"
					status.ReasonCode = "unsupported"
					return
				}
				for _, pid := range dep.Packages {
					if !ready[pid] {
						status.Missing = append(status.Missing, pid)
						if installedOnly[pid] {
							status.Unactivated = append(status.Unactivated, pid)
						}
					}
				}
				for _, d := range dep.Dependencies {
					visit(d)
				}
			}
			visit(id)
			sort.Strings(status.Missing)
			status.Missing = unique(status.Missing)
			sort.Strings(status.Unactivated)
			status.Unactivated = unique(status.Unactivated)
			if len(status.Missing) > 0 && status.State != "unsupported" {
				status.State = "missing"
				status.ReasonCode = "missing_dependency"
				// Every "missing" package is in fact already on this machine and only
				// needs its resource kind released: the capability is delivered, so
				// report it as candidate and drop Missing (which would otherwise read
				// as "download these"). Unactivated carries the precise list.
				if len(status.Unactivated) == len(status.Missing) {
					status.State = "candidate"
					status.ReasonCode = "pending_activation"
					status.Missing = nil
					for _, pid := range status.Unactivated {
						if kindInUse(pid, packages, kindsInUse) {
							status.ReasonCode = "in_use"
							break
						}
					}
				}
			}
			if check := s.providers[c.Provider].CapabilityReason; status.State == "ready" && check != nil {
				if reason := check(s.root, c); reason != "" {
					status.State = "incomplete"
					status.ReasonCode = reason
				}
			}
		}
		result.Capabilities = append(result.Capabilities, status)
	}
	return result
}

// kindInUse reports whether any release of a package shares a resource kind the
// runtime is currently using. Callers hold no lock: like Catalog itself, it only
// reads the cloned release map.
func kindInUse(id string, packages map[string]map[string]Package, kindsInUse map[string]bool) bool {
	for _, version := range packages[id] {
		if kindsInUse[version.Kind] {
			return true
		}
	}
	return false
}
func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
func unique(a []string) []string {
	if len(a) == 0 {
		return a
	}
	result := a[:1]
	for _, s := range a[1:] {
		if s != result[len(result)-1] {
			result = append(result, s)
		}
	}
	return result
}
func newer(a, b string) bool {
	aa := strings.Split(strings.TrimPrefix(a, "v"), ".")
	bb := strings.Split(strings.TrimPrefix(b, "v"), ".")
	for i := 0; i < len(aa) && i < len(bb); i++ {
		an, ae := strconv.Atoi(aa[i])
		bn, be := strconv.Atoi(bb[i])
		if ae == nil && be == nil && an != bn {
			return an > bn
		}
		if aa[i] != bb[i] {
			return aa[i] > bb[i]
		}
	}
	return a > b
}
func (s *Service) Plan(request PlanRequest) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Plan{}, errCode("conflict", "installer is closed")
	}
	plan, err := s.planLocked(request)
	if err != nil {
		return Plan{}, err
	}
	if err := s.checkSpace(plan.DownloadBytes); err != nil {
		return Plan{}, err
	}
	s.state.Plans[plan.ID] = plan
	if err = s.saveLocked(); err != nil {
		delete(s.state.Plans, plan.ID)
		return Plan{}, err
	}
	return clonePlan(plan), nil
}
func clonePlan(p Plan) Plan {
	p.Usage = append([]PackageUsage(nil), p.Usage...)
	for i := range p.Usage {
		p.Usage[i].Capabilities = append([]string(nil), p.Usage[i].Capabilities...)
		p.Usage[i].RequiredBy = append([]string(nil), p.Usage[i].RequiredBy...)
	}
	p.LegacyKinds = append([]string(nil), p.LegacyKinds...)
	p.Capabilities = append([]string(nil), p.Capabilities...)
	p.Packages = append([]Package(nil), p.Packages...)
	for i := range p.Packages {
		p.Packages[i] = clonePackage(p.Packages[i])
	}
	return p
}
func (s *Service) planLocked(request PlanRequest) (Plan, error) {
	if len(request.Capabilities) == 0 && len(request.Packages) == 0 {
		return Plan{}, errCode("invalid_request", "select at least one capability or package")
	}
	plan := Plan{ID: newID(), Generation: s.active.Generation, CreatedAt: time.Now().UTC(), Capabilities: append([]string(nil), request.Capabilities...)}
	sort.Strings(plan.Capabilities)
	plan.Capabilities = unique(plan.Capabilities)
	capVisited := map[string]int{}
	packageVisited := map[string]int{}
	kinds := map[string]string{}
	selected := map[string]Package{}
	for _, id := range request.Packages {
		versions := s.packages[id]
		v := request.Versions[id]
		if v == "" {
			for candidate := range versions {
				if v == "" || newer(candidate, v) {
					v = candidate
				}
			}
		}
		p, ok := versions[v]
		if !ok {
			return Plan{}, errCode("unsupported", "selected package is unavailable")
		}
		if prior, ok := selected[p.Kind]; ok && prior.ID != id {
			return Plan{}, errCode("conflict", "multiple packages explicitly selected for resource kind")
		}
		selected[p.Kind] = p
	}
	var pkgVisit func(string) error
	pkgVisit = func(id string) error {
		if packageVisited[id] == 1 {
			return errCode("invalid_request", "cyclic package dependency")
		}
		if packageVisited[id] == 2 {
			return nil
		}
		for _, expected := range s.packages[id] {
			if chosen, ok := selected[expected.Kind]; ok && chosen.ID != id && compatibleRelease(expected, chosen) {
				packageVisited[id] = 2
				return pkgVisit(chosen.ID)
			}
		}
		explicitKind := false
		for _, expected := range s.packages[id] {
			if _, ok := selected[expected.Kind]; ok {
				explicitKind = true
			}
		}
		if request.Versions[id] == "" && !explicitKind {
			if alternative := s.alternativeActiveLocked(id); alternative != "" {
				packageVisited[id] = 2
				return pkgVisit(alternative)
			}
		}
		versions := s.packages[id]
		if len(versions) == 0 {
			return errCode("unsupported", "package unavailable: "+id)
		}
		v := request.Versions[id]
		if v == "" {
			v = s.active.Packages[id]
		}
		if v == "" {
			for candidate := range versions {
				if v == "" || newer(candidate, v) {
					v = candidate
				}
			}
		}
		p, ok := versions[v]
		if !ok {
			return errCode("unsupported", "package version unavailable: "+id)
		}
		if _, explicit := selected[p.Kind]; !explicit && request.Versions[id] == "" && !s.installed(p) {
			if legacy := s.providers[p.Provider].Legacy; legacy != nil && legacy(s.root, p.Kind) {
				packageVisited[id] = 2
				plan.LegacyKinds = append(plan.LegacyKinds, p.Kind)
				return nil
			}
		}
		if other := kinds[p.Kind]; other != "" && other != id {
			return errCode("conflict", "multiple packages selected for kind: "+p.Kind)
		}
		kinds[p.Kind] = id
		packageVisited[id] = 1
		for _, dep := range p.Dependencies {
			if err := pkgVisit(dep); err != nil {
				return err
			}
		}
		packageVisited[id] = 2
		plan.Packages = append(plan.Packages, clonePackage(p))
		if !s.installed(p) {
			for _, f := range p.Files {
				if f.Bytes > s.opts.MaxExpandedBytes-plan.DownloadBytes {
					return errCode("invalid_request", "plan size exceeds configured limit")
				}
				plan.DownloadBytes += f.Bytes
			}
		}
		return nil
	}
	var capVisit func(string) error
	capVisit = func(id string) error {
		if capVisited[id] == 1 {
			return errCode("invalid_request", "cyclic capability dependency")
		}
		if capVisited[id] == 2 {
			return nil
		}
		c, ok := s.capabilities[id]
		if !ok || c.UnsupportedReason != "" {
			return errCode("unsupported", "capability unavailable: "+id)
		}
		capVisited[id] = 1
		for _, dep := range c.Dependencies {
			if err := capVisit(dep); err != nil {
				return err
			}
		}
		for _, pid := range c.Packages {
			if err := pkgVisit(pid); err != nil {
				return err
			}
		}
		capVisited[id] = 2
		return nil
	}
	for _, id := range plan.Capabilities {
		if err := capVisit(id); err != nil {
			return Plan{}, err
		}
	}
	for _, id := range request.Packages {
		if err := pkgVisit(id); err != nil {
			return Plan{}, err
		}
	}
	for id := range request.Versions {
		if packageVisited[id] != 2 {
			return Plan{}, errCode("invalid_request", "version selection is outside the plan")
		}
	}
	if uint64(plan.DownloadBytes) > ^uint64(0)-s.opts.MinFreeBytes {
		return Plan{}, errCode("invalid_request", "disk estimate exceeds supported size")
	}
	plan.RequiredDiskBytes = uint64(plan.DownloadBytes) + s.opts.MinFreeBytes
	plan.Usage = s.planUsageLocked(plan)
	return plan, nil
}
func (s *Service) checkPlanDescriptors(plan Plan) error {
	for _, p := range plan.Packages {
		trusted, ok := s.releaseSnapshot()[p.ID][p.Version]
		if !ok || fingerprint(trusted) != fingerprint(p) {
			return errCode("unsupported", "planned release no longer supported")
		}
	}
	return nil
}
func (s *Service) Install(planID, key string, actor ...HistoryActor) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Job{}, errCode("conflict", "installer is closed")
	}
	if len(key) > 200 {
		return Job{}, errCode("invalid_request", "idempotency key too long")
	}
	if key != "" {
		for _, j := range s.state.Jobs {
			if j.IdempotencyKey == key {
				if j.PlanID != planID {
					return Job{}, errCode("conflict", "idempotency key belongs to another plan")
				}
				return j, nil
			}
		}
	}
	p, ok := s.state.Plans[planID]
	if !ok {
		return Job{}, errCode("not_found", "installation plan not found")
	}
	if err := s.checkPlanDescriptors(p); err != nil {
		return Job{}, err
	}
	if p.Offline && p.Generation != s.active.Generation {
		return Job{}, errCode("plan_stale", "active models changed; preview the offline archive again")
	}
	if !p.Offline {
		for _, pkg := range p.Packages {
			if pkg.OfflineOnly && !s.installed(pkg) {
				return Job{}, errCode("unsupported", "package requires offline import: "+pkg.ID)
			}
		}
	}
	if err := s.checkSpace(p.DownloadBytes); err != nil {
		return Job{}, err
	}
	j := Job{ID: newID(), PlanID: planID, IdempotencyKey: key, State: "queued", TotalBytes: p.DownloadBytes, Attempt: 1, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	j.Actor = operationActor(actor)
	s.state.Jobs[j.ID] = j
	if err := s.saveLocked(); err != nil {
		delete(s.state.Jobs, j.ID)
		return Job{}, err
	}
	s.signal()
	return j, nil
}
func (s *Service) jobsLocked() []Job {
	jobs := make([]Job, 0, len(s.state.Jobs))
	for _, j := range s.state.Jobs {
		jobs = append(jobs, j)
	}
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].CreatedAt.Before(jobs[j].CreatedAt) })
	return jobs
}
func (s *Service) Jobs() []Job { s.mu.Lock(); defer s.mu.Unlock(); return s.jobsLocked() }
func (s *Service) Job(id string) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.state.Jobs[id]
	if !ok {
		return Job{}, errCode("not_found", "installation job not found")
	}
	return j, nil
}
func (s *Service) Cancel(id string) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.state.Jobs[id]
	if !ok {
		return Job{}, errCode("not_found", "installation job not found")
	}
	if j.State == "succeeded" || j.State == "failed" || j.State == "cancelled" || j.State == "interrupted" {
		return j, nil
	}
	j.State = "cancelled"
	j.ErrorCode = "cancelled"
	j.UpdatedAt = time.Now().UTC()
	s.state.Jobs[id] = j
	if s.runningID == id && s.runningCancel != nil {
		s.runningCancel()
	}
	if err := s.saveLocked(); err != nil {
		return Job{}, err
	}
	return j, nil
}
func (s *Service) Retry(id string, actor ...HistoryActor) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Job{}, errCode("conflict", "installer is closed")
	}
	j, ok := s.state.Jobs[id]
	if !ok {
		return Job{}, errCode("not_found", "installation job not found")
	}
	if s.runningID == id {
		return Job{}, errCode("conflict", "job is still stopping")
	}
	if j.State != "failed" && j.State != "cancelled" && j.State != "interrupted" {
		return Job{}, errCode("conflict", "only stopped jobs can be retried")
	}
	p, ok := s.state.Plans[j.PlanID]
	if !ok {
		return Job{}, errCode("not_found", "installation plan not found")
	}
	if err := s.checkPlanDescriptors(p); err != nil {
		return Job{}, err
	}
	j.State = "queued"
	if len(actor) > 0 {
		j.Actor = operationActor(actor)
	}
	j.Error = ""
	j.ErrorCode = ""
	j.CompletedBytes = 0
	j.Attempt++
	j.UpdatedAt = time.Now().UTC()
	s.state.Jobs[id] = j
	if err := s.saveLocked(); err != nil {
		return Job{}, err
	}
	s.signal()
	return j, nil
}
func failureCode(err error) string {
	if isNoSpace(err) {
		return "disk_insufficient"
	}
	var typed *Error
	if errors.As(err, &typed) {
		return typed.Code
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	var pathError *os.PathError
	if errors.As(err, &pathError) {
		return "storage_error"
	}
	return "validation_failed"
}
func (s *Service) checkSpace(bytes int64) error {
	if bytes < 0 {
		return errCode("invalid_request", "invalid disk space estimate")
	}
	free, err := s.opts.AvailableSpace(s.dir)
	if err != nil {
		return errCode("storage_error", "cannot inspect model disk capacity")
	}
	needed := uint64(bytes)
	if needed > ^uint64(0)-s.opts.MinFreeBytes || free < needed+s.opts.MinFreeBytes {
		return errCode("disk_insufficient", "model installation requires more free disk capacity")
	}
	return nil
}
func (s *Service) mutateGuarded(kinds []string, change func() error) error {
	byProvider := map[string][]string{}
	for _, kind := range kinds {
		for id, p := range s.providers {
			for _, pkg := range p.Packages {
				if pkg.Kind == kind {
					byProvider[id] = append(byProvider[id], kind)
					break
				}
			}
		}
	}
	ids := sortedKeys(byProvider)
	var run func(int) error
	run = func(i int) error {
		if i == len(ids) {
			for id, ks := range byProvider {
				p := s.providers[id]
				if p.Guard == nil && p.InUse != nil {
					for _, k := range ks {
						if p.InUse(k) {
							return errCode("in_use", "model is currently in use: "+k)
						}
					}
				}
			}
			return change()
		}
		p := s.providers[ids[i]]
		ks := byProvider[ids[i]]
		sort.Strings(ks)
		ks = unique(ks)
		if p.Guard != nil {
			return p.Guard(ks, func() error { return run(i + 1) })
		}
		return run(i + 1)
	}
	return run(0)
}
func (s *Service) notify(kinds []string) {
	if len(kinds) == 0 {
		return
	}
	sort.Strings(kinds)
	kinds = unique(kinds)
	for _, p := range s.providers {
		if p.Changed != nil {
			p.Changed(append([]string(nil), kinds...))
		}
	}
}
func (s *Service) Activate(request ActivateRequest, actor ...HistoryActor) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return errCode("conflict", "installer is closed")
	}
	if len(request.Packages) == 0 {
		s.mu.Unlock()
		return errCode("invalid_request", "select at least one candidate")
	}
	if request.ExpectedGeneration != nil && *request.ExpectedGeneration != s.active.Generation {
		s.mu.Unlock()
		return errCode("plan_stale", "active model snapshot changed; refresh before activation")
	}
	next := map[string]string{}
	for id, v := range s.active.Packages {
		next[id] = v
	}
	kinds := []string{}
	for id, v := range request.Packages {
		p, ok := s.packages[id][v]
		if !ok || !s.installed(p) {
			s.mu.Unlock()
			return errCode("not_found", "verified package not found")
		}
		if next[id] != v {
			kinds = append(kinds, p.Kind)
		}
		for activeID, activeVersion := range next {
			if activeID != id && s.packages[activeID][activeVersion].Kind == p.Kind {
				delete(next, activeID)
			}
		}
		next[id] = v
	}
	err := s.validateActive(next)
	if err == nil {
		err = s.mutateGuarded(kinds, func() error {
			return s.mutateWithHistoryLocked(s.switchHistoryLocked(next, operationActor(actor)), func() error {
				snapshot := activeSnapshot{SchemaVersion: 1, Generation: s.active.Generation + 1, Packages: next}
				if err := atomicJSON(filepath.Join(s.dir, "active.json"), snapshot); err != nil {
					return errCode("storage_error", err.Error())
				}
				s.active = snapshot
				return s.refreshSnapshotLocked()
			})
		})
	}
	s.mu.Unlock()
	if err == nil {
		s.notify(kinds)
	}
	return err
}
func (s *Service) validateActive(next map[string]string) error {
	kinds := map[string]string{}
	for id, v := range next {
		p, ok := s.packages[id][v]
		if !ok || !s.installed(p) {
			return errCode("missing_dependency", "active package is not verified: "+id)
		}
		if prior := kinds[p.Kind]; prior != "" && prior != id {
			return errCode("conflict", "resource kind already active: "+p.Kind)
		}
		kinds[p.Kind] = id
		for _, dep := range p.Dependencies {
			if !s.dependencySatisfiedLocked(next, dep) {
				return errCode("missing_dependency", "active dependency missing: "+dep)
			}
		}
	}
	return nil
}
func (s *Service) Verify(id, version string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.packages[id][version]
	if !ok {
		return errCode("not_found", "package not found")
	}
	if !s.installed(p) {
		return errCode("integrity", "package verification receipt or resource metadata changed")
	}
	if err := validateFiles(s.packageDir(p), p); err != nil {
		return err
	}
	if validate := s.providers[p.Provider].Validate; validate != nil {
		return validate(s.ctx, p, s.packageDir(p))
	}
	return nil
}
func (s *Service) Delete(id, version string, actor ...HistoryActor) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return errCode("conflict", "installer is closed")
	}
	p, ok := s.packages[id][version]
	if !ok {
		s.mu.Unlock()
		return errCode("not_found", "verified package not found")
	}
	if _, err := os.Lstat(s.packageDir(p)); err != nil {
		s.mu.Unlock()
		return errCode("not_found", "package directory not found")
	}
	if err := checkDirectory(s.packageDir(p)); err != nil {
		s.mu.Unlock()
		return errCode("storage_error", err.Error())
	}
	active := s.active.Packages[id] == version
	next := map[string]string{}
	for pid, v := range s.active.Packages {
		if pid != id || !active {
			next[pid] = v
		}
	}
	kinds := []string{}
	if active {
		if err := s.validateActive(next); err != nil {
			s.mu.Unlock()
			return errCode("referenced", "model required by another active package")
		}
		for pid, v := range next {
			for _, dep := range s.packages[pid][v].Dependencies {
				if dep == id {
					s.mu.Unlock()
					return errCode("referenced", "model required by installed package: "+pid)
				}
			}
		}
		provider := s.providers[p.Provider]
		if provider.References != nil && len(provider.References(s.root, p.Kind)) > 0 {
			s.mu.Unlock()
			return errCode("referenced", "model referenced by enabled configuration")
		}
		kinds = append(kinds, p.Kind)
	}
	// A queued/running plan owns its package until it reaches a terminal state.
	for _, j := range s.state.Jobs {
		if j.State == "queued" || j.State == "downloading" || j.State == "verifying" || j.State == "publishing" {
			for _, pkg := range s.state.Plans[j.PlanID].Packages {
				if pkg.ID == id && pkg.Version == version {
					s.mu.Unlock()
					return errCode("in_use", "package belongs to a running installation")
				}
			}
		}
	}
	err := s.mutateGuarded(kinds, func() error {
		return s.mutateWithHistoryLocked([]HistoryEvent{{PackageID: id, Version: version, Action: "delete", Actor: operationActor(actor)}}, func() error {
			// Persist the user's removal intent before removing files. Retained flat
			// initialization resources must never recreate this version at startup.
			prior := s.state.Initialization[p.ID][p.Version]
			s.recordInitialization(p, "deleted")
			if err := s.saveLocked(); err != nil {
				if prior == "" {
					delete(s.state.Initialization[p.ID], p.Version)
				} else {
					s.state.Initialization[p.ID][p.Version] = prior
				}
				return err
			}
			if active {
				snapshot := activeSnapshot{SchemaVersion: 1, Generation: s.active.Generation + 1, Packages: next}
				if err := atomicJSON(filepath.Join(s.dir, "active.json"), snapshot); err != nil {
					return errCode("storage_error", err.Error())
				}
				s.active = snapshot
				if err := s.refreshSnapshotLocked(); err != nil {
					return err
				}
			}
			if err := os.RemoveAll(s.packageDir(p)); err != nil {
				return errCode("storage_error", fmt.Sprintf("cannot delete package: %v", err))
			}
			return nil
		})
	})
	s.mu.Unlock()
	if err == nil {
		s.notify(kinds)
	}
	return err
}
