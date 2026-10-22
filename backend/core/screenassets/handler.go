// Package screenassets serves versioned editor assets without a SPA fallback.
package screenassets

import (
	"io"
	"io/fs"
	"net/http"
	"strings"
)

const Prefix = "/screen-editor/"

// Handler deliberately shares the application's embedded FS, but never its
// index.html fallback: a missing module must remain a 404 instead of HTML.
func Handler(files fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !strings.HasPrefix(r.URL.Path, Prefix) || strings.ContainsAny(r.URL.Path, "\\\x00") {
			http.NotFound(w, r)
			return
		}
		name := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/"), "/")
		if !fs.ValidPath(name) {
			http.NotFound(w, r)
			return
		}
		// GoFrame strips trailing slashes before invoking this handler.
		// Resolve real directories explicitly; never fall back for missing files.
		if info, err := fs.Stat(files, name); err == nil && info.IsDir() {
			name += "/index.html"
		}
		file, err := files.Open(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}
		reader, ok := file.(io.ReadSeeker)
		if !ok {
			http.Error(w, "Asset unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if strings.HasSuffix(name, ".html") || name == "screen-editor/manifest.json" {
			w.Header().Set("Cache-Control", "no-cache")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		http.ServeContent(w, r, name, info.ModTime(), reader)
	})
}
