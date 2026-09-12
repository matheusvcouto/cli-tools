package main

import (
	"context"
	"os"

	"github.com/matheusvcouto/cli-tools/internal/aiprofile"
	"github.com/matheusvcouto/cli-tools/internal/aiprofile/platform"
	"github.com/matheusvcouto/cli-tools/internal/cliapp"
	"github.com/matheusvcouto/cli-tools/internal/version"
)

func main() {
	args := os.Args[1:]
	appIO := aiprofile.AppIO{
		In: os.Stdin, Out: os.Stdout, Err: os.Stderr,
		Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr,
	}

	// Static commands must stay independent from HOME/profile-store health.
	// This keeps --help/--version/completion usable even while diagnosing a
	// broken or not-yet-configured profile environment.
	if isStaticInvocation(args) {
		exitOnError(aiprofile.App{Version: version.Version}.Run(context.Background(), args, appIO))
		return
	}

	root, err := aiprofile.DefaultRoot()
	if err != nil {
		exitOnError(err)
		return
	}
	service, err := aiprofile.NewService(aiprofile.Store{Root: root}, platform.Runner{})
	if err != nil {
		exitOnError(err)
		return
	}
	app := aiprofile.App{Service: service, Version: version.Version}
	exitOnError(app.Run(context.Background(), args, appIO))
}

func isStaticInvocation(args []string) bool {
	if len(args) == 0 {
		return true
	}
	switch args[0] {
	case "--help", "-h", "help", "--version", "version", "completion":
		return true
	default:
		return false
	}
}

func exitOnError(err error) {
	if err == nil {
		return
	}
	cliapp.RenderError(os.Stderr, err)
	os.Exit(cliapp.ExitCode(err))
}
