package notionexport

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestParsePageIDAcceptsURLsUUIDsAndBareIDs(t *testing.T) {
	const id = "0123456789abcdef0123456789abcdef"
	for _, in := range []string{
		id,
		"0123456789ABCDEF0123456789ABCDEF",
		"01234567-89ab-cdef-0123-456789abcdef",
		"https://app.notion.com/p/your-public-page-slug-" + id,
		"https://example.notion.site/Some-Page-" + id + "?pvs=4",
		"https://www.notion.so/workspace/Page-" + id + "#heading",
		"https://www.notion.so/01234567-89ab-cdef-0123-456789abcdef",
		"  " + id + "\n",
	} {
		got, err := ParsePageID(in)
		if err != nil || got != id {
			t.Errorf("ParsePageID(%q) = %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "not-a-page", "0123456789abcdef", id + "00", "https://example.com/?p=" + id} {
		if got, err := ParsePageID(in); !errors.Is(err, ErrInvalidPageID) {
			t.Errorf("ParsePageID(%q) = %q, %v; want ErrInvalidPageID", in, got, err)
		}
	}
	if UUID(id) != "01234567-89ab-cdef-0123-456789abcdef" || UUID("x") != "x" {
		t.Fatal(UUID(id))
	}
}

func TestDecodeRecordMapEnvelopesAndValidation(t *testing.T) {
	f := newFixture(t, "Título", testID(2))
	f.block(testID(2), "text", map[string]any{"title": text("olá")})
	rm := f.recordMap()
	b, ok := rm.Block(strings.ReplaceAll(testID(2), "-", ""))
	if !ok || b.Type != "text" || PlainText(b.Property("title")) != "olá" {
		t.Fatalf("nested envelope not unwrapped: %+v", b)
	}
	// loadPageChunk wrapper and trailing data.
	wrapped, _ := json.Marshal(map[string]any{"recordMap": map[string]any{"block": f.blocks}})
	if _, err := DecodeRecordMap(bytes.NewReader(wrapped)); err != nil {
		t.Fatal(err)
	}
	for name, input := range map[string]string{
		"trailing":  `{"block":{"a":{}}} {}`,
		"array":     `[]`,
		"null":      `null`,
		"no blocks": `{"block":{}}`,
		"bad block": `{"block":[]}`,
		"bad urls":  `{"block":{"a":{}},"signed_urls":[1]}`,
	} {
		if _, err := DecodeRecordMap(strings.NewReader(input)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestUnauthorizedEnvelopeDoesNotShadowBlock(t *testing.T) {
	rm := NewRecordMap()
	rm.PutBlock(testID(5), json.RawMessage(`{"value":{"value":{"id":"`+testID(5)+`","type":"text"}}}`))
	rm.PutBlock(testID(5), json.RawMessage(`{"value":{"role":"none"}}`))
	if !rm.HasBlock(testID(5)) {
		t.Fatal("permission-less envelope replaced a usable block")
	}
}

func TestDeepEnvelopeIsBounded(t *testing.T) {
	raw := `{"id":"x","type":"text"}`
	for range maxUnwrapDepth + 1 {
		raw = `{"value":` + raw + `}`
	}
	if _, ok := parseBlock(json.RawMessage(raw)); ok {
		t.Fatal("unwrap depth not bounded")
	}
}

func TestRootPageSelection(t *testing.T) {
	f := newFixture(t, "Raiz", testID(2))
	f.block(testID(2), "text", nil)
	rm := f.recordMap()
	if _, err := rm.RootPage(testID(99)); !errors.Is(err, ErrPageNotFound) {
		t.Fatal(err)
	}
	if _, err := rm.RootPage(testID(2)); !errors.Is(err, ErrPageNotFound) {
		t.Fatal("non-page block accepted as root")
	}
	// A saved page.json also carries the ancestor page (workspace top level)
	// whose content is not loaded; the exported page has its content present.
	g := &fixture{t: t, blocks: map[string]any{}, signed: map[string]string{}}
	g.put(map[string]any{"id": testID(10), "type": "page", "parent_table": "space", "content": []string{testID(11), testID(12), testID(13)}})
	g.put(map[string]any{"id": testID(11), "type": "page", "parent_table": "block", "parent_id": testID(10), "content": []string{testID(20), testID(21)}})
	g.block(testID(20), "text", nil)
	g.block(testID(21), "text", nil)
	got, err := g.recordMap().RootPage("")
	if err != nil || CompactID(got.ID) != CompactID(testID(11)) {
		t.Fatalf("root = %+v, %v", got, err)
	}
}

func TestSignedURLLookupAcceptsBothIDForms(t *testing.T) {
	rm := NewRecordMap()
	rm.SetSignedURL(testID(3), "https://files.example/a")
	if rm.SignedURL(CompactID(testID(3))) != "https://files.example/a" || rm.SignedURL(testID(3)) == "" {
		t.Fatal("signed URL lookup failed")
	}
}

func TestSafeURL(t *testing.T) {
	for _, tc := range []struct {
		in, want string
		ok       bool
	}{
		{"https://example.com/a?b=1", "https://example.com/a?b=1", true},
		{"HTTP://example.com", "HTTP://example.com", true},
		{"mailto:someone@example.com", "mailto:someone@example.com", true},
		{"#section", "#section", true},
		{"assets/images/001-a.png", "assets/images/001-a.png", true},
		{"/0123456789abcdef0123456789abcdef", "https://www.notion.so/0123456789abcdef0123456789abcdef", true},
		{"/Some-Page-0123456789abcdef0123456789abcdef#h", "https://www.notion.so/0123456789abcdef0123456789abcdef#h", true},
		{"javascript:alert(1)", "", false},
		{" JaVaScRiPt:alert(1)", "", false},
		{"java\tscript:alert(1)", "", false},
		{"data:text/html,<script>", "", false},
		{"vbscript:x", "", false},
		{"//evil.example/x", "", false},
		{`\\evil\share`, "", false},
		{"/etc/passwd", "", false},
		{"../secret", "", false},
		{"assets/../../x", "", false},
		{"https://user:pass@example.com", "", false},
		{"https://", "", false},
		{"mailto:", "", false},
		{"https://example.com/‮", "", false},
		{"", "", false},
	} {
		got, ok := SafeURL(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("SafeURL(%q) = %q, %v; want %q, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
	if !IsLocalAssetPath("assets/videos/001.mp4") || IsLocalAssetPath("https://x/assets/a") || IsLocalAssetPath("assets/../x") {
		t.Fatal("IsLocalAssetPath")
	}
}

func TestCollectAssetsOrderAndFiltering(t *testing.T) {
	f := newFixture(t, "Assets", testID(2), testID(3), testID(4), testID(5), testID(6), testID(7), testID(8))
	f.block(testID(2), "image", map[string]any{"source": text("attachment:abc:foto.png"), "caption": text("Legenda")})
	f.signed[testID(2)] = "https://file.example/f/foto.png?signature=x"
	f.block(testID(3), "video", map[string]any{"source": text("https://www.youtube.com/watch?v=x")})
	f.block(testID(4), "file", map[string]any{"source": text("attachment:def:unsigned.zip")})
	f.block(testID(5), "column_list", nil, testID(9))
	f.block(testID(9), "pdf", map[string]any{"source": text("https://cdn.example/doc.pdf")})
	f.block(testID(6), "page", map[string]any{"title": text("Filha")}, testID(10))
	f.block(testID(10), "image", map[string]any{"source": text("https://cdn.example/child.png")})
	f.block(testID(7), "transclusion_reference", nil)
	f.withFormat(testID(7), map[string]any{"transclusion_reference_pointer": map[string]any{"id": testID(11)}})
	f.block(testID(11), "transclusion_container", nil, testID(12))
	f.block(testID(12), "audio", map[string]any{"source": text("https://cdn.example/a.mp3")})
	f.block(testID(8), "image", map[string]any{"source": text("https://cdn.example/again.png")})
	rm := f.recordMap()
	assets := CollectAssets(rm, f.root(rm))
	var got []string
	for _, a := range assets {
		got = append(got, string(a.Type)+":"+OriginalFilename(firstNonEmpty(a.Source, a.URL)))
	}
	want := "image:foto.png pdf:doc.pdf audio:a.mp3 image:again.png"
	if strings.Join(got, " ") != want {
		t.Fatalf("assets = %v, want %s", got, want)
	}
	for i, a := range assets {
		if a.Index != i+1 {
			t.Fatal("indexes must be sequential")
		}
	}
	if assets[0].URL != "https://file.example/f/foto.png?signature=x" || assets[0].Caption != "Legenda" {
		t.Fatalf("signed URL/caption: %+v", assets[0])
	}
}

func TestSafeAssetNameIsPortable(t *testing.T) {
	for _, tc := range []struct {
		source string
		want   string
	}{
		{"https://x/a/Relatório Final.pdf?x=1", "007-Relatorio-Final.pdf"},
		{"attachment:abc:../../etc/passwd", "007-etc-passwd"},
		{`https://x/a%2F..%2F..%2Fevil.png`, "007-evil.png"},
		{"https://x/", "007-asset"},
		{"https://x/...", "007-asset"},
		{"https://x/CON", "007-CON"},
		{"https://x/a:b*c?.png", "007-a-b-c"},
		{"https://x/日本語.png", "007-asset.png"}, {"https://x/.env", "007-asset.env"}, {"https://x/a.tar.gz", "007-a.tar.gz"},
		{"https://x/" + strings.Repeat("a", 300) + ".jpeg", "007-" + strings.Repeat("a", maxAssetNameBytes-5) + ".jpeg"},
	} {
		got := SafeAssetName(tc.source, 7)
		if got != tc.want {
			t.Errorf("SafeAssetName(%q) = %q, want %q", tc.source, got, tc.want)
		}
		if strings.ContainsAny(got, `/\:*?"<>|`) || strings.HasSuffix(got, ".") || len(got) > 120 {
			t.Errorf("unportable name %q", got)
		}
	}
}

func TestContentTypeRules(t *testing.T) {
	ok := map[AssetType][]string{
		AssetImage: {"image/png", "IMAGE/JPEG; charset=x", "", "application/octet-stream"},
		AssetVideo: {"video/mp4"},
		AssetAudio: {"audio/mpeg"},
		AssetPDF:   {"application/pdf"},
		AssetFile:  {"text/html", "application/zip"},
	}
	for typ, values := range ok {
		for _, v := range values {
			if err := CheckContentType(typ, v); err != nil {
				t.Errorf("%s %q: %v", typ, v, err)
			}
		}
	}
	for typ, v := range map[AssetType]string{AssetImage: "text/html", AssetVideo: "text/html; charset=utf-8", AssetAudio: "video/mp4", AssetPDF: "image/png"} {
		if err := CheckContentType(typ, v); err == nil {
			t.Errorf("%s accepted %q", typ, v)
		}
	}
	if ExtensionForContentType("image/jpeg; q=1") != ".jpg" || ExtensionForContentType("x/y") != "" {
		t.Fatal("extension map")
	}
}

func TestSlugify(t *testing.T) {
	for in, want := range map[string]string{
		"Minha Página — Ação!":    "minha-pagina-acao",
		"":                        "notion-page",
		"..":                      "notion-page",
		"日本語":                     "notion-page",
		"../../etc":               "etc",
		strings.Repeat("ab ", 60): strings.TrimRight(strings.Repeat("ab-", 27), "-"),
	} {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRichTextIgnoresMalformedParts(t *testing.T) {
	raw := json.RawMessage(`[["a",[["b"],[1],["a","https://x"]]],"bad",[1],[],["‣",[["p","x"]]],["c"]]`)
	segs := ParseRichText(raw)
	if len(segs) != 3 || len(segs[0].Decorations) != 2 || PlainText(raw) != "ac" {
		t.Fatalf("segments = %+v", segs)
	}
}
