package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"noyo/core"
)

//go:embed dist
var distFS embed.FS

func main() {
	fmt.Println("STARTING BACKEND...")
	// Create Server
	server, err := core.NewServer()
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Setup UI
	uiFS, err := fs.Sub(distFS, "dist")
	if err != nil {
		log.Println("Warning: Failed to load UI files:", err)
	} else {
		server.SetUI(uiFS)
	}

	// Run Server
	if err := server.Run(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
