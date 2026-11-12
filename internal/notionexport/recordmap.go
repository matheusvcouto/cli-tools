package notionexport

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
)

// MaxRecordMapBytes bounds a saved/received recordMap. It protects the process
// from unbounded memory use while leaving ample room for large public pages.
const MaxRecordMapBytes = 256 << 20

// maxUnwrapDepth mirrors the defensive limit for Notion's nested
// {"value":{"value":{...}}} record envelopes.
const maxUnwrapDepth = 12

// ErrPageNotFound reports a recordMap without the requested page block.
var ErrPageNotFound = errors.New("página não encontrada no recordMap")

// Block is the subset of a Notion block that the exporter understands.
// Properties and Format keep their raw JSON so renderers decide what to trust.
type Block struct {
	ID          string
	Type        string
	Content     []string
	ParentTable string
	ParentID    string
	Alive       bool
	Properties  map[string]json.RawMessage
	Format      map[string]json.RawMessage
}

// RecordMap is the Notion page snapshot used by every renderer. Raw tables are
// preserved byte for byte for page.json; parsed blocks are indexed by compact ID.
type RecordMap struct {
	blocks     map[string]json.RawMessage
	signedURLs map[string]string
	extra      map[string]json.RawMessage
	parsed     map[string]*Block
}

// NewRecordMap returns an empty, writable recordMap.
func NewRecordMap() *RecordMap {
	return &RecordMap{blocks: map[string]json.RawMessage{}, signedURLs: map[string]string{}, extra: map[string]json.RawMessage{}, parsed: map[string]*Block{}}
}

// DecodeRecordMap decodes a recordMap saved by this tool, by the previous Deno
// exporter or returned by Notion. It rejects trailing data and oversized input;
// a block that cannot be understood is kept raw but ignored by renderers.
func DecodeRecordMap(r io.Reader) (*RecordMap, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxRecordMapBytes+1))
	if err != nil {
		return nil, fmt.Errorf("ler recordMap: %w", err)
	}
	if len(data) > MaxRecordMapBytes {
		return nil, fmt.Errorf("recordMap excede %d bytes", MaxRecordMapBytes)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	var top map[string]json.RawMessage
	if err := dec.Decode(&top); err != nil {
		return nil, fmt.Errorf("recordMap JSON inválido: %w", err)
	}
	if top == nil {
		return nil, errors.New("recordMap deve ser um objeto JSON")
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("recordMap contém dados após o objeto JSON")
	}
	// A loadPageChunk response wraps the map in {"recordMap": {...}}.
	if inner, ok := top["recordMap"]; ok && top["block"] == nil {
		return DecodeRecordMap(bytes.NewReader(inner))
	}
	rm := NewRecordMap()
	for key, raw := range top {
		switch key {
		case "block":
			var blocks map[string]json.RawMessage
			if err := json.Unmarshal(raw, &blocks); err != nil {
				return nil, fmt.Errorf("recordMap.block inválido: %w", err)
			}
			for id, value := range blocks {
				rm.PutBlock(id, value)
			}
		case "signed_urls":
			if isJSONNull(raw) {
				continue
			}
			var signed map[string]string
			if err := json.Unmarshal(raw, &signed); err != nil {
				return nil, fmt.Errorf("recordMap.signed_urls inválido: %w", err)
			}
			for k, v := range signed {
				rm.signedURLs[k] = v
			}
		default:
			rm.extra[key] = raw
		}
	}
	if len(rm.blocks) == 0 {
		return nil, errors.New("recordMap não contém blocos")
	}
	return rm, nil
}

func isJSONNull(raw json.RawMessage) bool { return bytes.Equal(bytes.TrimSpace(raw), []byte("null")) }

// PutBlock stores one raw block envelope. A later value replaces the earlier one.
func (rm *RecordMap) PutBlock(key string, raw json.RawMessage) {
	block, ok := parseBlock(raw)
	if ok {
		// Envelopes keyed by a different ID are indexed by the block's own ID.
		rm.parsed[CompactID(block.ID)] = block
	}
	if !ok {
		// Keep unknown/unauthorized envelopes, but do not let them shadow a
		// block already received under the same key.
		if _, exists := rm.blocks[key]; exists {
			return
		}
	}
	rm.blocks[key] = append(json.RawMessage(nil), raw...)
}

// SetSignedURL records a temporary signed URL by block ID and/or source URL,
// matching notion-client's signed_urls layout.
func (rm *RecordMap) SetSignedURL(key, url string) { rm.signedURLs[key] = url }

// BlockCount reports the number of raw block envelopes.
func (rm *RecordMap) BlockCount() int { return len(rm.blocks) }

// SignedURLCount reports the number of signed URL entries.
func (rm *RecordMap) SignedURLCount() int { return len(rm.signedURLs) }

// Block returns a parsed block by compact or hyphenated ID.
func (rm *RecordMap) Block(id string) (*Block, bool) {
	b, ok := rm.parsed[CompactID(id)]
	return b, ok
}

// HasBlock reports whether a usable block with this ID is present.
func (rm *RecordMap) HasBlock(id string) bool {
	_, ok := rm.Block(id)
	return ok
}

// SignedURL returns the signed URL for a block ID, accepting either ID form.
func (rm *RecordMap) SignedURL(blockID string) string {
	if v := rm.signedURLs[blockID]; v != "" {
		return v
	}
	if v := rm.signedURLs[UUID(blockID)]; v != "" {
		return v
	}
	compact := CompactID(blockID)
	for _, key := range slices.Sorted(maps.Keys(rm.signedURLs)) {
		if CompactID(key) == compact {
			return rm.signedURLs[key]
		}
	}
	return ""
}

// RootPage finds the exported page. With preferredID it must be that page.
// Without it (a saved page.json), Notion's chunk also carries ancestor pages,
// so the exported page is the one whose content is loaded: most child blocks
// present, then a parent outside the map, then sorted ID for determinism.
func (rm *RecordMap) RootPage(preferredID string) (*Block, error) {
	if preferredID != "" {
		if b, ok := rm.Block(preferredID); ok && b.Type == "page" {
			return b, nil
		}
		return nil, fmt.Errorf("%w: %s (a página pode não ser pública)", ErrPageNotFound, CompactID(preferredID))
	}
	var best *Block
	bestLoaded, bestDetached := -1, false
	for _, id := range slices.Sorted(maps.Keys(rm.parsed)) {
		b := rm.parsed[id]
		if b.Type != "page" {
			continue
		}
		loaded := 0
		for _, child := range b.Content {
			if rm.HasBlock(child) {
				loaded++
			}
		}
		detached := !(b.ParentTable == "block" && rm.HasBlock(b.ParentID))
		if loaded > bestLoaded || loaded == bestLoaded && detached && !bestDetached {
			best, bestLoaded, bestDetached = b, loaded, detached
		}
	}
	if best == nil {
		return nil, fmt.Errorf("%w: nenhum bloco de página", ErrPageNotFound)
	}
	return best, nil
}

// MarshalJSON writes a deterministic page.json compatible with DecodeRecordMap.
func (rm *RecordMap) MarshalJSON() ([]byte, error) {
	top := make(map[string]any, len(rm.extra)+2)
	for k, v := range rm.extra {
		top[k] = v
	}
	top["block"] = rm.blocks
	top["signed_urls"] = rm.signedURLs
	return json.Marshal(top)
}

func parseBlock(raw json.RawMessage) (*Block, bool) {
	current := raw
	for range maxUnwrapDepth {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(current, &object); err != nil || object == nil {
			return nil, false
		}
		var id, typ string
		if json.Unmarshal(object["id"], &id) == nil && json.Unmarshal(object["type"], &typ) == nil && id != "" && typ != "" {
			return blockFromObject(id, typ, object), true
		}
		next, ok := object["value"]
		if !ok || len(bytes.TrimSpace(next)) == 0 || bytes.TrimSpace(next)[0] != '{' {
			return nil, false
		}
		current = next
	}
	return nil, false
}

func blockFromObject(id, typ string, object map[string]json.RawMessage) *Block {
	b := &Block{ID: id, Type: typ, Alive: true}
	var content []json.RawMessage
	if json.Unmarshal(object["content"], &content) == nil {
		for _, item := range content {
			var child string
			if json.Unmarshal(item, &child) == nil && child != "" {
				b.Content = append(b.Content, child)
			}
		}
	}
	_ = json.Unmarshal(object["parent_table"], &b.ParentTable)
	_ = json.Unmarshal(object["parent_id"], &b.ParentID)
	var alive bool
	if json.Unmarshal(object["alive"], &alive) == nil {
		b.Alive = alive
	}
	_ = json.Unmarshal(object["properties"], &b.Properties)
	_ = json.Unmarshal(object["format"], &b.Format)
	return b
}

// Property returns a raw property value.
func (b *Block) Property(name string) json.RawMessage {
	if b == nil || b.Properties == nil {
		return nil
	}
	return b.Properties[name]
}

// FormatString returns a string format attribute, or "".
func (b *Block) FormatString(name string) string {
	if b == nil || b.Format == nil {
		return ""
	}
	var s string
	_ = json.Unmarshal(b.Format[name], &s)
	return s
}

// FormatBool returns a boolean format attribute, or false.
func (b *Block) FormatBool(name string) bool {
	if b == nil || b.Format == nil {
		return false
	}
	var v bool
	_ = json.Unmarshal(b.Format[name], &v)
	return v
}

// FormatStrings returns a string-array format attribute.
func (b *Block) FormatStrings(name string) []string {
	if b == nil || b.Format == nil {
		return nil
	}
	var v []string
	_ = json.Unmarshal(b.Format[name], &v)
	return v
}

// FormatPointerID returns format.<name>.id for pointer attributes such as
// alias_pointer and transclusion_reference_pointer.
func (b *Block) FormatPointerID(name string) string {
	if b == nil || b.Format == nil {
		return ""
	}
	var p struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(b.Format[name], &p)
	return p.ID
}
