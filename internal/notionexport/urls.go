package notionexport

import (
	"net/url"
	"path"
	"strings"
	"unicode"
)

// publicNotionOrigin is used for links to other Notion pages. Child pages are
// not exported, so they stay reachable through Notion's public URL.
const publicNotionOrigin = "https://www.notion.so/"

// SafeURL returns value when it is safe to place in a rendered href/src:
// http(s) and mailto absolute URLs, fragments and local relative paths without
// parent traversal. Notion's internal "/<page-id>" links become public Notion
// URLs. Everything else (javascript:, data:, protocol-relative, unknown
// schemes, control characters) is rejected with ok=false.
func SafeURL(value string) (string, bool) {
	s := strings.TrimSpace(value)
	if s == "" || strings.ContainsFunc(s, func(r rune) bool { return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) }) {
		return "", false
	}
	if strings.HasPrefix(s, "#") {
		return s, true
	}
	if strings.HasPrefix(s, "//") || strings.HasPrefix(s, `\`) {
		return "", false
	}
	if strings.HasPrefix(s, "/") {
		return notionInternalLink(s)
	}
	if !hasScheme(s) {
		return safeRelative(s)
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		if u.Host == "" || u.User != nil {
			return "", false
		}
		return s, true
	case "mailto":
		if u.Opaque == "" {
			return "", false
		}
		return s, true
	}
	return "", false
}

func hasScheme(s string) bool {
	for i, r := range s {
		switch {
		case r == ':':
			return i > 0
		case r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z':
		case i > 0 && (r >= '0' && r <= '9' || r == '+' || r == '-' || r == '.'):
		default:
			return false
		}
	}
	return false
}

func safeRelative(s string) (string, bool) {
	if strings.Contains(s, `\`) {
		return "", false
	}
	p := s
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		p = p[:i]
	}
	for _, part := range strings.Split(p, "/") {
		if part == ".." {
			return "", false
		}
	}
	return s, true
}

// notionInternalLink converts "/<page-id>" or "/<slug>-<page-id>" links to the
// public Notion page URL; other root-relative paths have no meaning offline.
func notionInternalLink(s string) (string, bool) {
	p := s
	fragment := ""
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		if j := strings.IndexByte(p, '#'); j >= 0 {
			fragment = p[j:]
		}
		p = p[:i]
	}
	id, err := ParsePageID(path.Base(p))
	if err != nil || strings.Count(strings.Trim(p, "/"), "/") > 1 {
		return "", false
	}
	if _, ok := SafeURL(fragment); fragment != "" && !ok {
		fragment = ""
	}
	return publicNotionOrigin + id + fragment, true
}

// PublicPageURL is the public Notion URL for a page ID.
func PublicPageURL(id string) string { return publicNotionOrigin + CompactID(id) }

// IsLocalAssetPath reports whether a resolved URL is one of this exporter's
// own asset paths, used to decide between native media and plain links.
func IsLocalAssetPath(value string) bool {
	s := strings.ReplaceAll(value, `\`, "/")
	_, ok := safeRelative(s)
	return ok && strings.HasPrefix(s, "assets/")
}

// IsHTTPURL reports an absolute http(s) URL with a host.
func IsHTTPURL(value string) bool {
	u, err := url.Parse(strings.TrimSpace(value))
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
