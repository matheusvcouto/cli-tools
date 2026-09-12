//go:build darwin || linux

package platform

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/matheusvcouto/cli-tools/internal/testenv"

	"github.com/matheusvcouto/cli-tools/internal/aiprofile"
)

func TestRunnerReplaceHelper(t *testing.T) {
	if os.Getenv("GO_RUNNER_HELPER") != "1" {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	env, err := testenv.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	env = append(env, "GO_RUNNER_TARGET=1", "SYNTHETIC_VALUE=ok")
	if err := (Runner{}).Replace(context.Background(), exe, []string{"-test.run=TestRunnerExecTarget"}, env, aiprofile.ProcessIO{}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(97)
	}
	os.Exit(98)
}

func TestRunnerExecTarget(t *testing.T) {
	if os.Getenv("GO_RUNNER_TARGET") != "1" {
		return
	}
	fmt.Printf("target:%s", os.Getenv("SYNTHETIC_VALUE"))
	os.Exit(23)
}

func TestRunnerReplacesProcessAndPreservesExit(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=TestRunnerReplaceHelper")
	env, err := testenv.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cmd.Env = append(env, "GO_RUNNER_HELPER=1")
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("expected child exit error, got %v output=%q", err, out)
	}
	if exit.ExitCode() != 23 {
		t.Fatalf("exit code = %d, want 23; output=%q", exit.ExitCode(), out)
	}
	if !strings.Contains(string(out), "target:ok") {
		t.Fatalf("target output missing: %q", out)
	}
}
