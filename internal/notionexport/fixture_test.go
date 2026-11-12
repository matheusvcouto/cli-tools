package notionexport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
)

// Synthetic IDs: fixtures are born synthetic and never copied from real pages.
func testID(n int) string { return fmt.Sprintf("%08x-0000-4000-8000-%012x", n, n) }

const rootID = "00000001-0000-4000-8000-000000000001"

type fixture struct {
	t      *testing.T
	blocks map[string]any
	signed map[string]string
}

func newFixture(t *testing.T, title string, children ...string) *fixture {
	f := &fixture{t: t, blocks: map[string]any{}, signed: map[string]string{}}
	f.put(map[string]any{"id": rootID, "type": "page", "parent_table": "space", "properties": map[string]any{"title": [][]any{{title}}}, "content": children})
	return f
}

// put stores a block using Notion's doubly nested envelope.
func (f *fixture) put(block map[string]any) {
	f.blocks[block["id"].(string)] = map[string]any{"spaceId": "s", "value": map[string]any{"value": block, "role": "reader"}}
}

func (f *fixture) block(id, typ string, props map[string]any, children ...string) string {
	b := map[string]any{"id": id, "type": typ, "parent_table": "block", "parent_id": rootID, "alive": true}
	if props != nil {
		b["properties"] = props
	}
	if len(children) > 0 {
		b["content"] = children
	}
	f.put(b)
	return id
}

func (f *fixture) withFormat(id string, format map[string]any) {
	env := f.blocks[id].(map[string]any)["value"].(map[string]any)["value"].(map[string]any)
	env["format"] = format
}

func text(s string) [][]any { return [][]any{{s}} }

func (f *fixture) recordMap() *RecordMap {
	f.t.Helper()
	data, err := json.Marshal(map[string]any{"block": f.blocks, "signed_urls": f.signed})
	if err != nil {
		f.t.Fatal(err)
	}
	rm, err := DecodeRecordMap(bytes.NewReader(data))
	if err != nil {
		f.t.Fatal(err)
	}
	return rm
}

func (f *fixture) root(rm *RecordMap) *Block {
	f.t.Helper()
	b, err := rm.RootPage(rootID)
	if err != nil {
		f.t.Fatal(err)
	}
	return b
}
