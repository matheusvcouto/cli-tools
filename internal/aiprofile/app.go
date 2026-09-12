package aiprofile

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
)

type AppIO struct {
	In     io.Reader
	Out    io.Writer
	Err    io.Writer
	Stdin  *os.File
	Stdout *os.File
	Stderr *os.File
}

type App struct {
	Service *Service
	Version string
}

func (a App) Run(ctx context.Context, args []string, io AppIO) error {
	if len(args) == 0 {
		return fmt.Errorf("missing tool\nusage: ai-profile <claude|codex> [list|new|rename|delete|run|acp|apply-statusline]")
	}
	if args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		printHelp(io.Out)
		return nil
	}
	if args[0] == "--version" || args[0] == "version" {
		fmt.Fprintln(io.Out, a.Version)
		return nil
	}
	if args[0] == "completion" {
		if len(args) != 2 {
			return fmt.Errorf("usage: ai-profile completion <bash|fish|zsh>")
		}
		script, err := completionScript(args[1])
		if err != nil {
			return err
		}
		fmt.Fprint(io.Out, script)
		return nil
	}
	if args[0] == "__complete" {
		return a.complete(args[1:], io.Out)
	}

	tool := args[0]
	if _, ok := LookupTool(tool); !ok {
		return fmt.Errorf("unknown tool %q (available: claude, codex)", tool)
	}
	action := "list"
	rest := []string{}
	if len(args) > 1 {
		action = args[1]
		rest = args[2:]
	}

	switch action {
	case "list":
		jsonMode := false
		if len(rest) > 0 {
			if len(rest) == 1 && rest[0] == "--json" {
				jsonMode = true
			} else {
				return fmt.Errorf("usage: ai-profile %s list [--json]", tool)
			}
		}
		return a.list(tool, jsonMode, io.Out)
	case "--json":
		if len(rest) != 0 {
			return fmt.Errorf("usage: ai-profile %s [list] --json", tool)
		}
		return a.list(tool, true, io.Out)
	case "new":
		if len(rest) != 1 {
			return fmt.Errorf("usage: ai-profile %s new <name>", tool)
		}
		p, err := a.Service.Create(tool, rest[0])
		if err != nil {
			return err
		}
		fmt.Fprintf(io.Out, "created %s/%s\n%s\n", p.Tool, p.Alias, p.Dir)
		return nil
	case "rename":
		if len(rest) != 2 {
			return fmt.Errorf("usage: ai-profile %s rename <old> <new>", tool)
		}
		p, err := a.Service.Rename(tool, rest[0], rest[1])
		if err != nil {
			return err
		}
		fmt.Fprintf(io.Out, "renamed %s/%s\n%s\n", p.Tool, p.Alias, p.Dir)
		return nil
	case "delete":
		if len(rest) != 1 {
			return fmt.Errorf("usage: ai-profile %s delete <name>", tool)
		}
		p, _, err := a.Service.Profile(tool, rest[0])
		if err != nil {
			return err
		}
		if err := a.Service.ConfirmDelete(io.In, io.Err, p); err != nil {
			return err
		}
		path, err := a.Service.DeleteConfirmed(tool, rest[0])
		if err != nil {
			return err
		}
		fmt.Fprintf(io.Out, "deleted %s/%s\n%s\n", tool, rest[0], path)
		return nil
	case "run":
		if len(rest) < 1 {
			return fmt.Errorf("usage: ai-profile %s run <profile> [args...]", tool)
		}
		return a.Service.Run(ctx, tool, rest[0], rest[1:], ProcessIO{In: io.Stdin, Out: io.Stdout, Err: io.Stderr})
	case "acp":
		if len(rest) < 1 {
			return fmt.Errorf("usage: ai-profile %s acp <profile> [args...]", tool)
		}
		return a.Service.ACP(ctx, tool, rest[0], rest[1:], ProcessIO{In: io.Stdin, Out: io.Stdout, Err: io.Stderr})
	case "apply-statusline":
		if len(rest) < 1 || len(rest) > 2 {
			return fmt.Errorf("usage: ai-profile %s apply-statusline <profile> [template]", tool)
		}
		template := "default"
		if len(rest) == 2 {
			template = rest[1]
		}
		if err := a.Service.ApplyStatusline(tool, rest[0], template); err != nil {
			return err
		}
		fmt.Fprintf(io.Out, "applied statusline %q to %s/%s\n", template, tool, rest[0])
		return nil
	default:
		return fmt.Errorf("unknown action %q", action)
	}
}

type listItem struct {
	Profile string `json:"profile"`
	Env     string `json:"env"`
	Dir     string `json:"dir"`
}

func (a App) list(tool string, jsonMode bool, out io.Writer) error {
	profiles, spec, err := a.Service.List(tool)
	if err != nil {
		return err
	}
	items := make([]listItem, 0, len(profiles))
	for _, p := range profiles {
		items = append(items, listItem{Profile: p.Alias, Env: spec.ConfigEnv, Dir: p.Dir})
	}
	if jsonMode {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(items)
	}
	if len(items) == 0 {
		fmt.Fprintln(out, "No profiles.")
		return nil
	}
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "PROFILE\tENV\tDIR")
	for _, item := range items {
		fmt.Fprintf(w, "%s\t%s\t%s\n", item.Profile, item.Env, item.Dir)
	}
	return w.Flush()
}

func (a App) complete(args []string, out io.Writer) error {
	if len(args) == 0 {
		return nil
	}
	switch args[0] {
	case "tools":
		for _, t := range Tools {
			fmt.Fprintln(out, t.Name)
		}
	case "actions":
		for _, x := range []string{"list", "new", "rename", "delete", "run", "acp", "apply-statusline"} {
			fmt.Fprintln(out, x)
		}
	case "profiles":
		if len(args) != 2 {
			return nil
		}
		profiles, _, err := a.Service.List(args[1])
		if err != nil {
			return nil
		}
		for _, p := range profiles {
			fmt.Fprintln(out, p.Alias)
		}
	case "templates":
		names := []string{"default"}
		if a.Service.TemplateDir != "" {
			entries, _ := os.ReadDir(a.Service.TemplateDir)
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
					names = append(names, strings.TrimSuffix(e.Name(), ".json"))
				}
			}
		}
		sort.Strings(names)
		last := ""
		for _, name := range names {
			if name != last {
				fmt.Fprintln(out, name)
				last = name
			}
		}
	}
	return nil
}

func printHelp(out io.Writer) {
	fmt.Fprint(out, `ai-profile — isolated profiles for AI CLIs

Usage:
  ai-profile <tool> [list] [--json]
  ai-profile <tool> new <name>
  ai-profile <tool> rename <old> <new>
  ai-profile <tool> delete <name>
  ai-profile <tool> run <name> [args...]
  ai-profile <tool> acp <name> [args...]
  ai-profile <tool> apply-statusline <name> [template]
  ai-profile completion <bash|fish|zsh>

Tools: claude, codex
`)
}

func completionScript(shell string) (string, error) {
	switch shell {
	case "bash":
		return `# bash completion for ai-profile
_ai_profile_complete() {
  local cur prev tool action
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"
  if (( COMP_CWORD == 1 )); then
    COMPREPLY=( $(compgen -W "$(ai-profile __complete tools) completion" -- "$cur") )
    return
  fi
  tool="${COMP_WORDS[1]}"
  if [[ "$tool" == "completion" ]]; then
    COMPREPLY=( $(compgen -W "bash fish zsh" -- "$cur") )
    return
  fi
  if (( COMP_CWORD == 2 )); then
    COMPREPLY=( $(compgen -W "$(ai-profile __complete actions)" -- "$cur") )
    return
  fi
  action="${COMP_WORDS[2]}"
  case "$action" in
    run|acp|delete|rename|apply-statusline)
      if (( COMP_CWORD == 3 )); then
        COMPREPLY=( $(compgen -W "$(ai-profile __complete profiles "$tool")" -- "$cur") )
      elif [[ "$action" == "apply-statusline" && $COMP_CWORD -eq 4 ]]; then
        COMPREPLY=( $(compgen -W "$(ai-profile __complete templates)" -- "$cur") )
      fi
      ;;
  esac
}
complete -F _ai_profile_complete ai-profile
`, nil
	case "fish":
		return `# fish completion for ai-profile
complete -c ai-profile -f

function __ai_profile_tokens
  commandline -opc
end

function __ai_profile_at_action
  set -l tokens (__ai_profile_tokens)
  test (count $tokens) -eq 2; and contains -- $tokens[2] (ai-profile __complete tools)
end

function __ai_profile_at_profile
  set -l tokens (__ai_profile_tokens)
  test (count $tokens) -eq 3; and contains -- $tokens[3] run acp delete rename apply-statusline
end

function __ai_profile_at_template
  set -l tokens (__ai_profile_tokens)
  test (count $tokens) -eq 4; and test $tokens[3] = apply-statusline
end

complete -c ai-profile -n '__fish_use_subcommand' -a '(ai-profile __complete tools) completion'
complete -c ai-profile -n '__fish_seen_subcommand_from completion' -a 'bash fish zsh'
complete -c ai-profile -n '__ai_profile_at_action' -a '(ai-profile __complete actions)'
complete -c ai-profile -n '__ai_profile_at_profile' -a '(ai-profile __complete profiles (__ai_profile_tokens)[2])'
complete -c ai-profile -n '__ai_profile_at_template' -a '(ai-profile __complete templates)'
`, nil
	case "zsh":
		return `#compdef ai-profile
_ai_profile() {
  local -a tools actions
  tools=(${(f)"$(ai-profile __complete tools)"})
  actions=(${(f)"$(ai-profile __complete actions)"})
  if (( CURRENT == 2 )); then _describe 'tool' tools; return; fi
  if [[ $words[2] == completion ]]; then _values 'shell' bash fish zsh; return; fi
  if (( CURRENT == 3 )); then _describe 'action' actions; return; fi
  case $words[3] in
    run|acp|delete|rename|apply-statusline)
      if (( CURRENT == 4 )); then
        local -a profiles; profiles=(${(f)"$(ai-profile __complete profiles $words[2])"}); _describe 'profile' profiles
      elif [[ $words[3] == apply-statusline && CURRENT == 5 ]]; then
        local -a templates; templates=(${(f)"$(ai-profile __complete templates)"}); _describe 'template' templates
      fi
      ;;
  esac
}
_ai_profile "$@"
`, nil
	default:
		return "", fmt.Errorf("unsupported shell %q (use bash, fish, or zsh)", shell)
	}
}
