package notionexport

import (
	"encoding/json"
	"strings"
)

// maxRenderDepth bounds nesting so cyclic or hostile recordMaps cannot exhaust
// the stack. Notion's UI does not produce anything close to this depth.
const maxRenderDepth = 64

// AssetResolver maps an asset block to the URL written in the outputs: a local
// "assets/..." path when downloaded, else its remote URL. It must not do I/O.
type AssetResolver func(blockID, remoteURL string) string

// RemoteAssets is the resolver used when nothing was downloaded.
func RemoteAssets(_ string, remoteURL string) string { return remoteURL }

type renderContext struct {
	rm      *RecordMap
	resolve AssetResolver
	active  map[string]bool
}

func newRenderContext(rm *RecordMap, resolve AssetResolver) *renderContext {
	if resolve == nil {
		resolve = RemoteAssets
	}
	return &renderContext{rm: rm, resolve: resolve, active: map[string]bool{}}
}

// children returns renderable child blocks in order.
func (c *renderContext) children(ids []string) []*Block {
	out := make([]*Block, 0, len(ids))
	for _, id := range ids {
		if b, ok := c.rm.Block(id); ok && b.Alive {
			out = append(out, b)
		}
	}
	return out
}

// enter guards against cycles (synced blocks referencing ancestors) and depth.
func (c *renderContext) enter(b *Block, depth int) bool {
	key := CompactID(b.ID)
	if depth > maxRenderDepth || c.active[key] {
		return false
	}
	c.active[key] = true
	return true
}

func (c *renderContext) leave(b *Block) { delete(c.active, CompactID(b.ID)) }

// childIDs returns the content of a block, following synced-block references.
func (c *renderContext) childIDs(b *Block) []string {
	if b.Type == "transclusion_reference" {
		if target, ok := c.rm.Block(b.FormatPointerID("transclusion_reference_pointer")); ok {
			return target.Content
		}
		return nil
	}
	return b.Content
}

// assetLink resolves an asset block's final URL, falling back to the safe
// remote URL when the resolver result is unsafe.
func (c *renderContext) assetLink(b *Block) (string, bool) {
	remote := c.rm.SignedURL(b.ID)
	if !IsHTTPURL(remote) {
		remote = PlainText(b.Property("source"))
	}
	if remote == "" {
		return "", false
	}
	if u, ok := SafeURL(c.resolve(b.ID, remote)); ok {
		return u, true
	}
	return SafeURL(remote)
}

// pageMention resolves a ["p", id] mention to a title and public URL.
func (c *renderContext) pageMention(id string) (string, string) {
	title := "página do Notion"
	if b, ok := c.rm.Block(id); ok {
		if t := strings.TrimSpace(PlainText(b.Property("title"))); t != "" {
			title = t
		}
	}
	return title, PublicPageURL(id)
}

// mentionText renders inline mention placeholders as plain text plus an
// optional link; ok=false means the segment is ordinary text.
func (c *renderContext) mentionText(seg Segment) (text, link string, ok bool) {
	if seg.Text != mentionMarker {
		return "", "", false
	}
	for _, d := range seg.Decorations {
		switch d.Kind {
		case "p":
			t, u := c.pageMention(d.Arg)
			return t, u, true
		case "d":
			if date := d.DateStart(); date != "" {
				return date, "", true
			}
		case "u":
			return "@usuário", "", true
		case "lm":
			var v struct {
				Href  string `json:"href"`
				Title string `json:"title"`
			}
			if json.Unmarshal(d.Raw, &v) == nil && v.Href != "" {
				if v.Title == "" {
					v.Title = v.Href
				}
				return v.Title, v.Href, true
			}
		}
	}
	return "", "", true
}

func equationOf(seg Segment) (string, bool) {
	for _, d := range seg.Decorations {
		if d.Kind == "e" {
			return d.Arg, true
		}
	}
	return "", false
}

func hasDecoration(seg Segment, kind string) bool {
	for _, d := range seg.Decorations {
		if d.Kind == kind {
			return true
		}
	}
	return false
}

func linkOf(seg Segment) string {
	for _, d := range seg.Decorations {
		if d.Kind == "a" {
			return d.Arg
		}
	}
	return ""
}

// iconText returns an emoji/text page or callout icon; URL icons are ignored
// because they would be the only remote reference not covered by the asset flow.
func iconText(b *Block) string {
	icon := b.FormatString("page_icon")
	if icon == "" || strings.Contains(icon, "/") || strings.Contains(icon, ":") || len(icon) > 32 {
		return ""
	}
	return icon
}

func codeLanguage(b *Block) string {
	raw := strings.ToLower(strings.TrimSpace(PlainText(b.Property("language"))))
	if raw == "" || raw == "plain text" || raw == "plaintext" {
		return ""
	}
	var out strings.Builder
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '+' || r == '#' || r == '_':
			out.WriteRune(r)
		default:
			out.WriteByte('-')
		}
	}
	return strings.Trim(out.String(), "-")
}

// tableRows returns the column order, header flags and rows of a simple table.
func (c *renderContext) tableRows(b *Block) (columns []string, header bool, rows []*Block) {
	columns = b.FormatStrings("table_block_column_order")
	header = b.FormatBool("table_block_column_header")
	for _, row := range c.children(b.Content) {
		if row.Type == "table_row" {
			rows = append(rows, row)
		}
	}
	return columns, header, rows
}

func isListType(t string) bool { return t == "bulleted_list" || t == "numbered_list" }
