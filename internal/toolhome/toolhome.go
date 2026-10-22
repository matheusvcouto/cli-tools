// Package toolhome resolves the per-tool directory shared by the suite:
// <home>/.cli-tools/<tool>/, or $CLI_TOOLS_HOME/<tool>/ when overridden.
//
// Each tool owns exactly one directory, named after its tool.json manifest,
// with a fixed layout (config.toml, state/, cache/, backups/). Resolution is
// pure and never touches the filesystem; Ensure* creates on demand without
// following symbolic links and never overwrites existing files.
package toolhome

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/matheusvcouto/cli-tools/internal/safefs"
)

const (
	// EnvRoot overrides the suite root. It must be an absolute path.
	EnvRoot = "CLI_TOOLS_HOME"
	// DirName is the suite root created under the user's home directory.
	DirName = ".cli-tools"

	configFile = "config.toml"
	dirPerm    = 0o700
	filePerm   = 0o600
)

var toolNamePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Lookup supplies the process facts resolution depends on, so tests never
// read the real environment or home directory.
type Lookup struct {
	LookupEnv   func(string) (string, bool)
	UserHomeDir func() (string, error)
}

// OSLookup reads the real process environment and home directory.
func OSLookup() Lookup {
	return Lookup{LookupEnv: os.LookupEnv, UserHomeDir: os.UserHomeDir}
}

// Home is the resolved directory of one tool.
type Home struct {
	tool string
	dir  string
}

// Resolve returns the directory of tool without creating anything.
// An empty or blank CLI_TOOLS_HOME is treated as unset; a relative one is an
// error rather than a silent fallback to the default location.
func Resolve(tool string, lookup Lookup) (Home, error) {
	if !toolNamePattern.MatchString(tool) {
		return Home{}, fmt.Errorf("invalid tool name %q: use lowercase letters, digits and single hyphens", tool)
	}
	root, err := suiteRoot(lookup)
	if err != nil {
		return Home{}, err
	}
	return Home{tool: tool, dir: filepath.Join(root, tool)}, nil
}

func suiteRoot(lookup Lookup) (string, error) {
	if lookup.LookupEnv != nil {
		if value, ok := lookup.LookupEnv(EnvRoot); ok {
			if value = strings.TrimSpace(value); value != "" {
				if !filepath.IsAbs(value) {
					return "", fmt.Errorf("%s must be an absolute path, got %q", EnvRoot, value)
				}
				return filepath.Clean(value), nil
			}
		}
	}
	if lookup.UserHomeDir == nil {
		return "", errors.New("home directory lookup is not configured")
	}
	home, err := lookup.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	if !filepath.IsAbs(home) {
		return "", fmt.Errorf("home directory must be absolute, got %q", home)
	}
	return filepath.Join(home, DirName), nil
}

// Tool is the tool name the directory belongs to.
func (h Home) Tool() string { return h.tool }

// Dir is <suite root>/<tool>.
func (h Home) Dir() string { return h.dir }

// ConfigFile is <dir>/config.toml.
func (h Home) ConfigFile() string { return filepath.Join(h.dir, configFile) }

// StateDir holds durable machine state (indexes, records). Not user-edited.
func (h Home) StateDir() string { return filepath.Join(h.dir, "state") }

// CacheDir holds data that can be deleted and regenerated at any time.
func (h Home) CacheDir() string { return filepath.Join(h.dir, "cache") }

// BackupsDir holds copies kept before destructive replacements.
func (h Home) BackupsDir() string { return filepath.Join(h.dir, "backups") }

// Ensure creates <dir>/<sub> (or <dir> itself when sub is empty) with private
// permissions and returns its path. sub must be a local relative path; no
// component may be a symbolic link.
func (h Home) Ensure(sub string) (string, error) {
	if h.dir == "" {
		return "", errors.New("toolhome: zero Home; use Resolve")
	}
	target := h.dir
	if sub != "" {
		if !filepath.IsLocal(sub) {
			return "", fmt.Errorf("toolhome: %q is not a local relative path", sub)
		}
		target = filepath.Join(h.dir, sub)
	}
	path, err := safefs.EnsureDir(target, dirPerm)
	if err != nil {
		return "", fmt.Errorf("create %s directory: %w", h.tool, err)
	}
	return path, nil
}

// EnsureConfig creates config.toml with defaults when it does not exist and
// reports whether it did. An existing regular file is never rewritten; any
// other kind of object at that path is an error.
func (h Home) EnsureConfig(defaults []byte) (bool, error) {
	dir, err := h.Ensure("")
	if err != nil {
		return false, err
	}
	root, err := safefs.Open(dir)
	if err != nil {
		return false, fmt.Errorf("open %s directory: %w", h.tool, err)
	}
	defer root.Close()

	f, err := root.OpenFile(configFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, filePerm)
	if errors.Is(err, fs.ErrExist) {
		info, statErr := root.Lstat(configFile)
		if statErr != nil {
			return false, statErr
		}
		if !info.Mode().IsRegular() {
			return false, fmt.Errorf("%s exists but is not a regular file", h.ConfigFile())
		}
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("create %s: %w", h.ConfigFile(), err)
	}
	_, writeErr := f.Write(defaults)
	if writeErr == nil {
		writeErr = f.Sync()
	}
	closeErr := f.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		// The file was created by this call, so removing it cannot discard user data.
		_ = root.Remove(configFile)
		return false, fmt.Errorf("write %s: %w", h.ConfigFile(), err)
	}
	return true, nil
}
