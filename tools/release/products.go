package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// productDocs are required beside every distributed CLI so a new product
// cannot ship without its user guide, decisions and history.
var productDocs = []string{"README.md", "ADR.md", "CHANGELOG.md"}

// productStaticCommands must work without a store, network or user state for
// every CLI built on the CLI Core (help/version/schema/completion contract).
var productStaticCommands = [][]string{
	{"--help"},
	{"--version"},
	{"version", "--json"},
	{"__cli", "schema"},
	{"__cli", "contract"},
	{"completion", "generate", "bash"},
}

// checkProductSurface builds one product and proves, under a synthetic
// environment whose proxy is unreachable, that its static commands succeed,
// report the tool.json version and leave HOME/config/data/cache/TMP empty.
// It applies automatically to every directory discovered under cmd/.
func checkProductSurface(cmdRoot, name, version string) error {
	for _, doc := range productDocs {
		info, err := os.Lstat(filepath.Join(cmdRoot, name, doc))
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("product %s: missing %s", name, doc)
		}
	}
	sandbox, err := os.MkdirTemp("", "cli-tools-product-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(sandbox)
	buildEnv, err := isolatedGoEnv(filepath.Join(sandbox, "build"))
	if err != nil {
		return err
	}
	exe := filepath.Join(sandbox, "bin", name)
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	build := exec.Command(goCommandName(), "build", "-trimpath", "-o", exe, "./"+filepath.ToSlash(filepath.Join(cmdRoot, name)))
	build.Env = append(buildEnv, "CGO_ENABLED=0")
	var buildErr bytes.Buffer
	build.Stderr = &buildErr
	if err := build.Run(); err != nil {
		return fmt.Errorf("product %s: build: %w: %s", name, err, strings.TrimSpace(buildErr.String()))
	}

	run := filepath.Join(sandbox, "run")
	home, config, data, cache, tmp := filepath.Join(run, "home"), filepath.Join(run, "config"), filepath.Join(run, "data"), filepath.Join(run, "cache"), filepath.Join(run, "tmp")
	for _, dir := range []string{home, config, data, cache, tmp} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	// An unreachable proxy turns accidental network access into a failure.
	const deadProxy = "http://127.0.0.1:9"
	env := []string{
		"HOME=" + home, "XDG_CONFIG_HOME=" + config, "XDG_DATA_HOME=" + data, "XDG_CACHE_HOME=" + cache,
		"XDG_STATE_HOME=" + filepath.Join(data, "state"), "TMPDIR=" + tmp, "CLI_TOOLS_HOME=" + filepath.Join(home, ".cli-tools"),
		"HTTP_PROXY=" + deadProxy, "HTTPS_PROXY=" + deadProxy, "ALL_PROXY=" + deadProxy, "NO_PROXY=",
		"LANG=C", "LC_ALL=C", "TZ=UTC", "TERM=dumb", "NO_COLOR=1",
	}
	env = append(env, platformContractEnv(home, config, tmp)...)
	if path := os.Getenv("PATH"); path != "" {
		env = append(env, "PATH="+path)
	}
	for _, args := range productStaticCommands {
		cmd := exec.Command(exe, args...)
		cmd.Env = env
		cmd.Dir = run
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("product %s: %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
		}
		out := stdout.Bytes()
		switch strings.Join(args, " ") {
		case "--version":
			if want := name + " " + version + "\n"; string(out) != want {
				return fmt.Errorf("product %s: --version printed %q, want %q from tool.json", name, out, want)
			}
		case "version --json":
			var v struct {
				Tool    string `json:"tool"`
				Version string `json:"version"`
			}
			if err := json.Unmarshal(out, &v); err != nil || v.Tool != name || v.Version != version {
				return fmt.Errorf("product %s: version --json = %s (%v)", name, bytes.TrimSpace(out), err)
			}
		case "__cli schema", "__cli contract":
			if !json.Valid(out) {
				return fmt.Errorf("product %s: %s is not valid JSON", name, strings.Join(args, " "))
			}
		default:
			if len(bytes.TrimSpace(out)) == 0 {
				return fmt.Errorf("product %s: %s printed nothing", name, strings.Join(args, " "))
			}
		}
	}
	var written []string
	for _, dir := range []string{home, config, data, cache, tmp} {
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err == nil && path != dir {
				rel, _ := filepath.Rel(run, path)
				written = append(written, filepath.ToSlash(rel))
			}
			return nil
		})
	}
	if len(written) != 0 {
		return fmt.Errorf("product %s: static commands wrote user state: %s", name, strings.Join(written, ", "))
	}
	return nil
}
