package aiprofile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/matheusvcouto/cli-tools/internal/filelock"
	"github.com/matheusvcouto/cli-tools/internal/safefs"
)

type Store struct {
	Root string
}

// The profile index and Claude settings are small JSON documents. Read them
// with a hard cap: a corrupt or hostile file must not exhaust the process's
// memory during an ordinary list/run/new operation. The cap applies to the
// package's recovery helper too, so it cannot silently bypass this boundary.
const maxProfileMetadataBytes int64 = 8 << 20

func DefaultRoot() (string, error) {
	if override := strings.TrimSpace(os.Getenv("AI_PROFILE_ROOT")); override != "" {
		return filepath.Abs(override)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home: %w", err)
	}
	return filepath.Join(home, ".ai-profiles"), nil
}

func (s Store) indexPath() string  { return filepath.Join(s.Root, "index.json") }
func (s Store) backupPath() string { return filepath.Join(s.Root, "index.json.bak") }
func (s Store) lockPath() string   { return filepath.Join(s.Root, ".index.lock") }

func (s Store) EnsureRoot() error {
	rootPath, err := safefs.EnsureDir(s.Root, 0o700)
	if err != nil {
		return fmt.Errorf("create profile root safely: %w", err)
	}
	root, err := safefs.Open(rootPath)
	if err != nil {
		return fmt.Errorf("open profile root safely: %w", err)
	}
	defer root.Close()
	info, err := root.Lstat(".")
	if err != nil {
		return fmt.Errorf("inspect profile root: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("profile root must be a real directory: %s", rootPath)
	}
	return secureOpenedProfileRoot(root, rootPath)
}

func secureOpenedProfileRoot(root *safefs.Root, rootPath string) error {
	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return fmt.Errorf("resolve profile root path: %w", err)
	}
	f, err := root.Open(".")
	if err != nil {
		return fmt.Errorf("open profile root directory: %w", err)
	}
	secureErr := secureProfileRoot(f, absRoot)
	closeErr := f.Close()
	if secureErr != nil {
		return fmt.Errorf("secure profile root permissions: %w", secureErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close profile root directory: %w", closeErr)
	}
	return nil
}

func (s Store) Load() (StoreData, error) {
	info, err := os.Lstat(s.Root)
	if errors.Is(err, os.ErrNotExist) {
		return emptyStore(), nil
	}
	if err != nil {
		return StoreData{}, fmt.Errorf("inspect profile root: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return StoreData{}, fmt.Errorf("profile root must be a real directory: %s", s.Root)
	}
	root, err := safefs.Open(s.Root)
	if err != nil {
		return StoreData{}, fmt.Errorf("open profile safety root: %w", err)
	}
	defer root.Close()
	// Loading an existing store also enforces the current platform security
	// policy. This upgrades trees created by older versions before profile data
	// or credentials are consumed, rather than waiting for the next mutation.
	if err := secureOpenedProfileRoot(root, s.Root); err != nil {
		return StoreData{}, err
	}
	return s.loadRootOrEmpty(root)
}

func emptyStore() StoreData {
	return StoreData{SchemaVersion: StoreSchemaVersion, Profiles: []Profile{}}
}

func (s Store) loadRootOrEmpty(root *safefs.Root) (StoreData, error) {
	data, _, err := s.loadRootPathRaw(root, "index.json")
	if errors.Is(err, fs.ErrNotExist) {
		// A missing index is not necessarily an empty store. A previous index,
		// an imported legacy directory, or an interrupted quarantine may leave
		// credentials behind. Never reconstruct an empty index over residual state.
		// Recovery and migration are explicit operations.
		if err := refuseOrphanedProfileState(root); err != nil {
			return StoreData{}, err
		}
		return emptyStore(), nil
	}
	return data, err
}

func refuseOrphanedProfileState(root *safefs.Root) error {
	if _, err := root.Lstat("index.json.bak"); err == nil {
		return fmt.Errorf("profile index is missing while index.json.bak exists; restore the index explicitly before continuing")
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect missing-index backup: %w", err)
	}

	dir, err := root.Open(".")
	if err != nil {
		return fmt.Errorf("inspect missing-index profile root: %w", err)
	}
	defer dir.Close()
	for {
		// Bounded batches avoid loading an arbitrarily large directory into
		// memory. Entries are inspected even if their type cannot be read.
		entries, readErr := dir.ReadDir(128)
		for _, entry := range entries {
			// The lock can legitimately predate the first index: a failed first
			// transaction creates it. Every other entry is ambiguous without an
			// index. In particular, legacy NUON migrations allow arbitrary leaf
			// directory names, and failed deletion may leave .deleted-* behind.
			if isStoreLockEntry(entry.Name()) {
				info, err := root.Lstat(entry.Name())
				if err != nil {
					return fmt.Errorf("inspect existing index lock: %w", err)
				}
				if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
					return fmt.Errorf("existing index lock is not a regular file")
				}
				continue
			}
			if looksLikeManagedProfileDir(entry.Name()) {
				return fmt.Errorf("profile index is missing but managed profile state %q remains; recover the index explicitly", entry.Name())
			}
			return fmt.Errorf("profile index is missing but residual entry %q remains; inspect and recover the index explicitly", entry.Name())
		}
		if errors.Is(readErr, io.EOF) {
			return nil
		}
		if readErr != nil {
			return fmt.Errorf("inspect missing-index profile entries: %w", readErr)
		}
	}
}

// The lock filename uses the same platform filename matching as profile
// directory identity. On Windows an existing .INDEX.LOCK is the same leaf as
// .index.lock, even when ReadDir preserves its original casing.
func isStoreLockEntry(name string) bool {
	return profileDirIdentity(name) == profileDirIdentity(".index.lock")
}

func looksLikeManagedProfileDir(name string) bool {
	for _, tool := range Tools {
		prefix := tool.Name + "-"
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		// The allocator creates TOOL-YYYYMMDDhhmmss-8-hex filenames.
		suffix := strings.TrimPrefix(name, prefix)
		if len(suffix) != 23 || suffix[14] != '-' {
			continue
		}
		valid := true
		for i, char := range suffix {
			switch {
			case i < 14 && (char < '0' || char > '9'):
				valid = false
			case i > 14 && !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')):
				valid = false
			}
		}
		if valid {
			return true
		}
	}
	return false
}

// loadPath is kept as a package-level test/recovery helper. Production store
// operations keep an already-open safety root across read-modify-write.
func (s Store) loadPath(path string) (StoreData, error) {
	if filepath.Clean(filepath.Dir(path)) == filepath.Clean(s.Root) {
		root, err := safefs.Open(s.Root)
		if err != nil {
			return StoreData{}, err
		}
		defer root.Close()
		data, _, err := s.loadRootPathRaw(root, filepath.Base(path))
		return data, err
	}
	raw, err := readStableRegularFile(path)
	if err != nil {
		return StoreData{}, err
	}
	return s.decode(raw, filepath.Base(path))
}

func (s Store) loadRootPathRaw(root *safefs.Root, name string) (StoreData, []byte, error) {
	raw, err := readStableRegularRoot(root, name)
	if err != nil {
		return StoreData{}, nil, err
	}
	data, err := s.decode(raw, name)
	if err != nil {
		return StoreData{}, nil, err
	}
	return data, raw, nil
}

func (s Store) decode(raw []byte, label string) (StoreData, error) {
	var data StoreData
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&data); err != nil {
		return StoreData{}, fmt.Errorf("decode %s: %w", label, err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return StoreData{}, fmt.Errorf("decode %s: trailing JSON data", label)
		}
		return StoreData{}, fmt.Errorf("decode %s: trailing data: %w", label, err)
	}
	if err := s.validate(data); err != nil {
		return StoreData{}, fmt.Errorf("validate %s: %w", label, err)
	}
	return data, nil
}

func readStableRegularRoot(root *safefs.Root, name string) ([]byte, error) {
	before, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, fmt.Errorf("refuse non-regular or symbolic-link file: %s", name)
	}
	if before.Size() > maxProfileMetadataBytes {
		return nil, fmt.Errorf("profile metadata file %s exceeds %d bytes", name, maxProfileMetadataBytes)
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil {
		return nil, err
	}
	after, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if after.Mode()&os.ModeSymlink != 0 || !after.Mode().IsRegular() || !os.SameFile(before, opened) || !os.SameFile(opened, after) {
		return nil, fmt.Errorf("file changed identity while opening: %s", name)
	}
	return readBoundedProfileMetadata(f, name)
}

func readStableRegularFile(path string) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, fmt.Errorf("refuse non-regular or symbolic-link file: %s", path)
	}
	if before.Size() > maxProfileMetadataBytes {
		return nil, fmt.Errorf("profile metadata file %s exceeds %d bytes", path, maxProfileMetadataBytes)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil {
		return nil, err
	}
	after, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if after.Mode()&os.ModeSymlink != 0 || !after.Mode().IsRegular() || !os.SameFile(before, opened) || !os.SameFile(opened, after) {
		return nil, fmt.Errorf("file changed identity while opening: %s", path)
	}
	return readBoundedProfileMetadata(f, path)
}

func readBoundedProfileMetadata(f *os.File, name string) ([]byte, error) {
	// A second check during reading closes the gap where another process grows
	// the file after our initial Lstat. Never decode a truncated JSON document.
	raw, err := io.ReadAll(io.LimitReader(f, maxProfileMetadataBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read profile metadata %s: %w", name, err)
	}
	if int64(len(raw)) > maxProfileMetadataBytes {
		return nil, fmt.Errorf("profile metadata file %s exceeds %d bytes", name, maxProfileMetadataBytes)
	}
	return raw, nil
}

func (s Store) validate(data StoreData) error {
	if data.SchemaVersion != StoreSchemaVersion {
		return fmt.Errorf("unsupported schema_version %d (expected %d)", data.SchemaVersion, StoreSchemaVersion)
	}
	seen := make(map[string]struct{}, len(data.Profiles))
	seenDirs := make(map[string]string, len(data.Profiles))
	for i, p := range data.Profiles {
		if _, ok := LookupTool(p.Tool); !ok {
			return fmt.Errorf("profile %d references unknown tool %q", i, p.Tool)
		}
		if err := ValidateAlias(p.Alias); err != nil {
			return fmt.Errorf("profile %d: %w", i, err)
		}
		if strings.TrimSpace(p.CreatedAt) == "" {
			return fmt.Errorf("profile %d has empty created_at", i)
		}
		if err := s.validateProfileDir(p.Dir); err != nil {
			return fmt.Errorf("profile %d: %w", i, err)
		}
		// Two aliases must never refer to the same on-disk directory: deleting
		// either alias would otherwise erase the other's credentials/sessions.
		// Windows paths are case-insensitive even though JSON strings are not.
		dirKey := profileDirIdentity(p.Dir)
		if owner, exists := seenDirs[dirKey]; exists {
			return fmt.Errorf("profile %s/%s reuses directory of %s", p.Tool, p.Alias, owner)
		}
		seenDirs[dirKey] = p.Tool + "/" + p.Alias
		key := p.Tool + "\x00" + p.Alias
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate profile %s/%s", p.Tool, p.Alias)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func (s Store) validateProfileDir(dir string) error {
	if !filepath.IsAbs(dir) {
		return fmt.Errorf("profile dir must be absolute: %q", dir)
	}
	root, err := filepath.Abs(s.Root)
	if err != nil {
		return err
	}
	clean := filepath.Clean(dir)
	rel, err := filepath.Rel(root, clean)
	if err != nil {
		return err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("profile dir escapes profile root: %q", dir)
	}
	if filepath.Dir(rel) != "." {
		return fmt.Errorf("profile dir must be an immediate child of profile root: %q", dir)
	}
	return nil
}

func (s Store) openLockedRoot() (*safefs.Root, filelock.Lock, error) {
	if err := s.EnsureRoot(); err != nil {
		return nil, nil, err
	}
	root, err := safefs.Open(s.Root)
	if err != nil {
		return nil, nil, fmt.Errorf("open profile safety root: %w", err)
	}
	fail := func(err error) (*safefs.Root, filelock.Lock, error) {
		_ = root.Close()
		return nil, nil, err
	}

	f, err := root.OpenFile(".index.lock", os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if errors.Is(err, os.ErrExist) {
		before, inspectErr := root.Lstat(".index.lock")
		if inspectErr != nil {
			return fail(fmt.Errorf("inspect existing lock path: %w", inspectErr))
		}
		if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
			return fail(fmt.Errorf("lock path is not a regular file"))
		}
		f, err = root.OpenFile(".index.lock", os.O_RDWR, 0o600)
	}
	if err != nil {
		return fail(fmt.Errorf("open profile store lock: %w", err))
	}
	opened, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return fail(fmt.Errorf("inspect opened lock: %w", err))
	}
	after, err := root.Lstat(".index.lock")
	if err != nil || after.Mode()&os.ModeSymlink != 0 || !after.Mode().IsRegular() || !os.SameFile(opened, after) {
		_ = f.Close()
		if err != nil {
			return fail(fmt.Errorf("reinspect lock path: %w", err))
		}
		return fail(fmt.Errorf("lock path changed identity or became unsafe"))
	}
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return fail(fmt.Errorf("secure lock permissions: %w", err))
	}
	lock, err := filelock.AcquireFile(f)
	if err != nil {
		return fail(fmt.Errorf("acquire profile store lock: %w", err))
	}
	return root, lock, nil
}

func (s Store) createProfile(tool, alias string, now time.Time, initialize func(*safefs.Root, string, string) error) (Profile, error) {
	root, lock, err := s.openLockedRoot()
	if err != nil {
		return Profile{}, err
	}
	defer root.Close()
	defer lock.Close()

	current, err := s.loadRootOrEmpty(root)
	if err != nil {
		return Profile{}, fmt.Errorf("load profile store: %w", err)
	}
	if _, exists := FindProfile(current, tool, alias); exists {
		return Profile{}, fmt.Errorf("profile %s/%s already exists", tool, alias)
	}

	absRoot, err := filepath.Abs(s.Root)
	if err != nil {
		return Profile{}, fmt.Errorf("resolve profile root: %w", err)
	}
	now = now.UTC().Truncate(time.Second)

	var dirName string
	for i := 0; i < 8; i++ {
		suffix, err := randomHex(4)
		if err != nil {
			return Profile{}, err
		}
		candidate := fmt.Sprintf("%s-%s-%s", tool, now.Format("20060102150405"), suffix)
		if err := root.Mkdir(candidate, 0o700); err == nil {
			dirName = candidate
			break
		} else if !errors.Is(err, os.ErrExist) {
			return Profile{}, fmt.Errorf("create profile directory: %w", err)
		}
	}
	if dirName == "" {
		return Profile{}, fmt.Errorf("could not allocate a unique profile directory")
	}

	profile := Profile{
		Tool:      tool,
		Alias:     alias,
		Dir:       filepath.Join(absRoot, dirName),
		CreatedAt: now.Format(time.RFC3339),
	}
	fail := func(cause error) (Profile, error) {
		if cleanupErr := root.RemoveAll(dirName); cleanupErr != nil {
			return Profile{}, fmt.Errorf("%v; rollback of newly created profile directory %q failed: %w", cause, dirName, cleanupErr)
		}
		return Profile{}, cause
	}

	// Provider initialization is performed while holding the store lock and
	// before committing index.json. A failure rolls back the entire new
	// directory; clients cannot observe a published but uninitialized profile.
	if initialize != nil {
		if err := initialize(root, dirName, profile.Dir); err != nil {
			return fail(fmt.Errorf("initialize %s profile: %w", tool, err))
		}
	}
	current.Profiles = append(current.Profiles, profile)
	if err := s.validate(current); err != nil {
		return fail(fmt.Errorf("refuse invalid profile store update: %w", err))
	}
	sortProfiles(current.Profiles)
	if err := s.commitRoot(root, current); err != nil {
		return fail(err)
	}
	return profile, nil
}

func sortProfiles(profiles []Profile) {
	sort.SliceStable(profiles, func(i, j int) bool {
		if profiles[i].Tool == profiles[j].Tool {
			return profiles[i].Alias < profiles[j].Alias
		}
		return profiles[i].Tool < profiles[j].Tool
	})
}

func (s Store) Update(fn func(*StoreData) error) error {
	root, lock, err := s.openLockedRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	defer lock.Close()

	current, err := s.loadRootOrEmpty(root)
	if err != nil {
		return fmt.Errorf("load profile store: %w", err)
	}
	if err := fn(&current); err != nil {
		return err
	}
	if err := s.validate(current); err != nil {
		return fmt.Errorf("refuse invalid profile store update: %w", err)
	}
	sortProfiles(current.Profiles)
	return s.commitRoot(root, current)
}

// ImportLegacy is the only supported way to adopt pre-existing profile
// directories when no JSON index exists. Unlike Update, this intentionally
// handles an occupied root, but refuses missing, unexpected, non-directory,
// or aliased paths. It does not repair corrupt or missing indexes silently.
// The source migration file is not modified. If it resides inside the
// destination root (the original Nushell layout), it is an explicitly
// permitted regular file only after confirming its contents did not change
// between parsing and the locked commit.
func (s Store) ImportLegacy(profiles []Profile, sourcePath string, sourceContents []byte) error {
	root, lock, err := s.openLockedRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	defer lock.Close()

	for _, name := range []string{"index.json", "index.json.bak"} {
		if _, err := root.Lstat(name); err == nil {
			return fmt.Errorf("refuse legacy import: %s already exists", name)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("inspect legacy import destination %s: %w", name, err)
		}
	}

	data := emptyStore()
	data.Profiles = append(data.Profiles, profiles...)
	if err := s.validate(data); err != nil {
		return fmt.Errorf("invalid legacy profile index: %w", err)
	}

	// The legacy index itself normally lives in ~/.ai-profiles/index.nuon.
	// Allow exactly the source file explicitly read by the migration tool, not
	// arbitrary unknown root files, and reject concurrent source replacement.
	var sourceName string
	if sourcePath != "" {
		absSource, err := filepath.Abs(sourcePath)
		if err != nil {
			return fmt.Errorf("resolve legacy source: %w", err)
		}
		absRoot, err := filepath.Abs(s.Root)
		if err != nil {
			return fmt.Errorf("resolve legacy destination: %w", err)
		}
		var actual []byte
		if profileDirIdentity(filepath.Dir(absSource)) == profileDirIdentity(absRoot) {
			sourceName = filepath.Base(absSource)
			if profileDirIdentity(sourceName) == profileDirIdentity("index.json") ||
				profileDirIdentity(sourceName) == profileDirIdentity("index.json.bak") || isStoreLockEntry(sourceName) {
				return fmt.Errorf("legacy import source %q conflicts with the store's own files", sourceName)
			}
			actual, err = readStableRegularRoot(root, sourceName)
		} else {
			// External source files can also change while the user approves an
			// explicit import. Re-read them instead of committing stale aliases.
			actual, err = readStableRegularFile(absSource)
		}
		if err != nil {
			return fmt.Errorf("verify legacy index before commit: %w", err)
		}
		if !bytes.Equal(actual, sourceContents) {
			return fmt.Errorf("legacy source %q changed since parsing; refusing stale import", absSource)
		}
	}

	// All other entries in an explicit legacy import must correspond to the
	// supplied profiles. Do not silently absorb orphaned credentials, failed
	// temporary writes, or an interrupted deletion into a new index.
	expected := make(map[string]struct{}, len(profiles))
	for _, profile := range profiles {
		name := filepath.Base(profile.Dir)
		info, err := root.Lstat(name)
		if err != nil {
			return fmt.Errorf("inspect legacy profile %s/%s directory: %w", profile.Tool, profile.Alias, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("legacy profile %s/%s is not a real directory", profile.Tool, profile.Alias)
		}
		expected[profileDirIdentity(name)] = struct{}{}
	}

	dir, err := root.Open(".")
	if err != nil {
		return fmt.Errorf("inspect legacy import root: %w", err)
	}
	for {
		entries, readErr := dir.ReadDir(128)
		for _, entry := range entries {
			if isStoreLockEntry(entry.Name()) || (sourceName != "" && profileDirIdentity(entry.Name()) == profileDirIdentity(sourceName)) {
				continue
			}
			if _, ok := expected[profileDirIdentity(entry.Name())]; !ok {
				_ = dir.Close()
				return fmt.Errorf("refuse legacy import: unreferenced root entry %q", entry.Name())
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			_ = dir.Close()
			return fmt.Errorf("inspect legacy import entries: %w", readErr)
		}
	}
	if err := dir.Close(); err != nil {
		return fmt.Errorf("close legacy import directory: %w", err)
	}

	sortProfiles(data.Profiles)
	return s.commitRoot(root, data)
}

func (s Store) commitRoot(root *safefs.Root, data StoreData) error {
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode profile store: %w", err)
	}
	raw = append(raw, '\n')
	// A successful commit must produce an index the very next Load can read.
	// Reject an oversized update before touching index.json.bak or index.json.
	if int64(len(raw)) > maxProfileMetadataBytes {
		return fmt.Errorf("encoded profile index exceeds %d bytes", maxProfileMetadataBytes)
	}

	if _, current, err := s.loadRootPathRaw(root, "index.json"); err == nil {
		if err := s.writeCommittedFileRoot(root, "index.json.bak", current); err != nil {
			return fmt.Errorf("write profile store backup: %w", err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("refuse to overwrite invalid existing store: %w", err)
	}

	if err := s.writeCommittedFileRoot(root, "index.json", raw); err != nil {
		return fmt.Errorf("write profile store: %w", err)
	}
	return nil
}

func (s Store) writeCommittedFileRoot(root *safefs.Root, dst string, content []byte) error {
	return writeBoundedProfileMetadataRoot(root, dst, content, 0o600, ".index-")
}

// This write boundary mirrors the read boundary. Callers must not commit a
// metadata document that would become unreadable on their next operation.
func writeBoundedProfileMetadataRoot(root *safefs.Root, dst string, content []byte, perm os.FileMode, prefix string) error {
	if int64(len(content)) > maxProfileMetadataBytes {
		return fmt.Errorf("profile metadata file %s exceeds %d bytes", dst, maxProfileMetadataBytes)
	}
	return writeAtomicRoot(root, dst, content, perm, prefix)
}

func (s Store) BackupValid() bool {
	root, err := safefs.Open(s.Root)
	if err != nil {
		return false
	}
	defer root.Close()
	_, _, err = s.loadRootPathRaw(root, "index.json.bak")
	return err == nil
}

func (s Store) deleteProfile(tool, alias string) (string, error) {
	return s.deleteProfileIfUnchanged(tool, alias, nil)
}

// deleteProfileIfUnchanged protects the interactive confirmation against a
// second ai-profile process renaming/deleting the displayed profile and
// reusing its alias before confirmation completes. The identity check must
// happen under the same lock as the subsequent quarantine and index commit.
func (s Store) deleteProfileIfUnchanged(tool, alias string, expected *Profile) (string, error) {
	root, lock, err := s.openLockedRoot()
	if err != nil {
		return "", err
	}
	defer root.Close()
	defer lock.Close()

	data, err := s.loadRootOrEmpty(root)
	if err != nil {
		return "", fmt.Errorf("load profile store: %w", err)
	}
	profile, ok := FindProfile(data, tool, alias)
	if !ok {
		return "", fmt.Errorf("profile %s/%s does not exist", tool, alias)
	}
	if expected != nil && profile != *expected {
		return "", fmt.Errorf("profile %s/%s changed after deletion confirmation; inspect and confirm again", tool, alias)
	}
	if err := s.validateProfileDir(profile.Dir); err != nil {
		return "", err
	}
	profileName := filepath.Base(profile.Dir)
	info, err := root.Lstat(profileName)
	if err != nil {
		return "", fmt.Errorf("inspect profile directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", fmt.Errorf("refuse to delete profile path that is not a real directory: %s", profile.Dir)
	}

	suffix, err := randomHex(6)
	if err != nil {
		return "", err
	}
	quarantineName := ".deleted-" + suffix
	if err := root.Rename(profileName, quarantineName); err != nil {
		return "", fmt.Errorf("quarantine profile directory: %w", err)
	}
	rolledBack := false
	rollback := func() error {
		if rolledBack {
			return nil
		}
		rolledBack = true
		return root.Rename(quarantineName, profileName)
	}

	filtered := data.Profiles[:0]
	for _, p := range data.Profiles {
		if p.Tool == tool && p.Alias == alias {
			continue
		}
		filtered = append(filtered, p)
	}
	data.Profiles = filtered
	if err := s.commitRoot(root, data); err != nil {
		if rbErr := rollback(); rbErr != nil {
			return "", fmt.Errorf("store update failed (%v) and rollback failed (%v)", err, rbErr)
		}
		return "", err
	}
	rolledBack = true
	if err := root.RemoveAll(quarantineName); err != nil {
		return profile.Dir, fmt.Errorf("profile removed from index; cleanup of quarantined directory %q failed: %w", quarantineName, err)
	}
	return profile.Dir, nil
}
