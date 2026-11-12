package notionexport

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

// RenderMarkdown renders the page as CommonMark/GFM. Notion text is escaped so
// that it is shown literally (including raw HTML); links pass SafeURL. The
// page title is the only level-1 heading; Notion headings start at level 2.
func RenderMarkdown(rm *RecordMap, root *Block, resolve AssetResolver) string {
	c := newRenderContext(rm, resolve)
	c.active[CompactID(root.ID)] = true
	body := c.mdBlocks(c.children(root.Content), 0)
	out := "# " + mdEscapeLine(PageTitle(root))
	if body != "" {
		out += "\n\n" + body
	}
	return strings.TrimRight(out, "\n") + "\n"
}

// mdBlocks renders sibling blocks; consecutive list items form one tight list.
func (c *renderContext) mdBlocks(blocks []*Block, depth int) string {
	var chunks []string
	for i := 0; i < len(blocks); i++ {
		b := blocks[i]
		if isListType(b.Type) || b.Type == "to_do" {
			var items []string
			n := 0
			for ; i < len(blocks) && blocks[i].Type == b.Type; i++ {
				n++
				if item := c.mdListItem(blocks[i], n, depth); item != "" {
					items = append(items, item)
				}
			}
			i--
			if len(items) > 0 {
				chunks = append(chunks, strings.Join(items, "\n"))
			}
			continue
		}
		if s := c.mdBlock(b, depth); s != "" {
			chunks = append(chunks, s)
		}
	}
	return strings.Join(chunks, "\n\n")
}

func (c *renderContext) mdChildren(b *Block, depth int) string {
	return c.mdBlocks(c.children(c.childIDs(b)), depth+1)
}

func (c *renderContext) mdListItem(b *Block, n, depth int) string {
	if !c.enter(b, depth) {
		return ""
	}
	defer c.leave(b)
	marker := "- "
	switch b.Type {
	case "numbered_list":
		marker = fmt.Sprintf("%d. ", n)
	case "to_do":
		if strings.EqualFold(PlainText(b.Property("checked")), "Yes") {
			marker = "- [x] "
		} else {
			marker = "- [ ] "
		}
	}
	return mdItem(marker, c.mdRich(b.Property("title")), c.mdChildren(b, depth))
}

// mdItem lays out a list-like item: continuation lines and children are
// indented to the item's content column so they stay inside the item.
func mdItem(marker, text, children string) string {
	pad := strings.Repeat(" ", len(marker))
	out := marker + indentLines(text, pad, false)
	if children != "" {
		sep := "\n\n"
		if first, _, _ := strings.Cut(strings.TrimLeft(children, " "), "\n"); strings.HasPrefix(first, "- ") || len(first) > 2 && first[0] >= '0' && first[0] <= '9' {
			sep = "\n"
		}
		out += sep + indentLines(children, pad, true)
	}
	return out
}

func indentLines(s, pad string, first bool) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if (i > 0 || first) && line != "" {
			lines[i] = pad + line
		}
	}
	return strings.Join(lines, "\n")
}

func quoteLines(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line == "" {
			lines[i] = ">"
		} else {
			lines[i] = "> " + line
		}
	}
	return strings.Join(lines, "\n")
}

func (c *renderContext) mdBlock(b *Block, depth int) string {
	if !c.enter(b, depth) {
		return ""
	}
	defer c.leave(b)
	text := c.mdRich(b.Property("title"))
	join := func(parts ...string) string {
		var kept []string
		for _, p := range parts {
			if p != "" {
				kept = append(kept, p)
			}
		}
		return strings.Join(kept, "\n\n")
	}
	switch b.Type {
	case "text":
		return join(text, c.mdChildren(b, depth))
	case "header", "sub_header", "sub_sub_header", "header_4":
		level := map[string]int{"header": 2, "sub_header": 3, "sub_sub_header": 4, "header_4": 5}[b.Type]
		heading := ""
		if text != "" {
			heading = strings.Repeat("#", level) + " " + strings.ReplaceAll(text, "\\\n", " ")
		}
		return join(heading, c.mdChildren(b, depth))
	case "divider":
		return "---"
	case "quote":
		return quoteLines(join(text, c.mdChildren(b, depth)))
	case "callout":
		if icon := iconText(b); icon != "" && text != "" {
			text = icon + " " + text
		}
		return quoteLines(join(text, c.mdChildren(b, depth)))
	case "toggle":
		return mdItem("- ", text, c.mdChildren(b, depth))
	case "code":
		return mdFence(RawText(b.Property("title")), codeLanguage(b))
	case "equation":
		return "$$\n" + RawText(b.Property("title")) + "\n$$"
	case "image", "video", "audio", "pdf", "file":
		return c.mdAsset(b)
	case "embed", "tweet", "maps", "figma", "drive", "gist", "codepen", "excalidraw", "typeform", "replit", "miro":
		if u, ok := SafeURL(PlainText(b.Property("source"))); ok {
			return "[" + mdEscape(firstNonEmpty(PlainText(b.Property("caption")), "Conteúdo incorporado")) + "](" + mdURL(u) + ")"
		}
		return ""
	case "bookmark":
		if u, ok := SafeURL(PlainText(b.Property("link"))); ok {
			return "[" + firstNonEmpty(text, mdEscape(u)) + "](" + mdURL(u) + ")"
		}
		return text
	case "external_object_instance":
		if u, ok := SafeURL(b.FormatString("uri")); ok {
			return "[" + mdEscape(u) + "](" + mdURL(u) + ")"
		}
		return ""
	case "page":
		title := mdEscapeLine(PageTitle(b))
		if icon := iconText(b); icon != "" {
			title = icon + " " + title
		}
		return "[" + title + "](" + PublicPageURL(b.ID) + ")"
	case "alias":
		id := b.FormatPointerID("alias_pointer")
		if id == "" {
			return ""
		}
		title, u := c.pageMention(id)
		return "[↗ " + mdEscape(title) + "](" + u + ")"
	case "collection_view", "collection_view_page":
		return "*" + mdEscape("Base de dados do Notion não incluída no export.") + "*"
	case "table":
		return c.mdTable(b)
	case "column_list", "column", "transclusion_container", "transclusion_reference":
		return c.mdBlocks(c.children(c.childIDs(b)), depth+1)
	case "table_of_contents", "breadcrumb", "table_row":
		return ""
	default:
		return join(text, c.mdChildren(b, depth))
	}
}

func (c *renderContext) mdAsset(b *Block) string {
	caption := PlainText(b.Property("caption"))
	u, ok := c.assetLink(b)
	if !ok {
		if caption == "" {
			return ""
		}
		return mdEscapeLine(caption)
	}
	name := PlainText(b.Property("title"))
	label := func(prefix string) string {
		if d := firstNonEmpty(caption, name); d != "" {
			return mdEscape(prefix + ": " + d)
		}
		return mdEscape(prefix)
	}
	switch b.Type {
	case "image":
		img := "![" + mdEscape(singleLine(caption)) + "](" + mdURL(u) + ")"
		if caption != "" {
			img += "\n\n*" + mdEscape(singleLine(caption)) + "*"
		}
		return img
	case "video":
		return "[" + label("▶ Vídeo") + "](" + mdURL(u) + ")"
	case "audio":
		return "[" + label("▶ Áudio") + "](" + mdURL(u) + ")"
	case "pdf":
		return "[" + label("PDF") + "](" + mdURL(u) + ")"
	}
	return "[" + label("Arquivo") + "](" + mdURL(u) + ")"
}

func (c *renderContext) mdTable(b *Block) string {
	columns, header, rows := c.tableRows(b)
	if len(columns) == 0 || len(rows) == 0 {
		return ""
	}
	cell := func(row *Block, col string) string {
		// mdRich already escapes "|"; hard breaks cannot exist inside a cell.
		return strings.ReplaceAll(c.mdRich(row.Property(col)), "\\\n", " ")
	}
	line := func(cells []string) string { return "| " + strings.Join(cells, " | ") + " |" }
	var lines []string
	start := 0
	head := make([]string, len(columns))
	if header {
		for i, col := range columns {
			head[i] = cell(rows[0], col)
		}
		start = 1
	}
	lines = append(lines, line(head))
	sep := make([]string, len(columns))
	for i := range sep {
		sep[i] = "---"
	}
	lines = append(lines, line(sep))
	for _, row := range rows[start:] {
		cells := make([]string, len(columns))
		for i, col := range columns {
			cells[i] = cell(row, col)
		}
		lines = append(lines, line(cells))
	}
	return strings.Join(lines, "\n")
}

// mdRich renders rich text. Notion line breaks become CommonMark hard breaks.
func (c *renderContext) mdRich(raw json.RawMessage) string {
	var b strings.Builder
	for _, seg := range ParseRichText(raw) {
		b.WriteString(c.mdSegment(seg))
	}
	return mdEscapeLineStarts(b.String())
}

func (c *renderContext) mdSegment(seg Segment) string {
	if latex, ok := equationOf(seg); ok && seg.Text == equationMarker {
		return "$" + strings.ReplaceAll(latex, "$", `\$`) + "$"
	}
	text, link := seg.Text, linkOf(seg)
	var out string
	if mention, mentionLink, ok := c.mentionText(seg); ok {
		if mention == "" {
			return ""
		}
		out, link = mdEscape(mention), firstNonEmpty(mentionLink, link)
	} else if hasDecoration(seg, "c") {
		out = mdCodeSpan(strings.ReplaceAll(text, "\n", " "))
	} else {
		out = strings.ReplaceAll(mdEscape(text), "\n", "\\\n")
	}
	for _, d := range seg.Decorations {
		switch d.Kind {
		case "b":
			out = mdWrap(out, "**")
		case "i":
			out = mdWrap(out, "*")
		case "s":
			out = mdWrap(out, "~~")
		}
	}
	if link != "" {
		if u, ok := SafeURL(link); ok && strings.TrimSpace(out) != "" {
			out = "[" + out + "](" + mdURL(u) + ")"
		}
	}
	return out
}

// mdWrap keeps surrounding whitespace outside emphasis markers, as CommonMark
// requires for "**bold **" to be recognized.
func mdWrap(s, marker string) string {
	inner := strings.TrimFunc(s, unicode.IsSpace)
	if inner == "" {
		return s
	}
	start := strings.Index(s, inner)
	return s[:start] + marker + inner + marker + s[start+len(inner):]
}

var mdEscaper = strings.NewReplacer(`\`, `\\`, "`", "\\`", "*", `\*`, "_", `\_`, "[", `\[`, "]", `\]`, "<", `\<`, ">", `\>`, "~", `\~`, "|", `\|`)

func mdEscape(s string) string { return mdEscaper.Replace(s) }

func mdEscapeLine(s string) string { return mdEscapeLineStarts(mdEscape(singleLine(s))) }

// mdEscapeLineStarts escapes block markers that would otherwise turn text into
// headings, lists or setext underlines at the start of a line.
func mdEscapeLineStarts(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " ")
		lead := line[:len(line)-len(trimmed)]
		switch {
		case trimmed == "":
		case strings.HasPrefix(trimmed, "#"), strings.HasPrefix(trimmed, "="):
			lines[i] = lead + `\` + trimmed
		case strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "+ ") || trimmed == "-" || trimmed == "+" || strings.HasPrefix(trimmed, "---"):
			lines[i] = lead + `\` + trimmed
		default:
			j := 0
			for j < len(trimmed) && j < 9 && trimmed[j] >= '0' && trimmed[j] <= '9' {
				j++
			}
			if j > 0 && j < len(trimmed) && (trimmed[j] == '.' || trimmed[j] == ')') {
				lines[i] = lead + trimmed[:j] + `\` + trimmed[j:]
			}
		}
	}
	return strings.Join(lines, "\n")
}

func mdCodeSpan(s string) string {
	fence := strings.Repeat("`", longestRun(s, '`')+1)
	if strings.HasPrefix(s, "`") || strings.HasSuffix(s, "`") || strings.HasPrefix(s, " ") && strings.HasSuffix(s, " ") && strings.TrimSpace(s) != "" {
		return fence + " " + s + " " + fence
	}
	return fence + s + fence
}

func mdFence(code, lang string) string {
	fence := strings.Repeat("`", max(3, longestRun(code, '`')+1))
	return fence + lang + "\n" + code + "\n" + fence
}

func longestRun(s string, ch byte) int {
	best, cur := 0, 0
	for i := 0; i < len(s); i++ {
		if s[i] == ch {
			cur++
			best = max(best, cur)
		} else {
			cur = 0
		}
	}
	return best
}

// mdURL makes a validated URL safe inside "(...)": characters that would end
// or split the destination are percent-encoded.
func mdURL(u string) string {
	return strings.NewReplacer(" ", "%20", "(", "%28", ")", "%29", "<", "%3C", ">", "%3E", "\n", "", "\r", "").Replace(u)
}

func singleLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
