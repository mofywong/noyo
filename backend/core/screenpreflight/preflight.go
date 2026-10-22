// Package screenpreflight checks a candidate release against stored screen
// engine references without initializing the application or migrating its DB.
package screenpreflight

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	_ "github.com/glebarez/go-sqlite"
)

const AssetsManifest = ".noyo-assets.json"
const assetsProtocol = "noyo.screen.assets/1"

var buildPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
var shaPattern = regexp.MustCompile(`^[a-f0-9]{40}$`)
var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

// Report contains only counts and build identifiers, never document contents.
type Report struct {
	Drafts   int      `json:"drafts"`
	Releases int      `json:"releases"`
	BuildIDs []string `json:"buildIds"`
	Files    int      `json:"files"`
}

type engine struct {
	Name        string `json:"name"`
	BuildID     string `json:"buildId"`
	UpstreamSHA string `json:"upstreamSha"`
}

type assetsManifest struct {
	Protocol    string            `json:"protocol"`
	BuildID     string            `json:"buildId"`
	UpstreamSHA string            `json:"upstreamSha"`
	Files       map[string]string `json:"files"`
}

// Run uses SQLite URI mode=ro and query_only. Every retained release is checked,
// including unpublished history and releases of soft-deleted screens: this
// storage model has no release garbage-collection policy.
func Run(ctx context.Context, database string, candidate fs.FS) (Report, error) {
	report := Report{BuildIDs: []string{}}
	if candidate == nil {
		return report, errors.New("candidate UI filesystem is unavailable")
	}
	absolute, err := filepath.Abs(database)
	if err != nil {
		return report, fmt.Errorf("resolve database: %w", err)
	}
	info, err := os.Stat(absolute)
	if err != nil || !info.Mode().IsRegular() {
		return report, fmt.Errorf("database must be an existing regular SQLite file: %s", absolute)
	}
	uriPath := filepath.ToSlash(absolute)
	if filepath.VolumeName(absolute) != "" && !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	dsn := (&url.URL{Scheme: "file", Path: uriPath, RawQuery: "mode=ro"}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return report, fmt.Errorf("open read-only database: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err = db.ExecContext(ctx, "PRAGMA query_only = ON"); err != nil {
		return report, fmt.Errorf("enable read-only queries: %w", err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return report, fmt.Errorf("begin read-only snapshot: %w", err)
	}
	defer tx.Rollback()
	var tableCount int
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name IN ('ioc_screens', 'ioc_screen_releases')").Scan(&tableCount); err != nil {
		return report, fmt.Errorf("inspect screen tables: %w", err)
	}
	if tableCount == 0 {
		return report, nil
	}
	if tableCount != 2 {
		return report, errors.New("incomplete screen schema: both ioc_screens and ioc_screen_releases must exist")
	}
	required := map[string]string{}
	rows, err := tx.QueryContext(ctx, "SELECT id, draft_json FROM ioc_screens WHERE deleted_at IS NULL ORDER BY id")
	if err != nil {
		return report, fmt.Errorf("read screen drafts: %w", err)
	}
	for rows.Next() {
		var id int64
		var document string
		if err = rows.Scan(&id, &document); err == nil {
			err = addDocument(required, document, nil)
		}
		if err != nil {
			rows.Close()
			return report, fmt.Errorf("screen draft %d: %w", id, err)
		}
		report.Drafts++
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return report, fmt.Errorf("read screen drafts: %w", err)
	}
	rows, err = tx.QueryContext(ctx, "SELECT id, content_json, engine_build_id, upstream_sha FROM ioc_screen_releases ORDER BY id")
	if err != nil {
		return report, fmt.Errorf("read screen releases: %w", err)
	}
	for rows.Next() {
		var id int64
		var document string
		var header engine
		if err = rows.Scan(&id, &document, &header.BuildID, &header.UpstreamSHA); err == nil {
			err = addDocument(required, document, &header)
		}
		if err != nil {
			rows.Close()
			return report, fmt.Errorf("screen release %d: %w", id, err)
		}
		report.Releases++
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return report, fmt.Errorf("read screen releases: %w", err)
	}
	for buildID := range required {
		report.BuildIDs = append(report.BuildIDs, buildID)
	}
	sort.Strings(report.BuildIDs)
	for _, buildID := range report.BuildIDs {
		count, checkErr := verifyEngine(ctx, candidate, buildID, required[buildID])
		if checkErr != nil {
			return report, fmt.Errorf("engine %s: %w", buildID, checkErr)
		}
		report.Files += count
	}
	return report, nil
}

func addDocument(required map[string]string, document string, header *engine) error {
	if len(document) > 5<<20 {
		return errors.New("document exceeds 5 MiB")
	}
	var envelope struct {
		SchemaVersion string `json:"schemaVersion"`
		Engine        engine `json:"engine"`
	}
	if err := json.Unmarshal([]byte(document), &envelope); err != nil {
		return errors.New("invalid document JSON")
	}
	e := envelope.Engine
	if envelope.SchemaVersion != "noyo.screen/1" || e.Name != "goview" || !buildPattern.MatchString(e.BuildID) || !shaPattern.MatchString(e.UpstreamSHA) {
		return errors.New("invalid document engine identity or schema")
	}
	if header != nil && (e.BuildID != header.BuildID || e.UpstreamSHA != header.UpstreamSHA) {
		return errors.New("release header does not match its immutable document engine")
	}
	if previous, exists := required[e.BuildID]; exists && previous != e.UpstreamSHA {
		return errors.New("one build identifier references multiple upstream commits")
	}
	required[e.BuildID] = e.UpstreamSHA
	return nil
}

func requireDirectory(candidate fs.FS, directory string) error {
	entries, err := fs.ReadDir(candidate, path.Dir(directory))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == path.Base(directory) && entry.IsDir() && entry.Type()&fs.ModeSymlink == 0 {
			return nil
		}
	}
	return fmt.Errorf("missing directory or unsafe link: %s", directory)
}

func verifyEngine(ctx context.Context, candidate fs.FS, buildID, upstreamSHA string) (int, error) {
	if err := requireDirectory(candidate, "screen-editor"); err != nil {
		return 0, err
	}
	directory := path.Join("screen-editor", buildID)
	if err := requireDirectory(candidate, directory); err != nil {
		return 0, err
	}
	manifestBytes, err := readLimited(candidate, path.Join(directory, AssetsManifest), 4<<20)
	if err != nil {
		return 0, fmt.Errorf("asset digest manifest unavailable; restore this exact engine from a trusted release package or rebuild its original pinned source and explicitly migrate its asset manifest: %w", err)
	}
	var manifest assetsManifest
	decoder := json.NewDecoder(strings.NewReader(string(manifestBytes)))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&manifest); err != nil {
		return 0, fmt.Errorf("invalid asset digest manifest: %w", err)
	}
	if decoder.Decode(new(interface{})) != io.EOF {
		return 0, errors.New("asset digest manifest contains trailing JSON")
	}
	if manifest.Protocol != assetsProtocol || manifest.BuildID != buildID || manifest.UpstreamSHA != upstreamSHA || len(manifest.Files) == 0 || len(manifest.Files) > 20000 {
		return 0, errors.New("asset digest manifest identity or file count is invalid")
	}
	if _, exists := manifest.Files["index.html"]; !exists {
		return 0, errors.New("index.html is absent from the asset digest manifest")
	}
	if _, exists := manifest.Files[".noyo-build.json"]; !exists {
		return 0, errors.New("build stamp is absent from the asset digest manifest")
	}
	for filename, digest := range manifest.Files {
		if filename == AssetsManifest || !fs.ValidPath(filename) || strings.Contains(filename, "\\") || !digestPattern.MatchString(digest) {
			return 0, fmt.Errorf("invalid manifest asset path or SHA256: %q", filename)
		}
	}
	count := 0
	err = fs.WalkDir(candidate, directory, func(filename string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.Type()&fs.ModeSymlink != 0 || (!entry.IsDir() && !entry.Type().IsRegular()) {
			return fmt.Errorf("unsafe non-regular asset: %s", filename)
		}
		if entry.IsDir() {
			return nil
		}
		relative := strings.TrimPrefix(filename, directory+"/")
		if relative == AssetsManifest {
			return nil
		}
		wanted, exists := manifest.Files[relative]
		if !exists {
			return fmt.Errorf("asset is not recorded in the digest manifest: %s", relative)
		}
		file, err := candidate.Open(filename)
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, err = io.Copy(hash, file)
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		if hex.EncodeToString(hash.Sum(nil)) != wanted {
			return fmt.Errorf("SHA256 mismatch: %s", relative)
		}
		count++
		return nil
	})
	if err != nil {
		return 0, err
	}
	if count != len(manifest.Files) {
		return 0, fmt.Errorf("missing engine assets: manifest expects %d files, found %d", len(manifest.Files), count)
	}
	index, err := readLimited(candidate, path.Join(directory, "index.html"), 5<<20)
	if err != nil || !strings.Contains(string(index), "/screen-editor/"+buildID+"/") {
		return 0, errors.New("index.html has an invalid engine deployment base")
	}
	stamp, err := readLimited(candidate, path.Join(directory, ".noyo-build.json"), 64<<10)
	var metadata struct {
		BuildID     string `json:"buildId"`
		UpstreamSHA string `json:"upstreamSha"`
		Base        string `json:"base"`
	}
	if err != nil || json.Unmarshal(stamp, &metadata) != nil || metadata.BuildID != buildID || metadata.UpstreamSHA != upstreamSHA || metadata.Base != "/screen-editor/"+buildID+"/" {
		return 0, errors.New("engine build stamp does not match the stored identity")
	}
	return count, nil
}

func readLimited(candidate fs.FS, filename string, limit int64) ([]byte, error) {
	file, err := candidate.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	contents, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err == nil && int64(len(contents)) > limit {
		err = fmt.Errorf("file exceeds %d bytes: %s", limit, filename)
	}
	return contents, err
}
