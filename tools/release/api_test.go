package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicAPIContractAllowsAdditionsAndRejectsBreakingChanges(t *testing.T) {
	root := t.TempDir()
	write := func(src string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, "api.go"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(`package sample

type Config struct {
	Name string
	private string
}

func Parse(input string) (Config, error) { return Config{}, nil }
`)
	baseline, err := generatePublicAPIContract(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(baseline.Symbols) != 2 {
		t.Fatalf("symbols=%v", baseline.Symbols)
	}
	for _, symbol := range baseline.Symbols {
		if strings.Contains(symbol.Declaration, "private") {
			t.Fatalf("private struct field leaked into API contract: %q", symbol.Declaration)
		}
	}

	write(`package sample

type Config struct {
	Name string
	private int
}

func Parse(input string) (Config, error) { return Config{}, nil }
func Added() {}
`)
	withAddition, err := generatePublicAPIContract(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := checkPublicAPICompatibility(baseline, withAddition); err != nil {
		t.Fatalf("additive API rejected: %v", err)
	}

	write(`package sample

type Config struct { Name string }
func Parse(input []byte) (Config, error) { return Config{}, nil }
`)
	changed, err := generatePublicAPIContract(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := checkPublicAPICompatibility(baseline, changed); err == nil || !strings.Contains(err.Error(), "func Parse changed") {
		t.Fatalf("breaking signature change not detected: %v", err)
	}
}

func TestPublicAPIContractRoundTrip(t *testing.T) {
	contract := publicAPIContract{
		SchemaVersion: publicAPIContractSchemaVersion,
		Package:       "sample",
		Symbols:       []publicAPISymbol{{ID: "func Parse", Declaration: "func Parse(string) error"}},
	}
	raw, err := marshalPublicAPIContract(contract)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "api.contract.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := loadPublicAPIContract(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Package != contract.Package || len(got.Symbols) != 1 || got.Symbols[0] != contract.Symbols[0] {
		t.Fatalf("round trip=%+v", got)
	}
}
