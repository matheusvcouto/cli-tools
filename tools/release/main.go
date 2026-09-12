package main

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type target struct{ GOOS, GOARCH string }

var defaultTargets = []target{{"darwin", "amd64"}, {"darwin", "arm64"}, {"linux", "amd64"}, {"linux", "arm64"}}

func main() {
	var version, outDir string
	flag.StringVar(&version, "version", "dev", "release version, e.g. v1.2.0")
	flag.StringVar(&outDir, "out", "dist", "output directory")
	flag.Parse()
	if flag.NArg() != 0 {
		fatalf("unexpected arguments: %v", flag.Args())
	}
	if err := requireReleaseToolchain(); err != nil {
		fatalf("toolchain: %v", err)
	}
	module, err := modulePath()
	if err != nil {
		fatalf("module path: %v", err)
	}
	bins, err := discoverCommands("cmd")
	if err != nil {
		fatalf("discover commands: %v", err)
	}
	if len(bins) == 0 {
		fatalf("no commands found under cmd/")
	}
	if err := os.RemoveAll(outDir); err != nil {
		fatalf("clean output: %v", err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fatalf("mkdir output: %v", err)
	}
	for _, t := range defaultTargets {
		if err := buildTarget(version, outDir, module, bins, t); err != nil {
			fatalf("build %s/%s: %v", t.GOOS, t.GOARCH, err)
		}
	}
	if err := writeChecksums(outDir); err != nil {
		fatalf("checksums: %v", err)
	}
}

func discoverCommands(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var names []string
	seen := map[string]string{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		fold := strings.ToLower(name)
		if prev, ok := seen[fold]; ok {
			return nil, fmt.Errorf("case-folding collision: %q and %q", prev, name)
		}
		seen[fold] = name
		cmd := exec.Command("go", "list", "-f", "{{.Name}}", "./"+filepath.ToSlash(filepath.Join(root, name)))
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("%s is not a valid Go package: %w", name, err)
		}
		if strings.TrimSpace(string(out)) != "main" {
			return nil, fmt.Errorf("%s is not package main", name)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func modulePath() (string, error) {
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Path}}")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", fmt.Errorf("empty module path")
	}
	return path, nil
}

var goVersionRE = regexp.MustCompile(`^go(\d+)\.(\d+)(?:\.(\d+))?`)

func requireReleaseToolchain() error {
	cmd := exec.Command("go", "env", "GOVERSION")
	out, err := cmd.Output()
	if err != nil {
		return err
	}
	version := strings.TrimSpace(string(out))
	parts, err := parseGoVersion(version)
	if err != nil {
		return err
	}
	min := [3]int{1, 27, 1}
	for i := range parts {
		if parts[i] > min[i] {
			return nil
		}
		if parts[i] < min[i] {
			return fmt.Errorf("Go %d.%d.%d or newer is required for release builds; found %s", min[0], min[1], min[2], version)
		}
	}
	return nil
}

func parseGoVersion(version string) ([3]int, error) {
	m := goVersionRE.FindStringSubmatch(version)
	if m == nil {
		return [3]int{}, fmt.Errorf("cannot parse GOVERSION %q", version)
	}
	parts := [3]int{}
	for i := range parts {
		if i+1 >= len(m) || m[i+1] == "" {
			continue
		}
		v, err := strconv.Atoi(m[i+1])
		if err != nil {
			return [3]int{}, err
		}
		parts[i] = v
	}
	return parts, nil
}

func buildTarget(version, outDir, module string, bins []string, t target) error {
	stage, err := os.MkdirTemp("", "cli-tools-release-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	binDir := filepath.Join(stage, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}
	for _, name := range bins {
		exe := name
		if t.GOOS == "windows" {
			exe += ".exe"
		}
		out := filepath.Join(binDir, exe)
		args := []string{"build", "-trimpath", "-ldflags", fmt.Sprintf("-s -w -X %s/internal/version.Version=%s", module, version), "-o", out, "./cmd/" + name}
		cmd := exec.Command("go", args...)
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+t.GOOS, "GOARCH="+t.GOARCH)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	base := fmt.Sprintf("cli-tools_%s_%s_%s", strings.TrimPrefix(version, "v"), releaseOSName(t.GOOS), t.GOARCH)
	if t.GOOS == "windows" {
		return zipDir(filepath.Join(outDir, base+".zip"), stage)
	}
	return tarGzDir(filepath.Join(outDir, base+".tar.gz"), stage)
}

func releaseOSName(goos string) string {
	if goos == "darwin" {
		return "macos"
	}
	return goos
}

func tarGzDir(dst, root string) error {
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	gz.Header.ModTime = time.Unix(0, 0).UTC()
	gz.Header.OS = 255
	tw := tar.NewWriter(gz)
	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		h, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		h.Name = filepath.ToSlash(rel)
		h.ModTime = time.Unix(0, 0).UTC()
		h.AccessTime = time.Time{}
		h.ChangeTime = time.Time{}
		h.Uid = 0
		h.Gid = 0
		h.Uname = ""
		h.Gname = ""
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tw, in)
		closeErr := in.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

func zipDir(dst, root string) error {
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root || info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		h, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		h.Name = filepath.ToSlash(rel)
		h.Method = zip.Deflate
		h.Modified = time.Unix(0, 0).UTC()
		w, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(w, in)
		closeErr := in.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if err != nil {
		return err
	}
	return zw.Close()
}

func writeChecksums(outDir string) error {
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && e.Name() != "SHA256SUMS" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	f, err := os.Create(filepath.Join(outDir, "SHA256SUMS"))
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	for _, name := range names {
		in, err := os.Open(filepath.Join(outDir, name))
		if err != nil {
			return err
		}
		h := sha256.New()
		_, copyErr := io.Copy(h, in)
		closeErr := in.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(h.Sum(nil)), name)
	}
	return w.Flush()
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
