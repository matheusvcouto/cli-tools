package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const probeMain = `package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const name, version = "NAME", "VERSION"

func main() {
	switch strings.Join(os.Args[1:], " ") {
	case "--version":
		fmt.Printf("%s %s\n", name, version)
	case "version --json":
		fmt.Printf("{\"tool\":%q,\"version\":%q}\n", name, version)
	case "__cli schema", "__cli contract":
		fmt.Println("{}")
	case "--help":
		if LEAK {
			os.WriteFile(filepath.Join(os.Getenv("HOME"), "state.json"), []byte("{}"), 0o600)
		}
		fmt.Println("usage")
	default:
		fmt.Println("ok")
	}
}
`

func TestProductSurfaceGateCatchesUnsafeProducts(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.test/products\n\ngo 1.27.1\n")
	probe := func(name, printed string, leak bool, docs ...string) {
		write("cmd/"+name+"/main.go", strings.NewReplacer("NAME", name, "VERSION", printed, "LEAK", fmt.Sprint(leak)).Replace(probeMain))
		for _, doc := range docs {
			write("cmd/"+name+"/"+doc, "# "+name+"\n")
		}
	}
	probe("good", "1.0.0", false, productDocs...)
	probe("leaky", "1.0.0", true, productDocs...)
	probe("badver", "9.9.9", false, productDocs...)
	probe("nodocs", "1.0.0", false, "README.md")
	t.Chdir(root)

	if err := checkProductSurface("cmd", "good", "1.0.0"); err != nil {
		t.Fatalf("good product rejected: %v", err)
	}
	for name, want := range map[string]string{
		"leaky":  "wrote user state: home/state.json",
		"badver": "--version printed",
		"nodocs": "missing ADR.md",
	} {
		err := checkProductSurface("cmd", name, "1.0.0")
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want %q", name, err, want)
		}
	}
}
