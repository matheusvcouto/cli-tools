package repozip

import (
	"context"
	"errors"
	"fmt"
	"io"
)

type App struct {
	Service Service
	Version string
}

func (a App) Run(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "version") {
		fmt.Fprintln(out, a.Version)
		return nil
	}
	opts, err := ParseArgs(args)
	if errors.Is(err, ErrHelp) {
		printHelp(out)
		return nil
	}
	if err != nil {
		return err
	}
	result, err := a.Service.Run(ctx, opts)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, result.Output)
	return nil
}

func printHelp(out io.Writer) {
	fmt.Fprint(out, `repo-zip — create a Git-aware ZIP snapshot

Usage:
  repo-zip [source] [options]

Options:
  -o, --output PATH   exact output path (.zip)
  -n, --name NAME     base name for automatic output
      --git           include restorable Git bundle and add HEAD/dirty to auto name
  -s, --suffix TEXT   append suffix to automatic name
  -v, --version TEXT  alias for --suffix
  -f, --force         atomically replace an existing regular output file
  -h, --help          show help

Default output: <repo>/.tmp/repo-zip/<name>.zip
`)
}
