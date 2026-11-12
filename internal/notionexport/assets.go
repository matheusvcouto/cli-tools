package notionexport

import (
	"fmt"
	"mime"
	"net/url"
	"path"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// AssetType groups assets for prompts, folders and Content-Type validation.
type AssetType string

const (
	AssetImage AssetType = "image"
	AssetVideo AssetType = "video"
	AssetAudio AssetType = "audio"
	AssetPDF   AssetType = "pdf"
	AssetFile  AssetType = "file"
)

// AssetTypes is the stable prompt/summary order.
var AssetTypes = []AssetType{AssetImage, AssetVideo, AssetAudio, AssetPDF, AssetFile}

func assetTypeOf(blockType string) (AssetType, bool) {
	switch AssetType(blockType) {
	case AssetImage, AssetVideo, AssetAudio, AssetPDF, AssetFile:
		return AssetType(blockType), true
	}
	return "", false
}

// Asset is a downloadable file referenced by a reachable block.
type Asset struct {
	Index         int
	BlockID       string
	Type          AssetType
	Source        string
	URL           string
	Caption       string
	EstimatedSize *int64
	ContentType   string
}

// streamingHosts are video/audio pages, not media files; they stay links.
var streamingHosts = []string{"youtube.com", "youtu.be", "youtube-nocookie.com", "vimeo.com", "loom.com", "twitch.tv", "dailymotion.com", "spotify.com", "soundcloud.com"}

func isStreamingPage(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, h := range streamingHosts {
		if host == h || strings.HasSuffix(host, "."+h) {
			return true
		}
	}
	return false
}

// assetURL is the URL used to fetch a block's file: the signed URL when
// present, else the block source when it is a direct http(s) URL.
func assetURL(rm *RecordMap, b *Block) string {
	if signed := rm.SignedURL(b.ID); IsHTTPURL(signed) {
		return signed
	}
	if source := PlainText(b.Property("source")); IsHTTPURL(source) {
		return source
	}
	return ""
}

// CollectAssets lists the downloadable assets reachable from the page in
// document order. Blocks are visited once; child pages are not traversed.
func CollectAssets(rm *RecordMap, root *Block) []Asset {
	var assets []Asset
	walkPage(rm, root, func(b *Block) {
		kind, ok := assetTypeOf(b.Type)
		if !ok {
			return
		}
		u := assetURL(rm, b)
		if u == "" || (kind == AssetVideo || kind == AssetAudio) && isStreamingPage(u) {
			return
		}
		assets = append(assets, Asset{
			Index:   len(assets) + 1,
			BlockID: b.ID,
			Type:    kind,
			Source:  PlainText(b.Property("source")),
			URL:     u,
			Caption: PlainText(b.Property("caption")),
		})
	})
	return assets
}

// walkPage visits the page content depth-first in document order, following
// synced-block references, and never descends into child pages.
func walkPage(rm *RecordMap, root *Block, visit func(*Block)) {
	seen := map[string]bool{CompactID(root.ID): true}
	var walk func(ids []string, depth int)
	walk = func(ids []string, depth int) {
		if depth > maxRenderDepth {
			return
		}
		for _, id := range ids {
			b, ok := rm.Block(id)
			if !ok || !b.Alive || seen[CompactID(b.ID)] {
				continue
			}
			seen[CompactID(b.ID)] = true
			visit(b)
			if b.Type == "page" || b.Type == "collection_view_page" {
				continue
			}
			if ref := b.FormatPointerID("transclusion_reference_pointer"); b.Type == "transclusion_reference" && ref != "" {
				if target, ok := rm.Block(ref); ok {
					walk(target.Content, depth+1)
				}
			}
			walk(b.Content, depth+1)
		}
	}
	walk(root.Content, 0)
}

// AssetDir is the folder below assets/ for an asset type.
func AssetDir(t AssetType) string {
	switch t {
	case AssetImage:
		return "images"
	case AssetVideo:
		return "videos"
	case AssetAudio:
		return "audio"
	case AssetPDF:
		return "pdfs"
	}
	return "files"
}

// OriginalFilename derives the human filename from a source or URL.
func OriginalFilename(source string) string {
	if rest, ok := strings.CutPrefix(source, "attachment:"); ok {
		if i := strings.LastIndexByte(rest, ':'); i >= 0 {
			rest = rest[i+1:]
		}
		if rest != "" {
			return rest
		}
		return "asset"
	}
	if u, err := url.Parse(source); err == nil && u.Path != "" {
		if base := path.Base(u.Path); base != "/" && base != "." {
			return base
		}
	}
	return "asset"
}

// maxAssetNameBytes keeps names well under common 255-byte component limits
// after adding the index prefix and a content-type extension.
const maxAssetNameBytes = 100

// SafeAssetName returns "<index>-<portable-name>": ASCII letters, digits,
// '.', '_' and '-' only, without leading/trailing dots or dashes, so it is a
// valid, non-reserved filename on every supported OS. The index prefix keeps
// names unique within an export.
func SafeAssetName(source string, index int) string {
	return fmt.Sprintf("%03d-%s", index, portableName(OriginalFilename(source)))
}

func portableName(raw string) string {
	ext := path.Ext(raw)
	base := portableComponent(strings.TrimSuffix(raw, ext))
	ext = portableComponent(ext)
	if ext != "" {
		ext = "." + ext
	}
	if len(ext) > 16 {
		ext = ""
	}
	if base == "" {
		base = "asset"
	}
	if len(base)+len(ext) > maxAssetNameBytes {
		base = strings.TrimRight(base[:maxAssetNameBytes-len(ext)], "-._")
	}
	return base + ext
}

// portableComponent keeps ASCII letters/digits and '.', '_', '-', folds
// accents, collapses other runs into '-' and trims separators at both ends.
func portableComponent(raw string) string {
	var b strings.Builder
	dash := false
	for _, r := range norm.NFKD.String(raw) {
		switch {
		case unicode.Is(unicode.Mn, r):
			continue
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_'):
			b.WriteRune(r)
			dash = false
		default:
			if !dash {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.Trim(b.String(), "-.")
}

var contentTypeExtensions = map[string]string{
	"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp", "image/gif": ".gif",
	"image/svg+xml": ".svg", "image/avif": ".avif", "image/heic": ".heic",
	"video/mp4": ".mp4", "video/webm": ".webm", "video/quicktime": ".mov",
	"audio/mpeg": ".mp3", "audio/mp4": ".m4a", "audio/ogg": ".ogg", "audio/wav": ".wav", "audio/x-wav": ".wav", "audio/webm": ".weba",
	"application/pdf": ".pdf",
}

// ExtensionForContentType returns a filename extension for known media types.
func ExtensionForContentType(contentType string) string {
	return contentTypeExtensions[MediaType(contentType)]
}

// MediaType returns the lowercase media type without parameters.
func MediaType(contentType string) string {
	if contentType == "" {
		return ""
	}
	mt, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mt, _, _ = strings.Cut(contentType, ";")
	}
	return strings.ToLower(strings.TrimSpace(mt))
}

// CheckContentType rejects a response whose declared type contradicts the
// asset type, e.g. an HTML error page saved as a video. Missing and generic
// binary types are accepted because storage backends often omit them.
func CheckContentType(t AssetType, contentType string) error {
	mt := MediaType(contentType)
	if mt == "" || mt == "application/octet-stream" || mt == "binary/octet-stream" {
		return nil
	}
	ok := true
	switch t {
	case AssetImage:
		ok = strings.HasPrefix(mt, "image/")
	case AssetVideo:
		ok = strings.HasPrefix(mt, "video/")
	case AssetAudio:
		ok = strings.HasPrefix(mt, "audio/")
	case AssetPDF:
		ok = mt == "application/pdf"
	}
	if !ok {
		return fmt.Errorf("Content-Type incompatível para %s: %s", t, mt)
	}
	return nil
}
