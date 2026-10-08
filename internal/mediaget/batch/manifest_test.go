package batch

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestLegacyDiscoveryImportsFifteenWithoutCommands(t *testing.T) {
	items := []string{}
	for i := range 15 {
		items = append(items, fmt.Sprintf(`{"title":"Synthetic %d","url":"https://cdn.example.invalid/%d.m3u8","referer":"https://example.invalid/page","origin":"https://example.invalid","refererProvenance":"inferred","confidence":"hint","sources":["poster"],"command":"ignored synthetic command"}`, i, i))
	}
	m, err := Read(strings.NewReader(`{"pageUrl":"https://example.invalid","exportedAt":"synthetic","videos":[` + strings.Join(items, ",") + `]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Items) != 15 || len(m.Warnings) != 16 {
		t.Fatalf("%+v", m)
	}
	r, err := m.Request(2, Options{}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Name != "Synthetic 2" || r.Source.Origin != "https://example.invalid" || r.Source.FragmentConcurrency() != 4 || r.Selection.Kind != "video" {
		t.Fatalf("%+v", r)
	}
}
func TestNativePresencePrecedenceAndIDs(t *testing.T) {
	m, err := Read(strings.NewReader(`{"schemaVersion":1,"defaults":{"kind":"subtitle","subtitleLang":"en","autoSubs":true,"referer":"https://example.invalid","concurrentFragments":25},"items":[{"id":"item-2","url":"https://example.invalid/a","name":"first.txt","options":{"autoSubs":false},"referer":""},{"url":"https://example.invalid/b","title":"second"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	n := 8
	r, err := m.Request(0, Options{}, Options{ConcurrentFragments: &n})
	if err != nil {
		t.Fatal(err)
	}
	if r.Selection.Track.Auto || r.Source.Referer != "" || r.Source.ConcurrentFragments != 8 || r.Selection.SubtitleFormat != "txt" || r.Name != "first" || m.Items[1].ID == m.Items[0].ID {
		t.Fatalf("%+v %+v", r, m.Items)
	}
}
func TestInvalidManifestFailsClosedAndDoesNotEchoValues(t *testing.T) {
	base := `{"schemaVersion":1,"items":[{"url":"https://example.invalid"}]}`
	cases := []string{
		`{}`, `[]`, base + ` {}`, strings.Replace(base, `"schemaVersion":1`, `"schemaVersion":2`, 1),
		strings.Replace(base, `"schemaVersion":1`, `"SchemaVersion":1`, 1), strings.Replace(base, `"schemaVersion":1`, `"schemaVersion":1,"schemaVersion":1`, 1),
		strings.Replace(base, `"items"`, `"itemsX"`, 1), strings.Replace(base, `"url":"https://example.invalid"`, `"url":null`, 1),
		strings.Replace(base, `"url":"https://example.invalid"`, `"url":"file:///SECRET"`, 1),
		strings.Replace(base, `"url":"https://example.invalid"`, `"url":"https://example.invalid","origin":"https://example.invalid/path?SECRET"`, 1),
		strings.Replace(base, `"items"`, `"defaults":{"kind":"subtitle"},"items"`, 1),
		strings.Replace(base, `"items"`, `"execution":{"jobs":9},"items"`, 1),
		strings.Replace(base, `"url":"https://example.invalid"`, `"url":"https://example.invalid","name":"\ud800SECRET"`, 1),
		strings.Replace(base, `"items"`, `"metadata":{"x":`+strings.Repeat(`[`, 34)+`0`+strings.Repeat(`]`, 34)+`},"items"`, 1),
		strings.Replace(base, `"items"`, `"defaults":{"kind":"audio","quality":"best"},"items"`, 1),
		strings.Replace(base, `"url":"https://example.invalid"`, `"url":"https://example.invalid","options":{"concurrentFragments":0}`, 1),
		`{"videos":[{"url":"https://example.invalid","unknown":"SECRET"}]}`,
		strings.Repeat(" ", MaxBytes+1), string([]byte{0xff}),
	}
	for i, data := range cases {
		if _, err := Read(strings.NewReader(data)); err == nil || strings.Contains(err.Error(), "SECRET") {
			t.Errorf("case %d: %v", i, err)
		}
	}
}
func TestPublishedExampleAndEmbeddedSchema(t *testing.T) {
	if _, err := Read(strings.NewReader(string(Example()))); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(Schema(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
		t.Fatal(doc)
	}
	data := `{"schemaVersion":1,"items":[{"url":"https://example.invalid","name":"\ud83d\ude00"}]}`
	if _, err := Read(strings.NewReader(data)); err != nil {
		t.Fatal(err)
	}
}
