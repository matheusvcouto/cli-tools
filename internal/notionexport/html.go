package notionexport

import (
	_ "embed"
	"encoding/json"
	"html"
	"strings"
)

// PageCSS is the local stylesheet written next to page.html.
//
//go:embed styles.css
var PageCSS string

// ContentSecurityPolicy blocks scripts, frames, objects, forms and network
// connections. Images/media may be local or HTTPS (assets kept remote).
const ContentSecurityPolicy = "default-src 'none'; img-src 'self' https:; media-src 'self' https:; style-src 'self'; font-src 'none'; connect-src 'none'; frame-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'"

// RenderHTML renders a standalone, script-free HTML document. Every Notion
// string is escaped at output and every URL passes SafeURL; external embeds
// become links, never iframes.
func RenderHTML(rm *RecordMap, root *Block, resolve AssetResolver, generator string) string {
	c := newRenderContext(rm, resolve)
	c.active[CompactID(root.ID)] = true
	title := esc(PageTitle(root))
	body := c.htmlBlocks(c.children(root.Content), 0)
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"pt-BR\">\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	b.WriteString("<meta http-equiv=\"Content-Security-Policy\" content=\"" + esc(ContentSecurityPolicy) + "\">\n")
	b.WriteString("<meta name=\"referrer\" content=\"no-referrer\">\n")
	b.WriteString("<meta name=\"generator\" content=\"" + esc(generator) + "\">\n")
	b.WriteString("<title>" + title + "</title>\n<link rel=\"stylesheet\" href=\"styles.css\">\n</head>\n<body>\n")
	b.WriteString("<article class=\"notion-page\">\n<header class=\"page-header\"><h1 class=\"page-title\">" + title + "</h1></header>\n")
	b.WriteString("<main class=\"page-body\">\n" + body + "\n</main>\n</article>\n</body>\n</html>\n")
	return b.String()
}

func esc(s string) string { return html.EscapeString(s) }

func (c *renderContext) htmlBlocks(blocks []*Block, depth int) string {
	var pieces []string
	for i := 0; i < len(blocks); i++ {
		b := blocks[i]
		if isListType(b.Type) || b.Type == "to_do" {
			var items []string
			for ; i < len(blocks) && blocks[i].Type == b.Type; i++ {
				if item := c.htmlListItem(blocks[i], depth); item != "" {
					items = append(items, item)
				}
			}
			i--
			if len(items) == 0 {
				continue
			}
			switch b.Type {
			case "numbered_list":
				pieces = append(pieces, "<ol>"+strings.Join(items, "")+"</ol>")
			case "to_do":
				pieces = append(pieces, "<ul class=\"todo-list\">"+strings.Join(items, "")+"</ul>")
			default:
				pieces = append(pieces, "<ul>"+strings.Join(items, "")+"</ul>")
			}
			continue
		}
		if s := c.htmlBlock(b, depth); s != "" {
			pieces = append(pieces, s)
		}
	}
	return strings.Join(pieces, "\n")
}

func (c *renderContext) htmlChildren(b *Block, depth int) string {
	return c.htmlBlocks(c.children(c.childIDs(b)), depth+1)
}

func (c *renderContext) htmlListItem(b *Block, depth int) string {
	if !c.enter(b, depth) {
		return ""
	}
	defer c.leave(b)
	text := c.htmlRich(b.Property("title"))
	children := c.htmlChildren(b, depth)
	if children != "" {
		children = "\n" + children
	}
	if b.Type == "to_do" {
		checked := ""
		if strings.EqualFold(PlainText(b.Property("checked")), "Yes") {
			checked = " checked"
		}
		return "<li><label class=\"todo\"><input type=\"checkbox\" disabled" + checked + "><span>" + text + "</span></label>" + children + "</li>"
	}
	return "<li>" + text + children + "</li>"
}

func (c *renderContext) htmlBlock(b *Block, depth int) string {
	if !c.enter(b, depth) {
		return ""
	}
	defer c.leave(b)
	text := c.htmlRich(b.Property("title"))
	para := func() string {
		if text == "" {
			return ""
		}
		return "<p>" + text + "</p>"
	}
	switch b.Type {
	case "text":
		return para() + c.htmlChildren(b, depth)
	case "header", "sub_header", "sub_sub_header", "header_4":
		tag := map[string]string{"header": "h2", "sub_header": "h3", "sub_sub_header": "h4", "header_4": "h5"}[b.Type]
		heading := ""
		if text != "" {
			heading = "<" + tag + ">" + text + "</" + tag + ">"
		}
		if children := c.htmlChildren(b, depth); children != "" {
			return "<details open><summary>" + heading + "</summary>" + children + "</details>"
		}
		return heading
	case "divider":
		return "<hr>"
	case "quote":
		return "<blockquote>" + para() + c.htmlChildren(b, depth) + "</blockquote>"
	case "callout":
		icon := ""
		if i := iconText(b); i != "" {
			icon = "<span class=\"callout-icon\" aria-hidden=\"true\">" + esc(i) + "</span>"
		}
		return "<aside class=\"callout\">" + icon + "<div class=\"callout-body\">" + para() + c.htmlChildren(b, depth) + "</div></aside>"
	case "toggle":
		return "<details><summary>" + text + "</summary>" + c.htmlChildren(b, depth) + "</details>"
	case "code":
		class := ""
		if lang := codeLanguage(b); lang != "" {
			class = " class=\"language-" + esc(lang) + "\""
		}
		return "<pre><code" + class + ">" + esc(RawText(b.Property("title"))) + "</code></pre>"
	case "equation":
		return "<pre class=\"equation\"><code>" + esc(RawText(b.Property("title"))) + "</code></pre>"
	case "image", "video", "audio", "pdf", "file":
		return c.htmlAsset(b)
	case "embed", "tweet", "maps", "figma", "drive", "gist", "codepen", "excalidraw", "typeform", "replit", "miro":
		if u, ok := SafeURL(PlainText(b.Property("source"))); ok {
			return "<p>" + htmlLink(esc(firstNonEmpty(PlainText(b.Property("caption")), "Abrir conteúdo incorporado")), u, "asset-link") + "</p>"
		}
		return ""
	case "bookmark":
		if u, ok := SafeURL(PlainText(b.Property("link"))); ok {
			return "<p>" + htmlLink(firstNonEmpty(text, esc(u)), u, "") + "</p>"
		}
		return para()
	case "external_object_instance":
		if u, ok := SafeURL(b.FormatString("uri")); ok {
			return "<p>" + htmlLink(esc(u), u, "asset-link") + "</p>"
		}
		return ""
	case "page":
		title := esc(PageTitle(b))
		if icon := iconText(b); icon != "" {
			title = esc(icon) + " " + title
		}
		return "<p>" + htmlLink(title, PublicPageURL(b.ID), "page-link") + "</p>"
	case "alias":
		id := b.FormatPointerID("alias_pointer")
		if id == "" {
			return ""
		}
		title, u := c.pageMention(id)
		return "<p>" + htmlLink("↗ "+esc(title), u, "") + "</p>"
	case "collection_view", "collection_view_page":
		return "<p class=\"unsupported\">Base de dados do Notion não incluída no export.</p>"
	case "table":
		return c.htmlTable(b)
	case "column_list":
		var columns []string
		for _, column := range c.children(b.Content) {
			if !c.enter(column, depth+1) {
				continue
			}
			columns = append(columns, "<div class=\"column\">"+c.htmlChildren(column, depth+1)+"</div>")
			c.leave(column)
		}
		if len(columns) == 0 {
			return ""
		}
		return "<div class=\"columns\">" + strings.Join(columns, "") + "</div>"
	case "column", "transclusion_container", "transclusion_reference":
		return c.htmlChildren(b, depth)
	case "table_of_contents", "breadcrumb", "table_row":
		return ""
	default:
		return para() + c.htmlChildren(b, depth)
	}
}

func (c *renderContext) htmlAsset(b *Block) string {
	caption := c.htmlRich(b.Property("caption"))
	plainCaption := PlainText(b.Property("caption"))
	u, ok := c.assetLink(b)
	if !ok {
		if caption == "" {
			return ""
		}
		return "<p>" + caption + "</p>"
	}
	figcaption := ""
	if caption != "" {
		figcaption = "<figcaption>" + caption + "</figcaption>"
	}
	src := esc(u)
	label := func(prefix string) string {
		if d := firstNonEmpty(plainCaption, PlainText(b.Property("title"))); d != "" {
			return esc(prefix + ": " + d)
		}
		return esc(prefix)
	}
	switch b.Type {
	case "image":
		return "<figure><img src=\"" + src + "\" alt=\"" + esc(plainCaption) + "\" loading=\"lazy\" decoding=\"async\">" + figcaption + "</figure>"
	case "video":
		if IsLocalAssetPath(u) {
			return "<figure><video controls preload=\"metadata\" src=\"" + src + "\">Seu navegador não suporta vídeo HTML5.</video>" + figcaption + "</figure>"
		}
		return "<p>" + htmlLink(label("▶ Abrir vídeo"), u, "asset-link") + "</p>"
	case "audio":
		if IsLocalAssetPath(u) {
			return "<figure><audio controls preload=\"metadata\" src=\"" + src + "\">Seu navegador não suporta áudio HTML5.</audio>" + figcaption + "</figure>"
		}
		return "<p>" + htmlLink(label("▶ Abrir áudio"), u, "asset-link") + "</p>"
	case "pdf":
		return "<p>" + htmlLink(label("PDF"), u, "asset-link") + "</p>"
	}
	return "<p>" + htmlLink(label("Arquivo"), u, "asset-link") + "</p>"
}

func (c *renderContext) htmlTable(b *Block) string {
	columns, header, rows := c.tableRows(b)
	if len(columns) == 0 || len(rows) == 0 {
		return ""
	}
	var out strings.Builder
	out.WriteString("<table>")
	for i, row := range rows {
		tag := "td"
		if header && i == 0 {
			tag = "th"
			out.WriteString("<thead>")
		} else if i == 0 || header && i == 1 {
			out.WriteString("<tbody>")
		}
		out.WriteString("<tr>")
		for _, col := range columns {
			out.WriteString("<" + tag + ">" + c.htmlRich(row.Property(col)) + "</" + tag + ">")
		}
		out.WriteString("</tr>")
		if header && i == 0 {
			out.WriteString("</thead>")
		}
	}
	if !header || len(rows) > 1 {
		out.WriteString("</tbody>")
	}
	out.WriteString("</table>")
	return out.String()
}

// htmlLink builds an anchor from already-escaped label HTML and a raw URL.
func htmlLink(labelHTML, rawURL, class string) string {
	u, ok := SafeURL(rawURL)
	if !ok {
		return labelHTML
	}
	attrs := ""
	if class != "" {
		attrs = " class=\"" + esc(class) + "\""
	}
	if IsHTTPURL(u) {
		attrs += " target=\"_blank\" rel=\"noopener noreferrer\""
	}
	return "<a href=\"" + esc(u) + "\"" + attrs + ">" + labelHTML + "</a>"
}

func (c *renderContext) htmlRich(raw json.RawMessage) string {
	var b strings.Builder
	for _, seg := range ParseRichText(raw) {
		b.WriteString(c.htmlSegment(seg))
	}
	return b.String()
}

func (c *renderContext) htmlSegment(seg Segment) string {
	if latex, ok := equationOf(seg); ok && seg.Text == equationMarker {
		return "<code class=\"equation\">" + esc(latex) + "</code>"
	}
	link := linkOf(seg)
	var out string
	if mention, mentionLink, ok := c.mentionText(seg); ok {
		if mention == "" {
			return ""
		}
		out, link = esc(mention), firstNonEmpty(mentionLink, link)
	} else {
		out = strings.ReplaceAll(esc(seg.Text), "\n", "<br>")
	}
	for _, d := range seg.Decorations {
		switch d.Kind {
		case "b":
			out = "<strong>" + out + "</strong>"
		case "i":
			out = "<em>" + out + "</em>"
		case "s":
			out = "<s>" + out + "</s>"
		case "_":
			out = "<u>" + out + "</u>"
		case "c":
			out = "<code>" + out + "</code>"
		}
	}
	if link != "" {
		out = htmlLink(out, link, "")
	}
	return out
}
