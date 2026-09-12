package aiprofile

import (
	"bufio"
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/matheusvcouto/cli-tools/internal/safefs"
)

//go:embed templates/*.json
var builtinTemplates embed.FS

type ProcessIO struct {
	In  *os.File
	Out *os.File
	Err *os.File
}

type ProcessRunner interface {
	Replace(ctx context.Context, binary string, args []string, env []string, io ProcessIO) error
}

type Service struct {
	Store       Store
	Runner      ProcessRunner
	HomeDir     string
	TemplateDir string
	Now         func() time.Time
	Env         func() []string
}

func NewService(store Store, runner ProcessRunner) (*Service, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return &Service{Store: store, Runner: runner, HomeDir: home, TemplateDir: os.Getenv("AI_PROFILE_TEMPLATES_DIR"), Now: time.Now, Env: os.Environ}, nil
}

func (s *Service) List(tool string) ([]Profile, ToolSpec, error) {
	spec, ok := LookupTool(tool)
	if !ok {
		return nil, ToolSpec{}, fmt.Errorf("unknown tool %q", tool)
	}
	data, err := s.Store.Load()
	if err != nil {
		return nil, ToolSpec{}, fmt.Errorf("load profiles: %w", err)
	}
	return ProfilesForTool(data, tool), spec, nil
}

func (s *Service) Create(tool, alias string) (Profile, error) {
	if err := ValidateAlias(alias); err != nil {
		return Profile{}, err
	}
	if _, ok := LookupTool(tool); !ok {
		return Profile{}, fmt.Errorf("unknown tool %q", tool)
	}
	return s.Store.createProfile(tool, alias, s.Now())
}

func (s *Service) Rename(tool, oldAlias, newAlias string) (Profile, error) {
	if err := ValidateAlias(oldAlias); err != nil {
		return Profile{}, err
	}
	if err := ValidateAlias(newAlias); err != nil {
		return Profile{}, err
	}
	if _, ok := LookupTool(tool); !ok {
		return Profile{}, fmt.Errorf("unknown tool %q", tool)
	}
	var updated Profile
	err := s.Store.Update(func(data *StoreData) error {
		if _, exists := FindProfile(*data, tool, newAlias); exists {
			return fmt.Errorf("profile %s/%s already exists", tool, newAlias)
		}
		for i := range data.Profiles {
			if data.Profiles[i].Tool == tool && data.Profiles[i].Alias == oldAlias {
				data.Profiles[i].Alias = newAlias
				updated = data.Profiles[i]
				return nil
			}
		}
		return fmt.Errorf("profile %s/%s does not exist", tool, oldAlias)
	})
	return updated, err
}

func (s *Service) Profile(tool, alias string) (Profile, ToolSpec, error) {
	if err := ValidateAlias(alias); err != nil {
		return Profile{}, ToolSpec{}, err
	}
	spec, ok := LookupTool(tool)
	if !ok {
		return Profile{}, ToolSpec{}, fmt.Errorf("unknown tool %q", tool)
	}
	data, err := s.Store.Load()
	if err != nil {
		return Profile{}, ToolSpec{}, err
	}
	profile, exists := FindProfile(data, tool, alias)
	if !exists {
		return Profile{}, ToolSpec{}, fmt.Errorf("profile %s/%s does not exist (available: %s)", tool, alias, AvailableAliases(data, tool))
	}
	return profile, spec, nil
}

func (s *Service) ensureProfileDirectory(profile Profile) error {
	if err := s.Store.validateProfileDir(profile.Dir); err != nil {
		return err
	}
	info, err := os.Lstat(profile.Dir)
	if err != nil {
		return fmt.Errorf("inspect profile directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("profile path is not a real directory: %s", profile.Dir)
	}
	return nil
}

func (s *Service) DeleteConfirmed(tool, alias string) (string, error) {
	if err := ValidateAlias(alias); err != nil {
		return "", err
	}
	if _, ok := LookupTool(tool); !ok {
		return "", fmt.Errorf("unknown tool %q", tool)
	}
	return s.Store.deleteProfile(tool, alias)
}

func (s *Service) ConfirmDelete(in io.Reader, out io.Writer, profile Profile) error {
	reader := bufio.NewReader(in)
	fmt.Fprintf(out, "This permanently deletes the profile directory:\n  %s\n", profile.Dir)
	fmt.Fprintf(out, "Type %q to confirm: ", profile.Alias)
	typed, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if strings.TrimSpace(typed) != profile.Alias {
		return fmt.Errorf("confirmation did not match profile name")
	}
	fmt.Fprint(out, "Delete permanently? [y/N]: ")
	answer, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return nil
	default:
		return fmt.Errorf("deletion cancelled")
	}
}

func (s *Service) Run(ctx context.Context, tool, alias string, args []string, io ProcessIO) error {
	profile, spec, err := s.Profile(tool, alias)
	if err != nil {
		return err
	}
	if err := s.ensureProfileDirectory(profile); err != nil {
		return err
	}
	if err := s.prepareProfile(tool, profile.Dir); err != nil {
		return err
	}
	env := isolatedEnv(s.Env(), spec.ConfigEnv, profile.Dir, spec.ClearEnv)
	return s.Runner.Replace(ctx, spec.Binary, args, env, io)
}

func (s *Service) ACP(ctx context.Context, tool, alias string, args []string, io ProcessIO) error {
	profile, spec, err := s.Profile(tool, alias)
	if err != nil {
		return err
	}
	if spec.ACP == nil {
		return fmt.Errorf("tool %s has no ACP adapter configured", tool)
	}
	if err := s.ensureProfileDirectory(profile); err != nil {
		return err
	}
	if err := s.prepareProfile(tool, profile.Dir); err != nil {
		return err
	}
	env := isolatedEnv(s.Env(), spec.ConfigEnv, profile.Dir, spec.ClearEnv)
	argv := append(append([]string{}, spec.ACP.Args...), args...)
	return s.Runner.Replace(ctx, spec.ACP.Binary, argv, env, io)
}

func isolatedEnv(base []string, configKey, configValue string, clear []string) []string {
	remove := make(map[string]struct{}, len(clear)+1)
	remove[configKey] = struct{}{}
	for _, key := range clear {
		remove[key] = struct{}{}
	}
	out := make([]string, 0, len(base)+1)
	for _, item := range base {
		key, _, ok := strings.Cut(item, "=")
		if ok {
			if _, drop := remove[key]; drop {
				continue
			}
		}
		out = append(out, item)
	}
	return append(out, configKey+"="+configValue)
}

func (s *Service) prepareProfile(tool, profileDir string) error {
	switch tool {
	case "claude":
		return s.ensureClaudeContextIsolation(profileDir)
	case "codex":
		// Codex resolves user-global guidance from CODEX_HOME itself. A profile's
		// AGENTS.override.md / AGENTS.md belongs inside that profile and must not
		// be inherited from the default ~/.codex directory.
		return nil
	default:
		return fmt.Errorf("unknown tool %q", tool)
	}
}

func (s *Service) ensureClaudeContextIsolation(profileDir string) error {
	defaultDir := filepath.Join(s.HomeDir, ".claude")
	if filepath.Clean(profileDir) == filepath.Clean(defaultDir) {
		return nil
	}

	root, err := safefs.Open(profileDir)
	if err != nil {
		return fmt.Errorf("open Claude profile directory safely: %w", err)
	}
	defer root.Close()

	settings := map[string]json.RawMessage{}
	if raw, err := readStableRegularRoot(root, "settings.json"); err == nil {
		if err := json.Unmarshal(raw, &settings); err != nil {
			return fmt.Errorf("decode existing settings.json: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	var excludes []string
	if raw, ok := settings["claudeMdExcludes"]; ok && len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &excludes); err != nil {
			return fmt.Errorf("decode settings.json claudeMdExcludes: %w", err)
		}
	}

	want := []string{
		filepath.Join(defaultDir, "CLAUDE.md"),
		filepath.Join(defaultDir, "CLAUDE.local.md"),
		filepath.Join(defaultDir, "rules", "**"),
	}
	seen := make(map[string]struct{}, len(excludes)+len(want))
	for _, item := range excludes {
		seen[item] = struct{}{}
	}
	changed := false
	for _, item := range want {
		if _, ok := seen[item]; ok {
			continue
		}
		excludes = append(excludes, item)
		seen[item] = struct{}{}
		changed = true
	}
	if !changed {
		return nil
	}
	rawExcludes, err := json.Marshal(excludes)
	if err != nil {
		return err
	}
	settings["claudeMdExcludes"] = rawExcludes
	raw, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return writeAtomicRoot(root, "settings.json", raw, 0o600, ".settings-")
}

func (s *Service) ApplyStatusline(tool, alias, template string) error {
	if tool != "claude" {
		return fmt.Errorf("apply-statusline is only supported for Claude profiles")
	}
	profile, _, err := s.Profile(tool, alias)
	if err != nil {
		return err
	}
	if err := s.ensureProfileDirectory(profile); err != nil {
		return err
	}
	statusline, err := s.loadTemplate(template)
	if err != nil {
		return err
	}
	root, err := safefs.Open(profile.Dir)
	if err != nil {
		return fmt.Errorf("open profile directory safely: %w", err)
	}
	defer root.Close()

	settings := map[string]json.RawMessage{}
	if raw, err := readStableRegularRoot(root, "settings.json"); err == nil {
		if err := json.Unmarshal(raw, &settings); err != nil {
			return fmt.Errorf("decode existing settings.json: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	settings["statusLine"] = statusline
	raw, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return writeAtomicRoot(root, "settings.json", raw, 0o600, ".settings-")
}

func (s *Service) loadTemplate(name string) (json.RawMessage, error) {
	if name == "" {
		name = "default"
	}
	if strings.ContainsAny(name, `/\\`) || name == "." || name == ".." {
		return nil, fmt.Errorf("invalid template name %q", name)
	}
	if s.TemplateDir != "" {
		path := filepath.Join(s.TemplateDir, name+".json")
		if raw, err := os.ReadFile(path); err == nil {
			if !json.Valid(raw) {
				return nil, fmt.Errorf("template %q is not valid JSON", name)
			}
			return json.RawMessage(raw), nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	raw, err := builtinTemplates.ReadFile("templates/" + name + ".json")
	if err != nil {
		return nil, fmt.Errorf("template %q not found", name)
	}
	if !json.Valid(raw) {
		return nil, fmt.Errorf("built-in template %q is invalid", name)
	}
	return json.RawMessage(raw), nil
}

func randomHex(bytesN int) (string, error) {
	buf := make([]byte, bytesN)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
