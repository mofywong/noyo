package screenpreflight

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Execute returns an exit code and performs no application initialization.
// embedded must be the release's dist root, including dotfiles (all:dist).
func Execute(args []string, embedded fs.FS, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("screen-preflight", flag.ContinueOnError)
	flags.SetOutput(stderr)
	database := flags.String("database", "", "existing SQLite database file (opened mode=ro)")
	dist := flags.String("dist", "", "candidate dist directory; defaults to this binary's embedded UI")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *database == "" || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "Usage: noyo screen-preflight --database <sqlite> [--dist <candidate dist>]")
		return 2
	}
	candidate := embedded
	if *dist != "" {
		absolute, err := filepath.Abs(*dist)
		if err != nil {
			fmt.Fprintln(stderr, "screen-preflight:", err)
			return 1
		}
		info, err := os.Lstat(absolute)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			fmt.Fprintln(stderr, "screen-preflight: --dist must be an existing directory without a symlink root")
			return 1
		}
		candidate = os.DirFS(absolute)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	report, err := Run(ctx, *database, candidate)
	if err != nil {
		fmt.Fprintln(stderr, "screen-preflight: BLOCKED:", err)
		return 1
	}
	if err = json.NewEncoder(stdout).Encode(report); err != nil {
		fmt.Fprintln(stderr, "screen-preflight: write report:", err)
		return 1
	}
	return 0
}
