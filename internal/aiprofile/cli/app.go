package profilecli

import (
	"context"
	"encoding/json"
	"fmt"
	"text/tabwriter"

	core "github.com/matheusvcouto/cli-tools/cli"
	"github.com/matheusvcouto/cli-tools/internal/aiprofile"
)

// New compiles ai-profile from one declarative command tree. Domain services
// stay lazy, so static help/version/static completion never touches HOME/store.
func New(service *core.Lazy[*aiprofile.Service], processIO aiprofile.ProcessIO, product core.ProductMetadata) (*core.CompiledApp, error) {
	commands := make([]core.Command, 0, len(aiprofile.Tools))
	for _, tool := range aiprofile.Tools {
		commands = append(commands, toolCommand(tool, service, processIO))
	}
	return core.Compile(core.App{
		ID:       "ai-profile",
		Name:     "ai-profile",
		Summary:  "isolated profiles for AI CLIs",
		Product:  product,
		Builtins: core.Builtins{Help: true, Version: true, Completion: true, Schema: true, Doctor: true},
		Root:     core.Command{ID: "profiles.root", Name: "ai-profile", Commands: commands},
	})
}

func toolCommand(spec aiprofile.ToolSpec, lazy *core.Lazy[*aiprofile.Service], processIO aiprofile.ProcessIO) core.Command {
	tool := spec.Name
	prefix := "profiles." + tool
	jsonID := prefix + ".json"
	list := func(inv *core.Invocation) error {
		service, err := lazy.Get()
		if err != nil {
			return err
		}
		jsonMode, _ := core.ValueAs[bool](inv, jsonID)
		return renderList(service, tool, jsonMode, inv)
	}
	profiles := dynamicProfiles(tool, lazy)

	commands := []core.Command{
		{ID: prefix + ".list", Name: "list", Summary: "list profiles", Handler: list},
		{ID: prefix + ".new", Name: "new", Summary: "create a profile", Args: []core.Arg{{ID: prefix + ".new.name", Name: "name", Value: core.StringValue(), Required: true}}, Handler: func(inv *core.Invocation) error {
			service, err := lazy.Get()
			if err != nil {
				return err
			}
			name, _ := core.ValueAs[string](inv, prefix+".new.name")
			p, err := service.Create(tool, name)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(inv.IO.Out, "created %s/%s\n%s\n", p.Tool, p.Alias, p.Dir)
			return err
		}},
		{ID: prefix + ".rename", Name: "rename", Summary: "rename a profile", Args: []core.Arg{
			{ID: prefix + ".rename.old", Name: "old", Value: core.StringValue(), Required: true, Completer: profiles},
			{ID: prefix + ".rename.new", Name: "new", Value: core.StringValue(), Required: true},
		}, Handler: func(inv *core.Invocation) error {
			service, err := lazy.Get()
			if err != nil {
				return err
			}
			oldName, _ := core.ValueAs[string](inv, prefix+".rename.old")
			newName, _ := core.ValueAs[string](inv, prefix+".rename.new")
			p, err := service.Rename(tool, oldName, newName)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(inv.IO.Out, "renamed %s/%s\n%s\n", p.Tool, p.Alias, p.Dir)
			return err
		}},
		{ID: prefix + ".delete", Name: "delete", Summary: "permanently delete a profile", Args: []core.Arg{{ID: prefix + ".delete.name", Name: "profile", Value: core.StringValue(), Required: true, Completer: profiles}}, Handler: func(inv *core.Invocation) error {
			service, err := lazy.Get()
			if err != nil {
				return err
			}
			name, _ := core.ValueAs[string](inv, prefix+".delete.name")
			p, _, err := service.Profile(tool, name)
			if err != nil {
				return err
			}
			typed, err := inv.Interaction.Text(inv.Context, core.Prompt{
				Message:   fmt.Sprintf("This permanently deletes the profile directory:\n  %s\nType %q to confirm", p.Dir, p.Alias),
				Dangerous: true,
			})
			if err != nil {
				return err
			}
			if typed != p.Alias {
				return &core.Diagnostic{Code: core.CodeInvalidValue, Kind: "confirmation", Message: "confirmation did not match profile name", Class: core.ExitUsage}
			}
			confirmed, err := inv.Interaction.Confirm(inv.Context, core.Prompt{Message: "Delete permanently?", Dangerous: true})
			if err != nil {
				return err
			}
			if !confirmed {
				return &core.Diagnostic{Code: core.CodeInvalidValue, Kind: "confirmation", Message: "deletion cancelled", Class: core.ExitUsage}
			}
			path, err := service.DeleteConfirmed(tool, name)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(inv.IO.Out, "deleted %s/%s\n%s\n", tool, name, path)
			return err
		}},
		{ID: prefix + ".run", Name: "run", Summary: "run the AI CLI with a profile", Args: []core.Arg{
			{ID: prefix + ".run.profile", Name: "profile", Value: core.StringValue(), Required: true, Completer: profiles},
			{ID: prefix + ".run.argv", Name: "args", Value: core.StringValue(), Mode: core.ArgOpaque},
		}, Handler: func(inv *core.Invocation) error {
			service, err := lazy.Get()
			if err != nil {
				return err
			}
			profile, _ := core.ValueAs[string](inv, prefix+".run.profile")
			argv, _ := core.ValuesAs[string](inv, prefix+".run.argv")
			return service.Run(inv.Context, tool, profile, argv, processIO)
		}},
	}

	if spec.ACP != nil {
		commands = append(commands, core.Command{ID: prefix + ".acp", Name: "acp", Summary: "run the ACP adapter with a profile", Args: []core.Arg{
			{ID: prefix + ".acp.profile", Name: "profile", Value: core.StringValue(), Required: true, Completer: profiles},
			{ID: prefix + ".acp.argv", Name: "args", Value: core.StringValue(), Mode: core.ArgOpaque},
		}, Handler: func(inv *core.Invocation) error {
			service, err := lazy.Get()
			if err != nil {
				return err
			}
			profile, _ := core.ValueAs[string](inv, prefix+".acp.profile")
			argv, _ := core.ValuesAs[string](inv, prefix+".acp.argv")
			return service.ACP(inv.Context, tool, profile, argv, processIO)
		}})
	}

	return core.Command{
		ID:       prefix,
		Name:     tool,
		Summary:  "manage " + tool + " profiles",
		Flags:    []core.Flag{{ID: jsonID, Long: "json", Summary: "emit JSON", Action: core.FlagSwitch, Global: true}},
		Handler:  list,
		Commands: commands,
	}
}

func dynamicProfiles(tool string, lazy *core.Lazy[*aiprofile.Service]) core.Completer {
	return func(ctx context.Context, cc core.CompleteContext) ([]core.CompletionCandidate, error) {
		service, err := lazy.Get()
		if err != nil {
			return nil, err
		}
		profiles, _, err := service.List(tool)
		if err != nil {
			return nil, err
		}
		out := make([]core.CompletionCandidate, 0, len(profiles))
		for _, p := range profiles {
			out = append(out, core.CompletionCandidate{Value: p.Alias, Description: tool + " profile", Kind: core.CandidateValue, ID: "profile:" + tool + ":" + p.Alias})
		}
		return out, nil
	}
}

type listItem struct {
	Profile string `json:"profile"`
	Env     string `json:"env"`
	Dir     string `json:"dir"`
}

func renderList(service *aiprofile.Service, tool string, jsonMode bool, inv *core.Invocation) error {
	profiles, spec, err := service.List(tool)
	if err != nil {
		return err
	}
	items := make([]listItem, 0, len(profiles))
	for _, p := range profiles {
		items = append(items, listItem{Profile: p.Alias, Env: spec.ConfigEnv, Dir: p.Dir})
	}
	if jsonMode {
		enc := json.NewEncoder(inv.IO.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(items)
	}
	if len(items) == 0 {
		_, err := fmt.Fprintln(inv.IO.Out, "No profiles.")
		return err
	}
	w := tabwriter.NewWriter(inv.IO.Out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "PROFILE\tENV\tDIR")
	for _, item := range items {
		fmt.Fprintf(w, "%s\t%s\t%s\n", item.Profile, item.Env, item.Dir)
	}
	return w.Flush()
}
