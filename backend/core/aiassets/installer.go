package aiassets

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (s *Service) worker() {
	defer close(s.done)
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.wake:
		}
		for {
			s.mu.Lock()
			if s.closed {
				s.mu.Unlock()
				return
			}
			var job Job
			found := false
			for _, j := range s.jobsLocked() {
				if j.State == "queued" {
					job = j
					found = true
					break
				}
			}
			if !found {
				s.mu.Unlock()
				break
			}
			ctx, cancel := context.WithCancel(s.ctx)
			s.runningID = job.ID
			s.runningCancel = cancel
			job.State = "downloading"
			job.UpdatedAt = time.Now().UTC()
			s.state.Jobs[job.ID] = job
			err := s.saveLocked()
			plan := clonePlan(s.state.Plans[job.PlanID])
			s.mu.Unlock()
			if err == nil {
				err = s.processJob(ctx, job, plan)
			}
			cancel()
			s.mu.Lock()
			latest := s.state.Jobs[job.ID]
			if latest.State != "succeeded" && latest.State != "cancelled" {
				if s.closed {
					latest.State = "interrupted"
					latest.ErrorCode = "interrupted"
					latest.Error = "installation interrupted; retry to continue"
				} else if err != nil {
					latest.State = "failed"
					latest.ErrorCode = failureCode(err)
					latest.Error = err.Error()
				} else {
					latest.State = "succeeded"
				}
			}
			latest.UpdatedAt = time.Now().UTC()
			s.state.Jobs[job.ID] = latest
			_ = s.saveLocked()
			s.runningID = ""
			s.runningCancel = nil
			s.mu.Unlock()
		}
	}
}
func (s *Service) progress(id, state, pkg, file string, bytes int64, force bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.state.Jobs[id]
	if !ok {
		return errCode("not_found", "job not found")
	}
	if j.State == "cancelled" {
		return context.Canceled
	}
	j.State = state
	j.PackageID = pkg
	j.File = file
	j.CompletedBytes = bytes
	j.UpdatedAt = time.Now().UTC()
	s.state.Jobs[id] = j
	if force || time.Since(s.lastProgress) > 250*time.Millisecond {
		s.lastProgress = time.Now()
		return s.saveLocked()
	}
	return nil
}
func (s *Service) processJob(ctx context.Context, job Job, plan Plan) error {
	if err := s.checkPlanDescriptors(plan); err != nil {
		return err
	}
	remaining := int64(0)
	for _, p := range plan.Packages {
		if s.installed(p) {
			continue
		}
		dir := filepath.Join(s.dir, "staging", job.ID, p.ID, p.Version)
		for _, f := range p.Files {
			remaining += remainingResourceBytes(dir, f, plan.Offline)
		}
	}
	if err := s.checkSpace(remaining); err != nil {
		return err
	}
	stage := filepath.Join(s.dir, "staging", job.ID)
	if err := ensureDirectory(stage); err != nil {
		return errCode("storage_error", err.Error())
	}
	completed := int64(0)
	staged := []Package{}
	for _, p := range plan.Packages {
		if err := ctx.Err(); err != nil {
			return err
		}
		if s.installed(p) {
			if err := validateFiles(s.packageDir(p), p); err != nil {
				return err
			}
			continue
		}
		dir := filepath.Join(stage, p.ID, p.Version)
		if err := ensureDirectory(dir); err != nil {
			return errCode("storage_error", err.Error())
		}
		for _, f := range p.Files {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := s.progress(job.ID, "downloading", p.ID, f.Path, completed, true); err != nil {
				return err
			}
			if err := checkFile(dir, f); err == nil {
				completed += f.Bytes
				continue
			}
			if err := s.checkSpace(remainingResourceBytes(dir, f, plan.Offline)); err != nil {
				return err
			}
			name, err := fileAt(dir, f.Path)
			if err != nil {
				return err
			}
			if err = ensureDirectory(filepath.Dir(name)); err != nil {
				return err
			}
			if f.Inline != "" {
				err = os.WriteFile(name, []byte(f.Inline), 0600)
			} else if plan.Offline {
				source := filepath.Join(s.dir, "imports", plan.ID, p.ID, p.Version)
				err = copyResource(ctx, source, dir, f)
			} else {
				err = s.download(ctx, job.ID, p.ID, dir, f, completed)
			}
			if err != nil {
				return err
			}
			if err = checkFile(dir, f); err != nil {
				return err
			}
			completed += f.Bytes
		}
		if err := s.progress(job.ID, "verifying", p.ID, "", completed, true); err != nil {
			return err
		}
		if err := validateFiles(dir, p); err != nil {
			return err
		}
		if validate := s.providers[p.Provider].Validate; validate != nil {
			if err := validate(ctx, p, dir); err != nil {
				return err
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		record, err := makeReceipt(dir, p)
		if err != nil {
			return err
		}
		if err := atomicJSON(filepath.Join(dir, "receipt.json"), record); err != nil {
			return errCode("storage_error", err.Error())
		}
		staged = append(staged, p)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.progress(job.ID, "publishing", "", "", completed, true); err != nil {
		return err
	}
	s.mu.Lock()
	if err := ctx.Err(); err != nil {
		s.mu.Unlock()
		return err
	}
	if s.state.Jobs[job.ID].State == "cancelled" {
		s.mu.Unlock()
		return context.Canceled
	}
	if plan.Offline && plan.Generation != s.active.Generation {
		s.mu.Unlock()
		return errCode("plan_stale", "active models changed after offline preview")
	}
	// All files and contracts pass before any version becomes visible. Multiple
	// version directories may survive a crash, but only one atomic snapshot enables them.
	for _, p := range staged {
		target := s.packageDir(p)
		if err := ensureDirectory(filepath.Dir(target)); err != nil {
			s.mu.Unlock()
			return err
		}
		if _, err := os.Lstat(target); err == nil {
			if !s.installed(p) {
				s.mu.Unlock()
				return errCode("integrity", "existing immutable version is invalid; remove it before retry")
			}
			continue
		} else if !os.IsNotExist(err) {
			s.mu.Unlock()
			return err
		}
		if err := s.mutateWithHistoryLocked([]HistoryEvent{{PackageID: p.ID, Version: p.Version, Action: "add", Actor: job.Actor}}, func() error {
			return os.Rename(filepath.Join(stage, p.ID, p.Version), target)
		}); err != nil {
			s.mu.Unlock()
			return errCode("storage_error", err.Error())
		}
	}
	next := map[string]string{}
	for id, v := range s.active.Packages {
		next[id] = v
	}
	kinds := []string{}
	activated := []string{}
	pending := []PendingActivation{}

	// An offline import is a vendor-built delivery or upgrade, so the intent is
	// that the imported package takes over its resource kind: an occupied kind is
	// replaced rather than left as a candidate. Online plans keep the
	// conservative behaviour and only fill a kind that is still free.
	//
	// A hand-built archive may omit a dependency. Such a package stays a
	// candidate and is reported through Pending instead of failing the job. The
	// loop repeats so a package that was only waiting for its dependency gets
	// another chance once that dependency has been adopted.
	settled := make([]bool, len(plan.Packages))
	lastErr := make([]error, len(plan.Packages))
	for progress := true; progress; {
		progress = false
		for i, p := range plan.Packages {
			if settled[i] {
				continue
			}
			if version, already := next[p.ID]; already && version == p.Version {
				settled[i] = true
				continue
			}
			occupied := false
			// Existing unmanaged models are already the runtime selection.
			if legacy := s.providers[p.Provider].Legacy; legacy != nil && legacy(s.root, p.Kind) {
				occupied = true
			}
			for id, v := range next {
				if s.packages[id][v].Kind == p.Kind {
					occupied = true
					break
				}
			}
			if !plan.Offline && occupied {
				settled[i] = true
				continue
			}
			trial := make(map[string]string, len(next)+1)
			for id, v := range next {
				if plan.Offline && s.packages[id][v].Kind == p.Kind {
					continue
				}
				trial[id] = v
			}
			trial[p.ID] = p.Version
			if validationErr := s.validateActive(trial); validationErr != nil {
				lastErr[i] = validationErr
				continue
			}
			next = trial
			kinds = append(kinds, p.Kind)
			activated = append(activated, p.ID)
			settled[i] = true
			progress = true
		}
	}
	kinds = unique(kinds)
	if plan.Offline {
		for i, p := range plan.Packages {
			if settled[i] || lastErr[i] == nil {
				continue
			}
			pending = append(pending, PendingActivation{PackageID: p.ID, Version: p.Version, Kind: p.Kind, Reason: failureCode(lastErr[i])})
		}
	}
	err := s.validateActive(next)
	if err == nil {
		err = s.mutateGuarded(kinds, func() error {
			if err := ctx.Err(); err != nil {
				return err
			}
			return s.mutateWithHistoryLocked(s.switchHistoryLocked(next, job.Actor), func() error {
				generation := s.active.Generation
				if len(kinds) > 0 {
					generation++
				}
				snapshot := activeSnapshot{SchemaVersion: 1, Generation: generation, Packages: next}
				if err := atomicJSON(filepath.Join(s.dir, "active.json"), snapshot); err != nil {
					return errCode("storage_error", err.Error())
				}
				s.active = snapshot
				return s.refreshSnapshotLocked()
			})
		})
	}
	if err == nil {
		j := s.state.Jobs[job.ID]
		j.State = "succeeded"
		j.CompletedBytes = j.TotalBytes
		j.Error = ""
		j.ErrorCode = ""
		j.Activated = activated
		j.Pending = pending
		j.UpdatedAt = time.Now().UTC()
		s.state.Jobs[job.ID] = j
		err = s.saveLocked()
	}
	s.mu.Unlock()
	if err == nil {
		s.notify(kinds)
		_ = os.RemoveAll(stage)
		if plan.Offline {
			_ = os.RemoveAll(filepath.Join(s.dir, "imports", plan.ID))
		}
	}
	return err
}

// A resumed transfer needs space only for bytes not already staged. Final size
// and SHA256 checks still cover the complete resource before publication.
func remainingResourceBytes(dir string, f File, offline bool) int64 {
	if checkFile(dir, f) == nil {
		return 0
	}
	if !offline && f.Inline == "" {
		if partial, err := fileAt(dir, f.Path+".part"); err == nil {
			if info, err := os.Stat(partial); err == nil && info.Mode().IsRegular() && info.Size() > 0 && info.Size() < f.Bytes {
				return f.Bytes - info.Size()
			}
		}
	}
	return f.Bytes
}

func copyResource(ctx context.Context, source, dest string, f File) error {
	from, err := fileAt(source, f.Path)
	if err != nil {
		return err
	}
	in, err := os.Open(from)
	if err != nil {
		return errCode("integrity", "offline resource missing: "+f.Path)
	}
	defer in.Close()
	to, err := fileAt(dest, f.Path)
	if err != nil {
		return err
	}
	out, err := os.OpenFile(to, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, &contextReader{ctx: ctx, r: io.LimitReader(in, f.Bytes+1)})
	if err == nil {
		err = out.Sync()
	}
	closeErr := out.Close()
	if err == nil {
		err = closeErr
	}
	return err
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *contextReader) Read(b []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(b)
}

type progressWriter struct {
	s              *Service
	job, pkg, file string
	base, written  int64
	w              io.Writer
}

func (w *progressWriter) Write(b []byte) (int, error) {
	n, err := w.w.Write(b)
	w.written += int64(n)
	if err == nil {
		err = w.s.progress(w.job, "downloading", w.pkg, w.file, w.base+w.written, false)
	}
	return n, err
}
func (s *Service) download(ctx context.Context, job, pkg, dir string, f File, base int64) error {
	u, err := url.Parse(f.URL)
	if err != nil {
		return errCode("download_failed", "invalid resource URL")
	}
	if err = s.allowedURL(u); err != nil {
		return err
	}
	name, err := fileAt(dir, f.Path)
	if err != nil {
		return err
	}
	partial, err := fileAt(dir, f.Path+".part")
	if err != nil {
		return err
	}
	offset := int64(0)
	if stat, statErr := os.Stat(partial); statErr == nil {
		if !stat.Mode().IsRegular() {
			return errCode("integrity", "invalid partial resource")
		}
		offset = stat.Size()
		if offset >= f.Bytes {
			_ = os.Remove(partial)
			offset = 0
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept-Encoding", "identity")
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	resp, err := s.opts.Client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errCode("download_failed", err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		offset = 0
	} else if resp.StatusCode == http.StatusPartialContent && offset > 0 {
		expected := fmt.Sprintf("bytes %d-", offset)
		if !strings.HasPrefix(resp.Header.Get("Content-Range"), expected) || !strings.HasSuffix(resp.Header.Get("Content-Range"), "/"+strconv.FormatInt(f.Bytes, 10)) {
			return errCode("download_failed", "invalid ranged download response")
		}
	} else {
		return errCode("download_failed", fmt.Sprintf("download returned HTTP %d", resp.StatusCode))
	}
	remaining := f.Bytes - offset
	if resp.ContentLength >= 0 && resp.ContentLength != remaining {
		return errCode("integrity", "download size does not match release descriptor")
	}
	flags := os.O_CREATE | os.O_WRONLY
	if offset == 0 {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_APPEND
	}
	out, err := os.OpenFile(partial, flags, 0600)
	if err != nil {
		return errCode("storage_error", err.Error())
	}
	w := &progressWriter{s: s, job: job, pkg: pkg, file: f.Path, base: base + offset, w: out}
	n, copyErr := io.Copy(w, &contextReader{ctx: ctx, r: io.LimitReader(resp.Body, remaining+1)})
	if copyErr == nil {
		copyErr = out.Sync()
	}
	closeErr := out.Close()
	if copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if isNoSpace(copyErr) {
			return errCode("disk_insufficient", "model download exhausted disk capacity")
		}
		return errCode("download_failed", copyErr.Error())
	}
	if n != remaining {
		_ = os.Remove(partial)
		return errCode("integrity", "download size mismatch: "+f.Path)
	}
	// The whole file hash protects resumed bytes and responses independently of ETags.
	if err = os.Rename(partial, name); err != nil {
		return errCode("storage_error", err.Error())
	}
	if err = checkFile(dir, f); err != nil {
		_ = os.Remove(name)
		return err
	}
	return nil
}
