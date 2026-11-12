package notionexport

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// ManifestSchemaVersion is the manifest.json format version. It is
// independent from the product version and changes only with the format.
const ManifestSchemaVersion = 1

// GeneratorName is written to manifest.json and identifies exports that this
// tool may replace with --force. LegacyGeneratorName is the previous Deno CLI.
const (
	GeneratorName       = "notion-export"
	LegacyGeneratorName = "notion-public-exporter"
)

// maxManifestBytes bounds manifest reads during --force validation.
const maxManifestBytes = 8 << 20

// Manifest is the manifest.json document. Field names keep the camelCase
// layout of the original exporter (schemaVersion 1).
type Manifest struct {
	SchemaVersion int            `json:"schemaVersion"`
	Generator     Generator      `json:"generator"`
	Format        Format         `json:"format"`
	Title         string         `json:"title"`
	PageID        string         `json:"pageId"`
	Source        *string        `json:"source"`
	ExportedAt    string         `json:"exportedAt"`
	BlockCount    int            `json:"blockCount"`
	Assets        ManifestAssets `json:"assets"`
	Files         ManifestFiles  `json:"files"`
}

type Generator struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ManifestAssets struct {
	Discovered      int            `json:"discovered"`
	Selected        int            `json:"selected"`
	Downloaded      int            `json:"downloaded"`
	Skipped         int            `json:"skipped"`
	Failed          int            `json:"failed"`
	DownloadedBytes int64          `json:"downloadedBytes"`
	Items           []ManifestItem `json:"items"`
}

type ManifestItem struct {
	Index         int            `json:"index"`
	BlockID       string         `json:"blockId"`
	Type          AssetType      `json:"type"`
	Source        string         `json:"source"`
	URL           string         `json:"url"`
	Caption       string         `json:"caption"`
	EstimatedSize *int64         `json:"estimatedSize,omitempty"`
	ContentType   string         `json:"contentType,omitempty"`
	Status        DownloadStatus `json:"status"`
	Path          *string        `json:"path"`
	Bytes         *int64         `json:"bytes"`
	Error         *string        `json:"error"`
}

type ManifestFiles struct {
	Markdown  *string `json:"markdown"`
	HTML      *string `json:"html"`
	Styles    *string `json:"styles"`
	RecordMap string  `json:"recordMap"`
	Assets    string  `json:"assets"`
}

// readManifestIdentity reads only what --force needs to prove ownership.
func readManifestIdentity(r io.Reader) (generator, pageID string, err error) {
	data, err := io.ReadAll(io.LimitReader(r, maxManifestBytes+1))
	if err != nil {
		return "", "", err
	}
	if len(data) > maxManifestBytes {
		return "", "", fmt.Errorf("manifest.json excede %d bytes", maxManifestBytes)
	}
	var m struct {
		Generator struct {
			Name string `json:"name"`
		} `json:"generator"`
		PageID string `json:"pageId"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return "", "", fmt.Errorf("manifest.json inválido: %w", err)
	}
	return m.Generator.Name, m.PageID, nil
}

const maxSlugBytes = 80

// ValidateExportName checks a user-chosen export directory name: one
// portable component (ASCII letters, digits, '.', '_', '-'), at most 80
// bytes and not starting with '.', which is reserved for staging.
func ValidateExportName(name string) error {
	if name == "" || len(name) > maxSlugBytes || strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") || portableComponent(name) != name {
		return fmt.Errorf("nome de diretório inválido %q: use letras ASCII, números, '.', '_' ou '-', sem começar com '.' (até %d bytes)", name, maxSlugBytes)
	}
	return nil
}

// Slugify turns a title into the export directory name: lowercase ASCII
// letters/digits separated by single dashes, at most 80 bytes, never empty and
// never starting with a dot (staging names are reserved dot-names).
func Slugify(title string) string {
	var b strings.Builder
	dash := false
	for _, r := range norm.NFKD.String(title) {
		switch {
		case unicode.Is(unicode.Mn, r):
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(unicode.ToLower(r))
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > maxSlugBytes {
		slug = strings.TrimRight(slug[:maxSlugBytes], "-")
	}
	if slug == "" {
		return "notion-page"
	}
	return slug
}
