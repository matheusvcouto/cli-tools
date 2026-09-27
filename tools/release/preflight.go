package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// release preflight checks the exact release inputs without writing artifacts,
// altering version files, or launching any platform build. Run it before the
// expensive native test matrix on tag pushes. The full builder repeats its
// own checks; this is an early rejection boundary, not a replacement gate.
func runPreflightCommand(args []string) {
	fs := flag.NewFlagSet("preflight", flag.ExitOnError)
	version := fs.String("version", "", "release tag (vX.Y.Z)")
	changelog := fs.String("changelog", "CHANGELOG.md", "versioned changelog")
	if err := fs.Parse(args); err != nil {
		fatalf("parse release preflight options: %v", err)
	}
	if fs.NArg() != 0 {
		fatalf("unexpected release preflight arguments: %v", fs.Args())
	}
	if _, err := releaseNotesFromFile(*changelog, *version); err != nil {
		fatalf("release notes: %v", err)
	}
	parsed, err := parseReleaseSemver(*version)
	if err != nil {
		fatalf("release version: %v", err)
	}
	if err := requireReleaseToolchain(); err != nil {
		fatalf("toolchain: %v", err)
	}
	module, err := modulePath()
	if err != nil {
		fatalf("module path: %v", err)
	}
	if err := validateReleaseModulePath(module, parsed); err != nil {
		fatalf("release module version: %v", err)
	}
	bins, err := discoverCommands("cmd")
	if err != nil {
		fatalf("discover release commands: %v", err)
	}
	if len(bins) == 0 {
		fatalf("no release commands found under cmd/")
	}
	if err := rejectPendingReleaseChanges("changes"); err != nil {
		fatalf("release preparation: %v", err)
	}
	fmt.Printf("preflight PASS: %s, module %s, %d commands, no pending change records\n", *version, module, len(bins))
}

// A tag must point to a prepared commit, not a working tree that still has
// change records. In particular, do not ignore invalid JSON records: the
// changes validate command runs separately and is also a required preflight
// step in GitHub Actions.
func rejectPendingReleaseChanges(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("inspect change records: %w", err)
	}
	var pending []string
	for _, entry := range entries {
		// A symlink or directory named *.json is not a prepared release.
		// Reject its name rather than silently treating it as no pending changes.
		if strings.HasSuffix(entry.Name(), ".json") {
			pending = append(pending, filepath.Join(dir, entry.Name()))
		}
	}
	if len(pending) > 0 {
		return fmt.Errorf("pending change records remain: %s; run release prepare --write and commit the result before tagging", strings.Join(pending, ", "))
	}
	return nil
}
