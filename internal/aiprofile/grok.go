package aiprofile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/matheusvcouto/cli-tools/internal/safefs"
)

// New Grok profiles start with cross-tool compatibility disabled so selecting
// one profile does not silently import user-global Claude/Cursor state. Project
// Grok state and generic top-level project instructions remain governed by
// Grok Build's own trust/config rules.
const grokDefaultConfig = `# Created by ai-profile. Edit this file for profile-local Grok preferences.
# Do not store access tokens in source control or snapshot archives.
[cli]
auto_update = false

[compat.claude]
skills = false
rules = false
agents = false
mcps = false
hooks = false
sessions = false

[compat.cursor]
skills = false
rules = false
agents = false
mcps = false
hooks = false
sessions = false

[compat.codex]
# The current upstream config reference documents these scan sources.
skills = false
hooks = false
# Kept for installations that still expose compatible sessions.
sessions = false
`

// Only remove inherited values that can change the selected identity, move
// credentials/configuration outside GROK_HOME, or override the per-profile
// compatibility settings. GROK_DISABLE_API_KEY_AUTH and GROK_FORCE_LOGIN_TEAM_ID
// are security policy controls and intentionally remain inherited. Do not
// remove sandbox, enterprise requirements, telemetry restrictions, or
// unrelated GROK_* controls indiscriminately.
//
// Auth variables beyond those in the user guide are cleared defensively:
// older and embedded Grok launchers may also accept an inline token or a
// credential file path. The profile is authoritative for authentication.
var grokClearEnv = []string{
	"XAI_API_KEY",
	"GROK_CODE_XAI_API_KEY",
	"GROK_AUTH",
	"GROK_AUTH_JSON",
	"GROK_AUTH_PATH",
	"GROK_AUTH_EXPIRED",
	// Policy restriction GROK_DISABLE_API_KEY_AUTH is deliberately inherited.
	"GROK_AUTH_PROVIDER_COMMAND",
	"GROK_AUTH_PROVIDER_LABEL",
	"GROK_AUTH_TOKEN_TTL",
	"GROK_AUTH_EARLY_INVALIDATION_SECS",
	"GROK_OIDC_ISSUER",
	"GROK_OIDC_CLIENT_ID",
	"GROK_OIDC_AUDIENCE",
	"GROK_OIDC_SCOPES",
	"GROK_OAUTH2_ISSUER",
	"GROK_OAUTH2_CLIENT_ID",
	"GROK_OAUTH2_PRINCIPAL_ID",
	"GROK_OAUTH2_PRINCIPAL_TYPE",
	"GROK_OAUTH2_REFERRER",
	"GROK_OAUTH2_SCOPES",
	"GROK_WS_URL",
	"GROK_WS_ORIGIN",
	"GROK_DEPLOYMENT_KEY",
	"GROK_CONFIG",
	"GROK_CONFIG_PATH",
	"GROK_CLI_CHAT_PROXY_BASE_URL",
	"GROK_MODELS_BASE_URL",
	"GROK_MODELS_LIST_URL",
	"GROK_XAI_API_BASE_URL",
	"XAI_API_BASE_URL",
	"GROK_GATEWAY_URL",
	// Per-process routing and credential-file overrides must not point to an
	// unrelated profile. Admin pinned requirements and security toggles remain.
	"GROK_TRACE_UPLOAD_CREDENTIALS_FILE",
	"GROK_TRACE_UPLOAD_BUCKET",
	"GROK_TRACE_UPLOAD_ENDPOINT_URL",
	"GROK_TRACE_UPLOAD_REGION",
	"GROK_TRACE_UPLOAD_URL",
	// Feedback is optional, but an inherited destination must not redirect
	// profile-specific diagnostics to an unrelated collector.
	"GROK_FEEDBACK_BASE_URL",
	"GROK_LOG_FILE",
	"GROK_DEBUG_LOG",
	"GROK_AGENT",
	// Environment overrides take precedence over config.toml in Grok. Strip
	// inherited toggles instead of forcing false: new profiles start with
	// compatibility disabled, while an explicit local config edit can opt in.
	"GROK_CLAUDE_SKILLS_ENABLED",
	"GROK_CLAUDE_RULES_ENABLED",
	"GROK_CLAUDE_AGENTS_ENABLED",
	"GROK_CLAUDE_MCPS_ENABLED",
	"GROK_CLAUDE_HOOKS_ENABLED",
	"GROK_CLAUDE_SESSIONS_ENABLED",
	"GROK_CURSOR_SKILLS_ENABLED",
	"GROK_CURSOR_RULES_ENABLED",
	"GROK_CURSOR_AGENTS_ENABLED",
	"GROK_CURSOR_MCPS_ENABLED",
	"GROK_CURSOR_HOOKS_ENABLED",
	"GROK_CURSOR_SESSIONS_ENABLED",
	"GROK_CODEX_SESSIONS_ENABLED",
}

// Disable background self-updates during a managed launch. Profiles' own
// compatibility preferences live in config.toml, not forced environment flags.
func grokForcedEnvironment() []envValue {
	return []envValue{{key: "GROK_DISABLE_AUTOUPDATER", value: "1"}}
}

// initGrokProfile runs under the store lock before index.json is committed.
// Failure rolls back the new directory, so an initialized profile is never
// published half-created. No Grok process or network action occurs here.
func initGrokProfile(root *safefs.Root, name, _ string) error {
	config := filepath.Join(name, "config.toml")
	f, err := root.OpenFile(config, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create profile-local Grok config: %w", err)
	}
	_, writeErr := f.Write([]byte(grokDefaultConfig))
	if writeErr == nil {
		writeErr = f.Sync()
	}
	closeErr := f.Close()
	if writeErr != nil {
		return fmt.Errorf("persist profile-local Grok config: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close profile-local Grok config: %w", closeErr)
	}
	return nil
}

// Existing profile configuration is user-owned and is never rewritten at
// launch. We only verify the isolation anchor cannot be redirected through a
// symlink/special file. The containing .ai-profiles root is independently
// secured by Store.Load/EnsureRoot; on Unix its 0700 mode prevents traversal,
// and on Windows its protected DACL is inherited by newly created profiles.
func (s *Service) ensureGrokProfile(profileDir string) error {
	root, err := safefs.Open(profileDir)
	if err != nil {
		return fmt.Errorf("open Grok profile safely: %w", err)
	}
	defer root.Close()
	before, err := root.Lstat("config.toml")
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("Grok profile has no config.toml; refusing to recreate an existing profile: %w", err)
	}
	if err != nil {
		return fmt.Errorf("inspect Grok profile config: %w", err)
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return fmt.Errorf("Grok profile config.toml must be a real regular file")
	}
	// A bare Lstat can be invalidated before the next filesystem operation.
	// Recheck the opened file identity under os.Root, including after Open.
	// This protects against a swapped leaf during validation; it is not a
	// promise of immunity to concurrent same-user mutation after launch.
	f, err := root.Open("config.toml")
	if err != nil {
		return fmt.Errorf("open Grok profile config safely: %w", err)
	}
	opened, statErr := f.Stat()
	after, afterErr := root.Lstat("config.toml")
	closeErr := f.Close()
	if statErr != nil {
		return fmt.Errorf("stat opened Grok profile config: %w", statErr)
	}
	if afterErr != nil {
		return fmt.Errorf("reinspect Grok profile config: %w", afterErr)
	}
	if !opened.Mode().IsRegular() || after.Mode()&os.ModeSymlink != 0 || !after.Mode().IsRegular() ||
		!os.SameFile(before, opened) || !os.SameFile(opened, after) {
		return fmt.Errorf("Grok profile config.toml changed identity during validation")
	}
	if closeErr != nil {
		return fmt.Errorf("close Grok profile config: %w", closeErr)
	}
	return nil
}
