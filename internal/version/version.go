package version

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
)

// SuiteVersion is the Go module/GitHub release version. Release builds inject
// the git tag with -ldflags; tool product versions come from cmd/<tool>/tool.json.
var SuiteVersion = "dev"

const ToolManifestSchemaVersion = 1

type ToolManifest struct {
	SchemaVersion int    `json:"schema_version"`
	Name          string `json:"name"`
	Version       string `json:"version"`
	Stability     string `json:"stability"`
}

var semverRE = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-(?:[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+(?:[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)

func ParseToolManifest(raw []byte, expectedName string) (ToolManifest, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var m ToolManifest
	if err := dec.Decode(&m); err != nil {
		return ToolManifest{}, fmt.Errorf("decode tool manifest: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return ToolManifest{}, fmt.Errorf("tool manifest must contain exactly one JSON object")
		}
		return ToolManifest{}, fmt.Errorf("decode trailing tool manifest data: %w", err)
	}
	if m.SchemaVersion != ToolManifestSchemaVersion {
		return ToolManifest{}, fmt.Errorf("unsupported tool manifest schema_version %d (expected %d)", m.SchemaVersion, ToolManifestSchemaVersion)
	}
	if m.Name == "" {
		return ToolManifest{}, fmt.Errorf("tool manifest name is required")
	}
	if expectedName != "" && m.Name != expectedName {
		return ToolManifest{}, fmt.Errorf("tool manifest name %q does not match command %q", m.Name, expectedName)
	}
	if !semverRE.MatchString(m.Version) {
		return ToolManifest{}, fmt.Errorf("tool version %q must be SemVer without a leading v", m.Version)
	}
	switch m.Stability {
	case "experimental", "alpha", "beta", "stable":
	default:
		return ToolManifest{}, fmt.Errorf("unsupported tool stability %q", m.Stability)
	}
	return m, nil
}
