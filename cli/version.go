package cli

import (
	"encoding/json"
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

const (
	builtinVersionCommandID  = "cli.builtin.version"
	builtinVersionJSONFlagID = "cli.builtin.version.json"
)

type VersionInfo struct {
	Tool               string `json:"tool"`
	Version            string `json:"version"`
	Stability          string `json:"stability,omitempty"`
	SuiteVersion       string `json:"suite_version,omitempty"`
	Revision           string `json:"revision,omitempty"`
	VCSModified        bool   `json:"vcs_modified,omitempty"`
	GoVersion          string `json:"go_version"`
	OS                 string `json:"os"`
	Arch               string `json:"arch"`
	SchemaVersion      int    `json:"cli_schema_version"`
	CompletionProtocol int    `json:"completion_protocol"`
}

func builtinVersionCommand() Command {
	return Command{
		ID:      builtinVersionCommandID,
		Name:    "version",
		Summary: "show product and build version information",
		Outputs: []OutputFormat{OutputHuman, OutputJSON},
		Flags: []Flag{{
			ID:      builtinVersionJSONFlagID,
			Long:    "json",
			Summary: "emit structured version information",
			Action:  FlagSwitch,
		}},
	}
}

func (c *CompiledApp) installVersionHandlers() {
	c.handlers[builtinVersionCommandID] = func(inv *Invocation) error {
		jsonMode, _ := ValueAs[bool](inv, builtinVersionJSONFlagID)
		if !jsonMode {
			_, err := fmt.Fprintln(inv.IO.Out, c.versionLine())
			return err
		}
		enc := json.NewEncoder(inv.IO.Out)
		enc.SetEscapeHTML(false)
		return enc.Encode(c.VersionInfo())
	}
}

func (c *CompiledApp) versionLine() string {
	version := strings.TrimSpace(c.graph.ProductVersion)
	if version == "" {
		version = "dev"
	}
	return c.graph.Name + " " + version
}

func (c *CompiledApp) VersionInfo() VersionInfo {
	info := VersionInfo{
		Tool:               c.graph.Name,
		Version:            c.graph.ProductVersion,
		Stability:          c.graph.Stability,
		SuiteVersion:       c.graph.SuiteVersion,
		GoVersion:          runtime.Version(),
		OS:                 runtime.GOOS,
		Arch:               runtime.GOARCH,
		SchemaVersion:      SchemaVersion,
		CompletionProtocol: CompletionProtocol,
	}
	if info.Version == "" {
		info.Version = "dev"
	}
	if build, ok := debug.ReadBuildInfo(); ok {
		if build.GoVersion != "" {
			info.GoVersion = build.GoVersion
		}
		if info.SuiteVersion == "" && build.Main.Version != "" && build.Main.Version != "(devel)" {
			info.SuiteVersion = build.Main.Version
		}
		for _, setting := range build.Settings {
			switch setting.Key {
			case "vcs.revision":
				info.Revision = setting.Value
			case "vcs.modified":
				info.VCSModified = setting.Value == "true"
			}
		}
	}
	return info
}
