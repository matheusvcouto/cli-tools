package notionexport

import (
	"encoding/json"
	"strings"
)

// Decoration is one Notion rich-text annotation, for example ["b"] or
// ["a", "https://example.com"]. Arg holds the first string argument; mention
// decorations ("p", "u", "d") keep their raw argument for typed lookups.
type Decoration struct {
	Kind string
	Arg  string
	Raw  json.RawMessage
}

// Segment is one run of text with its annotations.
type Segment struct {
	Text        string
	Decorations []Decoration
}

// Notion stores inline mentions and equations as placeholder characters whose
// meaning lives in a decoration.
const (
	mentionMarker  = "‣"
	equationMarker = "⁍"
)

// ParseRichText decodes Notion's [[text, [[kind, arg?], ...]], ...] format.
// Malformed parts are skipped instead of failing the export.
func ParseRichText(raw json.RawMessage) []Segment {
	if len(raw) == 0 {
		return nil
	}
	var parts []json.RawMessage
	if json.Unmarshal(raw, &parts) != nil {
		return nil
	}
	segments := make([]Segment, 0, len(parts))
	for _, part := range parts {
		var fields []json.RawMessage
		if json.Unmarshal(part, &fields) != nil || len(fields) == 0 {
			continue
		}
		var seg Segment
		if json.Unmarshal(fields[0], &seg.Text) != nil {
			continue
		}
		if len(fields) > 1 {
			var decorations []json.RawMessage
			if json.Unmarshal(fields[1], &decorations) == nil {
				for _, d := range decorations {
					if dec, ok := parseDecoration(d); ok {
						seg.Decorations = append(seg.Decorations, dec)
					}
				}
			}
		}
		segments = append(segments, seg)
	}
	return segments
}

func parseDecoration(raw json.RawMessage) (Decoration, bool) {
	var fields []json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || len(fields) == 0 {
		return Decoration{}, false
	}
	var d Decoration
	if json.Unmarshal(fields[0], &d.Kind) != nil || d.Kind == "" {
		return Decoration{}, false
	}
	if len(fields) > 1 {
		d.Raw = fields[1]
		_ = json.Unmarshal(fields[1], &d.Arg)
	}
	return d, true
}

// PlainText returns the visible text, without mention/equation placeholders.
func PlainText(raw json.RawMessage) string {
	var b strings.Builder
	for _, seg := range ParseRichText(raw) {
		if seg.Text == mentionMarker || seg.Text == equationMarker {
			continue
		}
		b.WriteString(seg.Text)
	}
	return b.String()
}

// RawText concatenates every segment, including placeholders. It is used for
// verbatim values such as code and equations.
func RawText(raw json.RawMessage) string {
	var b strings.Builder
	for _, seg := range ParseRichText(raw) {
		b.WriteString(seg.Text)
	}
	return b.String()
}

// DateStart extracts start_date from a ["d", {...}] mention decoration.
func (d Decoration) DateStart() string {
	var v struct {
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
	}
	if json.Unmarshal(d.Raw, &v) != nil {
		return ""
	}
	if v.EndDate != "" && v.StartDate != "" {
		return v.StartDate + " → " + v.EndDate
	}
	return v.StartDate
}

// PageTitle returns the page title used for the export.
func PageTitle(page *Block) string {
	title := strings.TrimSpace(PlainText(page.Property("title")))
	if title == "" {
		return "Untitled Notion Page"
	}
	return title
}
