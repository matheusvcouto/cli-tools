package notionexport

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ErrInvalidPageID reports input that does not contain a Notion page ID.
var ErrInvalidPageID = errors.New("page ID do Notion inválido")

var (
	compactIDPattern = regexp.MustCompile(`(?i)(?:^|[^0-9a-f])([0-9a-f]{32})(?:[?#/&]|$)`)
	uuidIDPattern    = regexp.MustCompile(`(?i)(?:^|[^0-9a-f])([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})(?:[?#/&]|$)`)
)

// ParsePageID extracts the compact, lowercase 32-hex page ID from a public
// Notion URL, a hyphenated UUID or a bare ID. URL slugs commonly end with
// "-<id>", so the match is anchored to the ID boundaries rather than to the
// whole string; query and fragment parts never contribute an ID.
func ParsePageID(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("%w: entrada vazia", ErrInvalidPageID)
	}
	candidate := trimmed
	if i := strings.IndexAny(candidate, "?#"); i >= 0 {
		candidate = candidate[:i]
	}
	if m := compactIDPattern.FindStringSubmatch(candidate); m != nil {
		return strings.ToLower(m[1]), nil
	}
	if m := uuidIDPattern.FindStringSubmatch(candidate); m != nil {
		return CompactID(m[1]), nil
	}
	if compact := CompactID(candidate); isCompactID(compact) {
		return compact, nil
	}
	return "", fmt.Errorf("%w: %q", ErrInvalidPageID, trimmed)
}

// CompactID removes hyphens and lowercases an ID. It does not validate it.
func CompactID(id string) string { return strings.ToLower(strings.ReplaceAll(id, "-", "")) }

// UUID formats a compact 32-hex ID as the hyphenated form used by Notion's
// internal API. Values that are not compact IDs are returned unchanged.
func UUID(id string) string {
	compact := CompactID(id)
	if !isCompactID(compact) {
		return id
	}
	return compact[:8] + "-" + compact[8:12] + "-" + compact[12:16] + "-" + compact[16:20] + "-" + compact[20:]
}

func isCompactID(s string) bool {
	if len(s) != 32 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
