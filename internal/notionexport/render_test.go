package notionexport

import (
	"strings"
	"testing"
)

func richFixture(t *testing.T) (*fixture, *RecordMap) {
	f := newFixture(t, `Página <script>alert(1)</script> & "aspas"`,
		testID(2), testID(3), testID(4), testID(5), testID(6), testID(7), testID(8), testID(9), testID(10), testID(11), testID(12), testID(13), testID(14), testID(15))
	f.block(testID(2), "text", map[string]any{"title": []any{
		[]any{"negrito ", []any{[]any{"b"}}},
		[]any{"link", []any{[]any{"a", "https://example.com/a b"}}},
		[]any{" ruim", []any{[]any{"a", "javascript:alert(1)"}}},
		[]any{" <img src=x onerror=alert(1)> *lit* # x"},
	}})
	f.block(testID(3), "header", map[string]any{"title": text("# Título")})
	f.block(testID(4), "bulleted_list", map[string]any{"title": text("um")}, testID(16))
	f.block(testID(16), "bulleted_list", map[string]any{"title": text("aninhado")})
	f.block(testID(5), "bulleted_list", map[string]any{"title": text("dois")})
	f.block(testID(6), "numbered_list", map[string]any{"title": text("primeiro")})
	f.block(testID(7), "numbered_list", map[string]any{"title": text("segundo")})
	f.block(testID(8), "to_do", map[string]any{"title": text("feito"), "checked": text("Yes")})
	f.block(testID(9), "code", map[string]any{"title": text("a ``` b </code><script>"), "language": text("Plain Text")})
	f.block(testID(10), "embed", map[string]any{"source": text("https://evil.example/frame")})
	f.block(testID(11), "image", map[string]any{"source": text("https://cdn.example/x.png"), "caption": text(`"><script>x</script>`)})
	f.block(testID(12), "video", map[string]any{"source": text("https://cdn.example/v.mp4")})
	f.block(testID(13), "table", nil, testID(17), testID(18))
	f.withFormat(testID(13), map[string]any{"table_block_column_order": []string{"c1", "c2"}, "table_block_column_header": true})
	f.block(testID(17), "table_row", map[string]any{"c1": text("A|B"), "c2": text("C")})
	f.block(testID(18), "table_row", map[string]any{"c1": text("1"), "c2": text("2")})
	// Synced block whose reference points back at itself must not loop.
	f.block(testID(14), "transclusion_container", nil, testID(19))
	f.block(testID(19), "transclusion_reference", nil)
	f.withFormat(testID(19), map[string]any{"transclusion_reference_pointer": map[string]any{"id": testID(14)}})
	f.block(testID(15), "text", map[string]any{"title": []any{[]any{"‣", []any{[]any{"p", testID(16)}}}, []any{"⁍", []any{[]any{"e", "x^2"}}}}})
	return f, f.recordMap()
}

func TestMarkdownEscapesAndStructures(t *testing.T) {
	f, rm := richFixture(t)
	resolve := func(id, remote string) string {
		if CompactID(id) == CompactID(testID(12)) {
			return "assets/videos/001-v.mp4"
		}
		return remote
	}
	md := RenderMarkdown(rm, f.root(rm), resolve)
	for _, want := range []string{
		"# Página \\<script\\>alert(1)\\</script\\> & \"aspas\"\n",
		"**negrito** [link](https://example.com/a%20b) ruim \\<img src=x onerror=alert(1)\\> \\*lit\\* # x",
		"## \\# Título",
		"- um\n  - aninhado\n- dois",
		"1. primeiro\n2. segundo",
		"- [x] feito",
		"````\na ``` b </code><script>\n````",
		"[Conteúdo incorporado](https://evil.example/frame)",
		"[▶ Vídeo](assets/videos/001-v.mp4)",
		"| A\\|B | C |\n| --- | --- |\n| 1 | 2 |",
		"[aninhado](https://www.notion.so/" + CompactID(testID(16)) + ")$x^2$",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q in:\n%s", want, md)
		}
	}
	if strings.Contains(md, "javascript:") {
		t.Fatal("unsafe link rendered")
	}
}

func TestHTMLEscapesAndHasNoActiveContent(t *testing.T) {
	f, rm := richFixture(t)
	resolve := func(id, remote string) string {
		switch CompactID(id) {
		case CompactID(testID(12)):
			return "assets/videos/001-v.mp4"
		case CompactID(testID(11)):
			return "javascript:alert(1)" // a broken resolver falls back to the safe remote URL
		}
		return remote
	}
	doc := RenderHTML(rm, f.root(rm), resolve, `gen "x" <y>`)
	lower := strings.ToLower(doc)
	for _, banned := range []string{"<script", "<iframe", "javascript:", "<img src=x", "<object", "<form"} {
		if strings.Contains(lower, banned) {
			t.Errorf("HTML contains %q:\n%s", banned, doc)
		}
	}
	for _, want := range []string{
		`<meta http-equiv="Content-Security-Policy" content="default-src &#39;none&#39;;`,
		`<meta name="referrer" content="no-referrer">`,
		`<meta name="generator" content="gen &#34;x&#34; &lt;y&gt;">`,
		`<title>Página &lt;script&gt;alert(1)&lt;/script&gt; &amp; &#34;aspas&#34;</title>`,
		`<link rel="stylesheet" href="styles.css">`,
		`<strong>negrito </strong><a href="https://example.com/a b" target="_blank" rel="noopener noreferrer">link</a> ruim &lt;img`,
		`<ul><li>um` + "\n" + `<ul><li>aninhado</li></ul></li><li>dois</li></ul>`,
		`<ol><li>primeiro</li><li>segundo</li></ol>`,
		`<input type="checkbox" disabled checked>`,
		"<pre><code>a ``` b &lt;/code&gt;&lt;script&gt;</code></pre>",
		`<a href="https://evil.example/frame" class="asset-link"`,
		`<img src="https://cdn.example/x.png" alt="&#34;&gt;&lt;script&gt;x&lt;/script&gt;"`,
		`<video controls preload="metadata" src="assets/videos/001-v.mp4">`,
		`<thead><tr><th>A|B</th><th>C</th></tr></thead><tbody><tr><td>1</td><td>2</td></tr></tbody>`,
		`<code class="equation">x^2</code>`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("HTML missing %q", want)
		}
	}
}

func TestRemoteMediaStaysLinkAndCSSIsLocal(t *testing.T) {
	f := newFixture(t, "M", testID(2), testID(3))
	f.block(testID(2), "video", map[string]any{"source": text("https://cdn.example/v.mp4")})
	f.block(testID(3), "audio", map[string]any{"source": text("ftp://cdn.example/a.mp3")})
	rm := f.recordMap()
	doc := RenderHTML(rm, f.root(rm), nil, "g")
	if strings.Contains(doc, "<video") || !strings.Contains(doc, `href="https://cdn.example/v.mp4"`) || strings.Contains(doc, "ftp:") {
		t.Fatal(doc)
	}
	css := strings.ToLower(PageCSS)
	if strings.Contains(css, "@import") || strings.Contains(css, "url(") || strings.Contains(css, "http") {
		t.Fatal("styles.css must not load external resources")
	}
}

func TestRenderDepthIsBounded(t *testing.T) {
	ids := []string{}
	for i := range maxRenderDepth + 10 {
		ids = append(ids, testID(100+i))
	}
	f := newFixture(t, "Fundo", ids[0])
	for i, id := range ids {
		var children []string
		if i+1 < len(ids) {
			children = []string{ids[i+1]}
		}
		f.block(id, "toggle", map[string]any{"title": text("n")}, children...)
	}
	rm := f.recordMap()
	md := RenderMarkdown(rm, f.root(rm), nil)
	if got := strings.Count(md, "- n"); got == 0 || got > maxRenderDepth+2 {
		t.Fatalf("depth not bounded: %d", got)
	}
	_ = RenderHTML(rm, f.root(rm), nil, "g")
}

func TestMarkdownLineStartAndCodeSpans(t *testing.T) {
	for in, want := range map[string]string{
		"1. não é lista": "1\\. não é lista",
		"- nem isso":     "\\- nem isso",
		"===":            "\\===",
		"a\nb":           "a\\\nb",
	} {
		f := newFixture(t, "T", testID(2))
		f.block(testID(2), "text", map[string]any{"title": text(in)})
		rm := f.recordMap()
		if md := RenderMarkdown(rm, f.root(rm), nil); !strings.Contains(md, want) {
			t.Errorf("%q => %q, want %q", in, md, want)
		}
	}
	if mdCodeSpan("a`b") != "``a`b``" || mdCodeSpan("`x") != "`` `x ``" {
		t.Fatal(mdCodeSpan("a`b"), mdCodeSpan("`x"))
	}
	if mdWrap(" bold ", "**") != " **bold** " {
		t.Fatal(mdWrap(" bold ", "**"))
	}
}
