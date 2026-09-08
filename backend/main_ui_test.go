package main

import (
	"io/fs"
	"testing"
)

func TestLoadUIFSProvidesIndex(t *testing.T) {
	ui, source, err := loadUIFS()
	if err != nil {
		t.Fatalf("loadUIFS() error = %v", err)
	}
	if source == "" {
		t.Fatal("loadUIFS() returned an empty source")
	}
	if info, err := fs.Stat(ui, "index.html"); err != nil || info.Size() == 0 {
		t.Fatalf("index.html is unavailable or empty: info=%v err=%v", info, err)
	}
}
