package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"noyo/core"
	"os"
	"path/filepath"
)

//go:embed dist
var distFS embed.FS

// loadUIFS resolves the embedded production UI first and then supports the
// external dist directory used by unpacked deployments. The latter keeps a
// binary useful when the release process ships the UI beside it instead of
// rebuilding the binary after the frontend build.
func loadUIFS() (fs.FS, string, error) {
	embedded, err := fs.Sub(distFS, "dist")
	if err == nil {
		if _, statErr := fs.Stat(embedded, "index.html"); statErr == nil {
			return embedded, "embedded", nil
		}
	}

	workingDir, _ := os.Getwd()
	executable, _ := os.Executable()
	executableDir := filepath.Dir(executable)
	candidates := []string{
		filepath.Join(workingDir, "dist"),
		filepath.Join(workingDir, "backend", "dist"),
		filepath.Join(executableDir, "dist"),
		filepath.Join(executableDir, "..", "dist"),
	}
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		candidate, cleanErr := filepath.Abs(filepath.Clean(candidate))
		if cleanErr != nil {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		if _, statErr := os.Stat(filepath.Join(candidate, "index.html")); statErr == nil {
			return os.DirFS(candidate), candidate, nil
		}
	}

	return nil, "", fmt.Errorf("UI assets are unavailable: embedded dist/index.html is missing and no external dist directory was found")
}

func main() {
	fmt.Println("STARTING BACKEND...")
	// Create Server
	server, err := core.NewServer()
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Setup UI
	uiFS, source, err := loadUIFS()
	if err != nil {
		log.Println("ERROR: Failed to load UI files:", err)
	} else {
		log.Printf("UI assets loaded from %s", source)
		server.SetUI(uiFS)
	}

	// Run Server
	if err := server.Run(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
