package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/matheusvcouto/cli-tools/internal/aiprofile"
)

func main() {
	var from, root string
	flag.StringVar(&from, "from", "", "explicit path to legacy index.nuon")
	flag.StringVar(&root, "to-root", "", "explicit profile root that will receive index.json")
	flag.Parse()
	if from == "" || root == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: migrate-ai-profile-index --from /path/index.nuon --to-root /path/.ai-profiles")
		os.Exit(2)
	}
	if err := migrate(from, root); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("migrated legacy index to %s\n", filepath.Join(root, "index.json"))
}

func migrate(from, root string) error {
	fromAbs, err := filepath.Abs(from)
	if err != nil {
		return err
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	if fromAbs == filepath.Join(rootAbs, "index.json") {
		return errors.New("source and destination must be different files")
	}
	if _, err := os.Lstat(filepath.Join(rootAbs, "index.json")); err == nil {
		return errors.New("destination index.json already exists; refusing to merge or overwrite")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	raw, err := readLegacyIndex(fromAbs)
	if err != nil {
		return err
	}
	profiles, err := parseLegacyNUON(string(raw))
	if err != nil {
		return fmt.Errorf("parse legacy index: %w", err)
	}
	store := aiprofile.Store{Root: rootAbs}
	return store.ImportLegacy(profiles, fromAbs, raw)
}

// The legacy index is user-provided local metadata, not an unbounded input
// source. Reject links and special files; verify identity before and after a
// bounded read. The explicit migration never changes the source file.
const maxLegacyIndexBytes int64 = 8 << 20

func readLegacyIndex(path string) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, fmt.Errorf("legacy source is not a regular file: %s", path)
	}
	if before.Size() > maxLegacyIndexBytes {
		return nil, fmt.Errorf("legacy index exceeds %d bytes", maxLegacyIndexBytes)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	opened, statErr := f.Stat()
	if statErr != nil {
		_ = f.Close()
		return nil, statErr
	}
	if !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		_ = f.Close()
		return nil, fmt.Errorf("legacy source changed identity while opening")
	}
	raw, readErr := io.ReadAll(io.LimitReader(f, maxLegacyIndexBytes+1))
	closeErr := f.Close()
	if readErr != nil {
		return nil, fmt.Errorf("read legacy index: %w", readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close legacy index: %w", closeErr)
	}
	if int64(len(raw)) > maxLegacyIndexBytes {
		return nil, fmt.Errorf("legacy index exceeds %d bytes", maxLegacyIndexBytes)
	}
	after, err := os.Lstat(path)
	if err != nil || after.Mode()&os.ModeSymlink != 0 || !after.Mode().IsRegular() || !os.SameFile(opened, after) {
		return nil, fmt.Errorf("legacy source changed identity during read")
	}
	return raw, nil
}

type lexer struct {
	s string
	i int
}

func (l *lexer) skipWS() {
	for l.i < len(l.s) && strings.ContainsRune(" \t\r\n", rune(l.s[l.i])) {
		l.i++
	}
}
func (l *lexer) peek() byte {
	l.skipWS()
	if l.i >= len(l.s) {
		return 0
	}
	return l.s[l.i]
}
func (l *lexer) punct(ch byte) bool {
	l.skipWS()
	if l.i < len(l.s) && l.s[l.i] == ch {
		l.i++
		return true
	}
	return false
}
func (l *lexer) scalar() (string, error) {
	l.skipWS()
	if l.i >= len(l.s) {
		return "", errors.New("unexpected end of input")
	}
	start := l.i
	switch l.s[l.i] {
	case '"':
		l.i++
		escaped := false
		for l.i < len(l.s) {
			c := l.s[l.i]
			l.i++
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				v, err := strconv.Unquote(l.s[start:l.i])
				if err != nil {
					return "", err
				}
				return v, nil
			}
		}
		return "", errors.New("unterminated double-quoted string")
	case '\'':
		l.i++
		valStart := l.i
		for l.i < len(l.s) && l.s[l.i] != '\'' {
			l.i++
		}
		if l.i >= len(l.s) {
			return "", errors.New("unterminated single-quoted string")
		}
		v := l.s[valStart:l.i]
		l.i++
		return v, nil
	default:
		for l.i < len(l.s) && !strings.ContainsRune("[],; \t\r\n", rune(l.s[l.i])) {
			l.i++
		}
		if l.i == start {
			return "", fmt.Errorf("expected scalar near byte %d", l.i)
		}
		return l.s[start:l.i], nil
	}
}

func parseLegacyNUON(input string) ([]aiprofile.Profile, error) {
	l := &lexer{s: input}
	if !l.punct('[') {
		return nil, errors.New("expected outer [")
	}
	if l.punct(']') {
		return []aiprofile.Profile{}, nil
	}
	if !l.punct('[') {
		return nil, errors.New("expected header row [")
	}
	headers, err := parseScalarList(l, ']')
	if err != nil {
		return nil, err
	}
	if !l.punct(';') {
		return nil, errors.New("expected ; after table header")
	}
	rows := make([][]string, 0)
	for {
		if l.punct(']') {
			break
		}
		if !l.punct('[') {
			return nil, fmt.Errorf("expected data row [ near byte %d", l.i)
		}
		row, err := parseScalarList(l, ']')
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
		_ = l.punct(',')
	}
	l.skipWS()
	if l.i != len(l.s) {
		return nil, fmt.Errorf("trailing data near byte %d", l.i)
	}
	idx := map[string]int{}
	for i, h := range headers {
		idx[h] = i
	}
	for _, required := range []string{"tool", "alias", "dir", "created_at"} {
		if _, ok := idx[required]; !ok {
			return nil, fmt.Errorf("missing column %q", required)
		}
	}
	out := make([]aiprofile.Profile, 0, len(rows))
	for n, row := range rows {
		if len(row) != len(headers) {
			return nil, fmt.Errorf("row %d has %d values for %d columns", n+1, len(row), len(headers))
		}
		out = append(out, aiprofile.Profile{Tool: row[idx["tool"]], Alias: row[idx["alias"]], Dir: row[idx["dir"]], CreatedAt: row[idx["created_at"]]})
	}
	return out, nil
}

func parseScalarList(l *lexer, end byte) ([]string, error) {
	var out []string
	if l.punct(end) {
		return out, nil
	}
	for {
		v, err := l.scalar()
		if err != nil {
			return nil, err
		}
		out = append(out, v)
		if l.punct(end) {
			return out, nil
		}
		if !l.punct(',') {
			return nil, fmt.Errorf("expected comma near byte %d", l.i)
		}
	}
}
