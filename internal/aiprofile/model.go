package aiprofile

const StoreSchemaVersion = 1

type Profile struct {
	Tool      string `json:"tool"`
	Alias     string `json:"alias"`
	Dir       string `json:"dir"`
	CreatedAt string `json:"created_at"`
}

type StoreData struct {
	SchemaVersion int       `json:"schema_version"`
	Profiles      []Profile `json:"profiles"`
}

type ToolSpec struct {
	Name      string
	Binary    string
	ConfigEnv string
	ClearEnv  []string
	ACP       *ACPTool
}

type ACPTool struct {
	Binary string
	Args   []string
}

var Tools = []ToolSpec{
	{
		Name:      "claude",
		Binary:    "claude",
		ConfigEnv: "CLAUDE_CONFIG_DIR",
		ClearEnv: []string{
			// Provider selection and host routing. These can bypass the account/login
			// stored under CLAUDE_CONFIG_DIR, so a selected ai-profile must not inherit
			// them accidentally from the parent shell.
			"CLAUDE_CODE_USE_ANTHROPIC_AWS",
			"CLAUDE_CODE_USE_BEDROCK",
			"CLAUDE_CODE_USE_FOUNDRY",
			"CLAUDE_CODE_USE_MANTLE",
			"CLAUDE_CODE_USE_VERTEX",
			"CLAUDE_CODE_PROVIDER_MANAGED_BY_HOST",
			"CLAUDE_CODE_SKIP_BEDROCK_AUTH",
			"CLAUDE_CODE_SKIP_FOUNDRY_AUTH",
			"CLAUDE_CODE_SKIP_MANTLE_AUTH",
			"CLAUDE_CODE_SKIP_VERTEX_AUTH",

			// Direct Anthropic / OAuth / federation authentication.
			"ANTHROPIC_API_KEY",
			"ANTHROPIC_AUTH_TOKEN",
			"CLAUDE_CODE_OAUTH_TOKEN",
			"CLAUDE_CODE_OAUTH_REFRESH_TOKEN",
			"CLAUDE_CODE_OAUTH_SCOPES",
			"ANTHROPIC_PROFILE",
			"ANTHROPIC_FEDERATION_RULE_ID",
			"ANTHROPIC_ORGANIZATION_ID",
			"ANTHROPIC_WORKSPACE_ID",

			// Provider-specific Claude credentials and routing. Generic AWS/GCP/Azure
			// environment variables are intentionally preserved so project commands are
			// not broken; profiles that opt into those providers may still use the
			// normal cloud credential chain.
			"ANTHROPIC_AWS_API_KEY",
			"ANTHROPIC_AWS_BASE_URL",
			"ANTHROPIC_AWS_WORKSPACE_ID",
			"ANTHROPIC_BEDROCK_BASE_URL",
			"ANTHROPIC_BEDROCK_MANTLE_BASE_URL",
			"ANTHROPIC_BEDROCK_REGION_PREFIX",
			"ANTHROPIC_BEDROCK_SERVICE_TIER",
			"AWS_BEARER_TOKEN_BEDROCK",
			"ANTHROPIC_FOUNDRY_API_KEY",
			"ANTHROPIC_FOUNDRY_AUTH_TOKEN",
			"ANTHROPIC_FOUNDRY_BASE_URL",
			"ANTHROPIC_FOUNDRY_RESOURCE",
			"ANTHROPIC_VERTEX_BASE_URL",
			"ANTHROPIC_VERTEX_PROJECT_ID",
			"ANTHROPIC_BASE_URL",
			"ANTHROPIC_CUSTOM_HEADERS",

			// Alternate state roots can defeat profile isolation.
			"CLAUDE_SECURESTORAGE_CONFIG_DIR",
			"CLAUDE_CODE_PLUGIN_CACHE_DIR",
		},
		ACP: &ACPTool{Binary: "claude-agent-acp"},
	},
	{
		Name:      "codex",
		Binary:    "codex",
		ConfigEnv: "CODEX_HOME",
		ClearEnv: []string{
			"OPENAI_API_KEY",
			"OPENAI_BASE_URL",
			"CODEX_API_KEY",
			"CODEX_ACCESS_TOKEN",
			"CODEX_SQLITE_HOME",
		},
		ACP: &ACPTool{Binary: "codex-acp"},
	},
}

func LookupTool(name string) (ToolSpec, bool) {
	for _, tool := range Tools {
		if tool.Name == name {
			return tool, true
		}
	}
	return ToolSpec{}, false
}
