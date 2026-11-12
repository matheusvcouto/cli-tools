package notionexport

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/matheusvcouto/cli-tools/internal/safefs"
)

// ErrDestinationExists reports an existing export directory without --force.
var ErrDestinationExists = errors.New("o diretório de destino já existe")

// ErrForeignDestination reports a directory that --force refuses to replace
// because it is not a previous export of the same page.
var ErrForeignDestination = errors.New("o destino existente não é um export desta página")

const stagingPrefix = ".notion-export-"

func randomSuffix() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// fsOps is the subset of safefs.Root used by publication; tests inject
// failures here to prove rollback.
type fsOps interface {
	Lstat(name string) (os.FileInfo, error)
	Rename(oldname, newname string) error
	RemoveAll(name string) error
	OpenRoot(name string) (*safefs.Root, error)
}

// inspectDestination checks an existing destination without mutating it.
// Missing is fine; an existing entry needs force and must be a real directory
// holding a manifest.json written by this exporter (or its Deno predecessor)
// for the same page ID.
func inspectDestination(root fsOps, name, pageID string, force bool) (os.FileInfo, error) {
	info, err := root.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !force {
		return nil, fmt.Errorf("%w: %s (use --force para substituir um export anterior desta página)", ErrDestinationExists, name)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("%w: %s não é um diretório real", ErrForeignDestination, name)
	}
	dir, err := root.OpenRoot(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrForeignDestination, err)
	}
	defer dir.Close()
	mi, err := dir.Lstat("manifest.json")
	if err != nil || !mi.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %s não contém manifest.json regular", ErrForeignDestination, name)
	}
	f, err := dir.Open("manifest.json")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrForeignDestination, err)
	}
	generator, existingID, err := readManifestIdentity(f)
	_ = f.Close()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrForeignDestination, err)
	}
	if generator != GeneratorName && generator != LegacyGeneratorName {
		return nil, fmt.Errorf("%w: gerado por %q", ErrForeignDestination, generator)
	}
	if CompactID(existingID) != CompactID(pageID) {
		return nil, fmt.Errorf("%w: pertence à página %s", ErrForeignDestination, CompactID(existingID))
	}
	return info, nil
}

// publication moves a complete staging directory into place.
type publication struct {
	root    fsOps
	staging string
	target  string
	pageID  string
	force   bool
}

// publish commits the staging directory. Without force it only renames into
// a missing name: rename(2) never replaces a non-empty directory, and Windows
// refuses any existing target, so no existing content can be lost. With force
// the validated previous export is first moved to a quarantine name inside the
// same root; if installing the new export fails it is moved back. The
// quarantine is deleted only after success, and only when it is still the very
// directory that was validated. cleanupErr reports a leftover quarantine.
func (p publication) publish() (cleanupErr error, err error) {
	previous, err := inspectDestination(p.root, p.target, p.pageID, p.force)
	if err != nil {
		return nil, err
	}
	if previous == nil {
		if err := p.root.Rename(p.staging, p.target); err != nil {
			return nil, fmt.Errorf("publicar export: %w", err)
		}
		return nil, nil
	}
	suffix, err := randomSuffix()
	if err != nil {
		return nil, err
	}
	quarantine := stagingPrefix + p.target + "-" + suffix + ".old"
	if err := p.root.Rename(p.target, quarantine); err != nil {
		return nil, fmt.Errorf("mover export anterior: %w", err)
	}
	moved, err := p.root.Lstat(quarantine)
	if err != nil || !os.SameFile(previous, moved) {
		restoreErr := p.root.Rename(quarantine, p.target)
		return nil, errors.Join(fmt.Errorf("export anterior mudou durante a substituição"), err, restoreErr)
	}
	if err := p.root.Rename(p.staging, p.target); err != nil {
		if restoreErr := p.root.Rename(quarantine, p.target); restoreErr != nil {
			return nil, errors.Join(fmt.Errorf("publicar export: %w", err), fmt.Errorf("restaurar export anterior (%s): %w", quarantine, restoreErr))
		}
		return nil, fmt.Errorf("publicar export (export anterior restaurado): %w", err)
	}
	if current, err := p.root.Lstat(quarantine); err != nil || !os.SameFile(previous, current) {
		return fmt.Errorf("quarentena %s não foi removida: identidade não confirmada", quarantine), nil
	}
	if err := p.root.RemoveAll(quarantine); err != nil {
		return fmt.Errorf("remover export anterior em %s: %w", quarantine, err), nil
	}
	return nil, nil
}
