package repocli

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	core "github.com/matheusvcouto/cli-tools/cli"
	"github.com/matheusvcouto/cli-tools/internal/repozip"
)

func TestStaticSurfaceDoesNotNeedWorkingService(t *testing.T) {
	app, err := New(repozip.Service{}, core.ProductMetadata{Version: "9.8.7"})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := app.Run(context.Background(), []string{"--help"}, core.IO{Out: &out}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "--suffix") || strings.Contains(out.String(), "--version <") {
		t.Fatal(out.String())
	}
	out.Reset()
	if err := app.Run(context.Background(), []string{"--version"}, core.IO{Out: &out}); err != nil {
		t.Fatal(err)
	}
	if out.String() != "repo-zip 9.8.7\n" {
		t.Fatalf("version=%q", out.String())
	}
}

func TestLegacyVersionValueIsRejected(t *testing.T) {
	app, err := New(repozip.Service{}, core.ProductMetadata{Version: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	err = app.Run(context.Background(), []string{"--version", "v1"}, core.IO{})
	if err == nil {
		t.Fatal("expected --version value to be rejected")
	}
}

func TestCLIRejectsConflictingOutputOptionsBeforeService(t *testing.T) {
	app, err := New(repozip.Service{}, core.ProductMetadata{Version: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"repo", "--output", "x.zip", "--name", "x"},
		{"--output", "x.zip", "repo", "--suffix", "v1"},
	} {
		err := app.Run(context.Background(), args, core.IO{})
		if err == nil || core.ExitCode(err) != 2 {
			t.Fatalf("args=%v err=%v", args, err)
		}
	}
}

func TestContractLockIsCurrent(t *testing.T) {
	app, err := New(repozip.Service{}, core.ProductMetadata{Version: "0.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := app.ContractJSON()
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../../cmd/repo-zip/cli.contract.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("repo-zip cli.contract.json is stale; regenerate it from __cli contract")
	}
}
