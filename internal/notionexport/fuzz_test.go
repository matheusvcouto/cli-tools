package notionexport

import (
	"bytes"
	"net/url"
	"strings"
	"testing"
)

func FuzzRenderNeverPanicsOrEmitsActiveHTML(f *testing.F) {
	f.Add([]byte(`{"block":{"r":{"value":{"value":{"id":"r","type":"page","properties":{"title":[["T"]]},"content":["a","b","r"]}}},"a":{"id":"a","type":"text","properties":{"title":[["<script>x</script>",[["a","javascript:1"],["b"]]]]},"content":["a"]},"b":{"id":"b","type":"transclusion_reference","format":{"transclusion_reference_pointer":{"id":"r"}}}}}`))
	f.Add([]byte(`{"block":{"p":{"id":"p","type":"page","content":["t"]},"t":{"id":"t","type":"table","format":{"table_block_column_order":["x"]},"content":["t"]}}}`))
	f.Add([]byte(`{"recordMap":{"block":{"p":{"id":"p","type":"page","content":["i"]},"i":{"id":"i","type":"image","properties":{"source":[["data:text/html,<script>"]],"caption":[["\"><svg onload=1>"]]}}}}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		rm, err := DecodeRecordMap(bytes.NewReader(data))
		if err != nil {
			return
		}
		root, err := rm.RootPage("")
		if err != nil {
			return
		}
		_ = CollectAssets(rm, root)
		_ = RenderMarkdown(rm, root, nil)
		doc := strings.ToLower(RenderHTML(rm, root, nil, "fuzz"))
		for _, banned := range []string{"<script", "<iframe", "<object", "<embed", "<svg", "javascript:", "data:text"} {
			if strings.Contains(doc, banned) {
				t.Fatalf("HTML contains %q", banned)
			}
		}
		if _, err := rm.MarshalJSON(); err != nil {
			t.Fatal(err)
		}
	})
}

func FuzzSafeURLOnlyAllowsSafeTargets(f *testing.F) {
	for _, s := range []string{"https://x", "javascript:alert(1)", "//x", "assets/a.png", "/0123456789abcdef0123456789abcdef", "mailto:a@b", "#x", " data:x", "../a"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		out, ok := SafeURL(in)
		if !ok {
			return
		}
		if strings.HasPrefix(out, "//") || strings.HasPrefix(out, `\`) {
			t.Fatalf("protocol-relative accepted: %q", out)
		}
		if hasScheme(out) {
			u, err := url.Parse(out)
			if err != nil {
				t.Fatalf("unparsable accepted: %q", out)
			}
			switch strings.ToLower(u.Scheme) {
			case "http", "https", "mailto":
			default:
				t.Fatalf("scheme %q accepted from %q", u.Scheme, in)
			}
		} else if !strings.HasPrefix(out, "#") {
			if _, rel := safeRelative(out); !rel || strings.HasPrefix(out, "/") {
				t.Fatalf("unsafe relative accepted: %q", out)
			}
		}
		_ = SafeAssetName(in, 1)
		_ = Slugify(in)
	})
}
