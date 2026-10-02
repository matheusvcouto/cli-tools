package main

import (
	"archive/zip"
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A local module proxy proves that preparation fills a dedicated cache and
// subsequent checks need neither a reachable proxy nor the user's module cache.
func TestPreparedCacheOfflineChecksAndContracts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX check runner; native contract generation is covered by CI")
	}
	script, err := os.ReadFile(filepath.Join("..", "..", "scripts", "check-safe.sh"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	// Go normally makes extracted modules read-only. Only this synthetic test
	// cache is made removable before testing's TempDir cleanup runs.
	t.Cleanup(func() {
		cache := filepath.Join(root, "dist", "go-cache", "mod")
		if err := filepath.WalkDir(cache, func(path string, entry fs.DirEntry, err error) error {
			if os.IsNotExist(err) {
				return nil
			}
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return os.Chmod(path, 0o700)
			}
			return nil
		}); err != nil {
			t.Errorf("make synthetic cache removable: %v", err)
		}
	})
	write := func(name, contents string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("scripts/check-safe.sh", string(script))
	write("go.mod", "module fixture.test/checks\n\ngo 1.27.1\n\nrequire example.test/dependency v1.0.0\n")
	write("check_test.go", `package checks
import (
 "os"
 "path/filepath"
 "strings"
 "testing"
 "example.test/dependency"
)
func TestIsolation(t *testing.T) {
 if dependency.Value != "synthetic" { t.Fatal("wrong module") }
 for _, key := range []string{"CHECK_SECRET", "GH_TOKEN"} {
  if os.Getenv(key) != "" { t.Fatalf("inherited %s", key) }
 }
 if os.Getenv("GOPROXY") != "off" { t.Fatal("downloads enabled") }
 cwd, err := os.Getwd()
 if err != nil { t.Fatal(err) }
 rel, err := filepath.Rel(filepath.Join(cwd, "dist", "go-cache", "runs"), os.Getenv("HOME"))
 if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) { t.Fatal("HOME outside controlled check state") }
 if _, err := os.Stat(os.Getenv("GIT_CONFIG_GLOBAL")); !os.IsNotExist(err) { t.Fatal("real Git config") }
}
`)
	write("cmd/probe/main.go", `package main
import (
 "fmt"
 "os"
 "example.test/dependency"
)
func main() {
 if os.Getenv("CHECK_SECRET") != "" || os.Getenv("GH_TOKEN") != "" { panic("secret inherited") }
 if os.Getenv("GOPROXY") != "off" || os.Getenv("GOFLAGS") != "-mod=readonly" { panic("unsafe Go environment") }
 fmt.Printf("{\"value\":%q}\n", dependency.Value)
}
`)
	mod := "module example.test/dependency\n\ngo 1.27.1\n"
	write("proxy/example.test/dependency/@v/v1.0.0.mod", mod)
	write("proxy/example.test/dependency/@v/v1.0.0.info", `{"Version":"v1.0.0","Time":"2026-01-01T00:00:00Z"}`)
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	for name, contents := range map[string]string{
		"go.mod":        mod,
		"dependency.go": "package dependency\nconst Value = \"synthetic\"\n",
	} {
		f, err := zw.Create("example.test/dependency@v1.0.0/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(contents)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	write("proxy/example.test/dependency/@v/v1.0.0.zip", archive.String())
	t.Setenv("PATH", filepath.Join(runtime.GOROOT(), "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GOPROXY", "file://"+filepath.ToSlash(filepath.Join(root, "proxy")))
	// This synthetic proxy has no public checksum-database entry.
	t.Setenv("GOSUMDB", "off")
	t.Setenv("CHECK_SECRET", "synthetic-secret")
	t.Setenv("GH_TOKEN", "synthetic-token")
	t.Chdir(root)
	run := func(mode string) ([]byte, error) {
		t.Helper()
		cmd := exec.Command("sh", "scripts/check-safe.sh", mode)
		return cmd.CombinedOutput() // Diagnostics only, never parsed as structured output.
	}
	if out, err := run("test"); err == nil || !bytes.Contains(out, []byte("check-safe.sh prepare")) {
		t.Fatalf("missing-cache failure: %v\n%s", err, out)
	}
	if out, err := run("prepare"); err != nil {
		t.Fatalf("prepare: %v\n%s", err, out)
	}
	beforeMod, _ := os.ReadFile("go.mod")
	beforeSum, err := os.ReadFile("go.sum")
	if err != nil {
		t.Fatal(err)
	}
	// The proxy becomes unreachable without deleting any fixture.
	t.Setenv("GOPROXY", "file://"+filepath.ToSlash(filepath.Join(root, "unavailable")))
	for range 2 {
		if out, err := run("test"); err != nil {
			t.Fatalf("offline check: %v\n%s", err, out)
		}
	}
	t.Setenv("GOCACHE", filepath.Join(root, "dist", "go-cache", "build"))
	t.Setenv("GOMODCACHE", filepath.Join(root, "dist", "go-cache", "mod"))
	t.Setenv("GOENV", "off")
	t.Setenv("GOTOOLCHAIN", "local")
	out, err := generateContract("cmd", "probe")
	if err != nil || strings.TrimSpace(string(out)) != `{"value":"synthetic"}` {
		t.Fatalf("offline contract: %v\n%s", err, out)
	}
	for name, before := range map[string][]byte{"go.mod": beforeMod, "go.sum": beforeSum} {
		after, err := os.ReadFile(name)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("check changed %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "dist", "go-cache", "mod", "example.test", "dependency@v1.0.0", "dependency.go")); err != nil {
		t.Fatalf("prepared cache was not retained: %v", err)
	}
}
