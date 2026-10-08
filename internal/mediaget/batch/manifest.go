// Package batch imports bounded manifests into media-get domain requests.
package batch

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/matheusvcouto/cli-tools/internal/mediaget"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

const MaxBytes = 8 << 20

//go:embed schema.json
var schemaJSON []byte

//go:embed legacy.schema.json
var legacyJSON []byte

//go:embed example.json
var exampleJSON []byte

func Schema() []byte  { return bytes.Clone(schemaJSON) }
func Example() []byte { return bytes.Clone(exampleJSON) }

// Pointers preserve absence separately from false, empty strings and zero.
type Options struct {
	Kind                *string `json:"kind,omitempty"`
	Quality             *string `json:"quality,omitempty"`
	VideoFormat         *string `json:"videoFormat,omitempty"`
	SubtitleFormat      *string `json:"subtitleFormat,omitempty"`
	SubtitleLang        *string `json:"subtitleLang,omitempty"`
	AutoSubs            *bool   `json:"autoSubs,omitempty"`
	ConcurrentFragments *int    `json:"concurrentFragments,omitempty"`
	OutputDir           *string `json:"outputDir,omitempty"`
	Referer             *string `json:"referer,omitempty"`
	Origin              *string `json:"origin,omitempty"`
}

func (o Options) Overlay(next Options) Options {
	if next.Kind != nil {
		o.Kind = next.Kind
	}
	if next.Quality != nil {
		o.Quality = next.Quality
	}
	if next.VideoFormat != nil {
		o.VideoFormat = next.VideoFormat
	}
	if next.SubtitleFormat != nil {
		o.SubtitleFormat = next.SubtitleFormat
	}
	if next.SubtitleLang != nil {
		o.SubtitleLang = next.SubtitleLang
	}
	if next.AutoSubs != nil {
		o.AutoSubs = next.AutoSubs
	}
	if next.ConcurrentFragments != nil {
		o.ConcurrentFragments = next.ConcurrentFragments
	}
	if next.OutputDir != nil {
		o.OutputDir = next.OutputDir
	}
	if next.Referer != nil {
		o.Referer = next.Referer
	}
	if next.Origin != nil {
		o.Origin = next.Origin
	}
	return o
}

type Item struct {
	ID      string  `json:"id,omitempty"`
	URL     string  `json:"url"`
	Name    string  `json:"name,omitempty"`
	Title   string  `json:"title,omitempty"`
	Referer *string `json:"referer,omitempty"`
	Origin  *string `json:"origin,omitempty"`
	Options Options `json:"options,omitempty"`
}
type Manifest struct {
	SchemaVersion int     `json:"schemaVersion"`
	Defaults      Options `json:"defaults,omitempty"`
	Execution     struct {
		Jobs    int    `json:"jobs,omitempty"`
		OnError string `json:"onError,omitempty"`
	} `json:"execution,omitempty"`
	Items    []Item   `json:"items"`
	Warnings []string `json:"-"`
}

type schemaPair struct{ native, legacy *jsonschema.Schema }

var schemas = sync.OnceValues(func() (schemaPair, error) {
	compile := func(data []byte) (*jsonschema.Schema, error) {
		var doc any
		if err := json.Unmarshal(data, &doc); err != nil {
			return nil, err
		}
		c := jsonschema.NewCompiler()
		// No filesystem/network loader: all schemas and refs are embedded.
		c.UseLoader(nil)
		c.DefaultDraft(jsonschema.Draft2020)
		if err := c.AddResource("urn:media-get:manifest", doc); err != nil {
			return nil, err
		}
		return c.Compile("urn:media-get:manifest")
	}
	native, err := compile(schemaJSON)
	if err != nil {
		return schemaPair{}, err
	}
	legacy, err := compile(legacyJSON)
	return schemaPair{native, legacy}, err
})

func Read(r io.Reader) (Manifest, error) {
	var m Manifest
	data, err := io.ReadAll(io.LimitReader(r, MaxBytes+1))
	if err != nil {
		return m, errors.New("não foi possível ler o manifesto")
	}
	if len(data) > MaxBytes {
		return m, errors.New("manifesto excede 8 MiB")
	}
	if !utf8.Valid(data) {
		return m, errors.New("manifesto deve ser UTF-8 válido")
	}
	if !validSurrogates(data) {
		return m, errors.New("escape Unicode inválido no JSON")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	value, err := strictValue(d, 0)
	if err != nil {
		return m, err
	}
	if _, err := d.Token(); err != io.EOF {
		return m, errors.New("dados após o documento JSON")
	}
	obj, ok := value.(map[string]any)
	if !ok {
		return m, errors.New("manifesto deve ser um objeto JSON")
	}
	pair, err := schemas()
	if err != nil {
		return m, errors.New("schema interno inválido")
	}
	_, isNative := obj["schemaVersion"]
	schema := pair.native
	if !isNative {
		schema = pair.legacy
	}
	if err := schema.Validate(value); err != nil {
		var validation *jsonschema.ValidationError
		if errors.As(err, &validation) {
			for len(validation.Causes) > 0 {
				validation = validation.Causes[0]
			}
			// Locations refer to structure; error messages may echo signed URLs.
			return m, fmt.Errorf("manifesto inválido no campo %s; confira batch schema", safeLocation(validation.InstanceLocation))
		}
		return m, errors.New("manifesto não corresponde ao schema")
	}
	if isNative {
		if err := json.Unmarshal(data, &m); err != nil {
			return m, errors.New("tipos inválidos no manifesto")
		}
	} else {
		var old struct {
			Videos []struct {
				Title, URL, Command, Confidence, RefererProvenance string
				Referer, Origin                                    *string
			}
		}
		if err := json.Unmarshal(data, &old); err != nil {
			return m, errors.New("manifesto legado inválido")
		}
		m.SchemaVersion = 1
		m.Warnings = append(m.Warnings, "Formato de descoberta importado; URLs inferidas precisam de verificação.")
		for i, item := range old.Videos {
			m.Items = append(m.Items, Item{URL: item.URL, Title: item.Title, Referer: item.Referer, Origin: item.Origin})
			if item.Command != "" {
				m.Warnings = append(m.Warnings, fmt.Sprintf("Item %d: command ignorado; comandos nunca são executados.", i+1))
			}
		}
	}
	if m.Execution.Jobs == 0 {
		m.Execution.Jobs = 2
	}
	if m.Execution.OnError == "" {
		m.Execution.OnError = "continue"
	}
	ids := map[string]bool{}
	for _, item := range m.Items {
		if item.ID != "" {
			if ids[item.ID] {
				return m, errors.New("IDs duplicados no manifesto")
			}
			ids[item.ID] = true
		}
	}
	for i := range m.Items {
		if m.Items[i].ID == "" {
			id := fmt.Sprintf("item-%d", i+1)
			for ids[id] {
				id += "-"
			}
			m.Items[i].ID = id
			ids[id] = true
		}
		if _, err := m.Request(i, Options{}, Options{}); err != nil {
			return m, fmt.Errorf("item %d: %w", i+1, err)
		}
	}
	return m, nil
}

// encoding/json v1 repairs lone UTF-16 surrogates. Import must not silently
// change identifiers/URLs, so reject them before decoding.
func validSurrogates(data []byte) bool {
	inString := false
	for i := 0; i < len(data); i++ {
		if data[i] == '"' {
			inString = !inString
			continue
		}
		if !inString || data[i] != '\\' {
			continue
		}
		if i+1 >= len(data) {
			return false
		}
		if data[i+1] != 'u' {
			i++
			continue
		}
		if i+6 > len(data) {
			return false
		}
		n, err := strconv.ParseUint(string(data[i+2:i+6]), 16, 16)
		if err != nil {
			return false
		}
		if n >= 0xdc00 && n <= 0xdfff {
			return false
		}
		if n >= 0xd800 && n <= 0xdbff {
			if i+12 > len(data) || string(data[i+6:i+8]) != "\\u" {
				return false
			}
			low, err := strconv.ParseUint(string(data[i+8:i+12]), 16, 16)
			if err != nil || low < 0xdc00 || low > 0xdfff {
				return false
			}
			i += 6
		}
		i += 5
	}
	return true
}
func safeLocation(parts []string) string {
	var safe []string
	for _, s := range parts {
		if len(s) > 40 || strings.ContainsAny(s, ":?=/\\\n\r") {
			s = "campo"
		}
		safe = append(safe, mediaget.SafeName(s))
	}
	return "/" + strings.Join(safe, "/")
}

// strictValue rejects duplicate keys, null and excessive nesting before schema
// validation, including inside inert metadata. No invalid data is echoed.
func strictValue(d *json.Decoder, depth int) (any, error) {
	if depth > 32 {
		return nil, errors.New("JSON excede 32 níveis")
	}
	t, err := d.Token()
	if err != nil {
		return nil, errors.New("JSON malformado")
	}
	if t == nil {
		return nil, errors.New("null não é aceito; omita o campo para herdar")
	}
	if delimiter, ok := t.(json.Delim); ok {
		switch delimiter {
		case '{':
			obj := map[string]any{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return nil, errors.New("JSON malformado")
				}
				k, ok := key.(string)
				if !ok {
					return nil, errors.New("campo JSON inválido")
				}
				if _, exists := obj[k]; exists {
					return nil, errors.New("campo duplicado no JSON")
				}
				v, err := strictValue(d, depth+1)
				if err != nil {
					return nil, err
				}
				obj[k] = v
			}
			if _, err := d.Token(); err != nil {
				return nil, errors.New("JSON malformado")
			}
			return obj, nil
		case '[':
			arr := []any{}
			for d.More() {
				v, err := strictValue(d, depth+1)
				if err != nil {
					return nil, err
				}
				arr = append(arr, v)
			}
			if _, err := d.Token(); err != nil {
				return nil, errors.New("JSON malformado")
			}
			return arr, nil
		}
	}
	return t, nil
}

// Request resolves flags > item > defaults > fallback per field. It performs
// semantic checks only: destination existence and metadata are preparation.
func (m Manifest) Request(index int, fallback, overrides Options) (mediaget.Request, error) {
	item := m.Items[index]
	o := fallback.Overlay(m.Defaults).Overlay(item.Options).Overlay(Options{Referer: item.Referer, Origin: item.Origin}).Overlay(overrides)
	req := mediaget.Request{Source: mediaget.Source{URL: item.URL}, Selection: mediaget.Selection{Kind: mediaget.Video}}
	if o.Kind != nil {
		req.Selection.Kind = mediaget.Kind(*o.Kind)
	}
	if o.Quality != nil {
		if req.Selection.Kind != mediaget.Video {
			return req, errors.New("quality exige vídeo")
		}
		if *o.Quality != "best" {
			req.Selection.Height, _ = strconv.Atoi(*o.Quality)
		}
	}
	if o.VideoFormat != nil {
		req.Selection.VideoFormat = *o.VideoFormat
	}
	if o.SubtitleFormat != nil {
		req.Selection.SubtitleFormat = *o.SubtitleFormat
	}
	if o.SubtitleLang != nil {
		req.Selection.Track.Lang = *o.SubtitleLang
	}
	if o.AutoSubs != nil {
		req.Selection.Track.Auto = *o.AutoSubs
	}
	if req.Selection.Kind != mediaget.Subtitle && (o.SubtitleLang != nil || o.AutoSubs != nil) {
		return req, errors.New("opções de legenda exigem subtitle")
	}
	if o.ConcurrentFragments != nil {
		req.Source.ConcurrentFragments = *o.ConcurrentFragments
	}
	if o.Referer != nil {
		req.Source.Referer = *o.Referer
	}
	if o.Origin != nil {
		req.Source.Origin = *o.Origin
	}
	if o.OutputDir != nil {
		req.OutputDir = *o.OutputDir
		if strings.TrimSpace(req.OutputDir) == "" {
			return req, errors.New("outputDir vazio")
		}
	}
	req.Name = item.Name
	if strings.TrimSpace(req.Name) == "" {
		req.Name = item.Title
	}
	req.Name, req.Selection = mediaget.OutputSelection(req.Name, req.Selection)
	if err := mediaget.ValidateSource(req.Source); err != nil {
		return req, err
	}
	if err := mediaget.ValidateSelection(req.Selection); err != nil {
		return req, err
	}
	return req, nil
}
