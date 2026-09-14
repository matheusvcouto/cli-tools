package cli

import "fmt"

const (
	builtinHelpCommandID = "cli.builtin.help"
	builtinHelpPathArgID = "cli.builtin.help.path"
)

func builtinHelpCommand() Command {
	return Command{
		ID:      builtinHelpCommandID,
		Name:    "help",
		Summary: "show help for a command",
		Args: []Arg{{
			ID:    builtinHelpPathArgID,
			Name:  "command",
			Value: StringValue(),
			Mode:  ArgVariadic,
		}},
	}
}

func (c *CompiledApp) installHelpHandler() {
	c.handlers[builtinHelpCommandID] = func(inv *Invocation) error {
		path, _ := ValuesAs[string](inv, builtinHelpPathArgID)
		cmd := c.graph.Root
		for _, name := range path {
			next := cmd.ChildByName[name]
			if next == nil || next.Hidden {
				return &Diagnostic{
					Code:        CodeUnknownCommand,
					Kind:        "command",
					Message:     fmt.Sprintf("unknown command %q", name),
					Hint:        suggestionHint(name, childNames(cmd)),
					CommandPath: commandPath(cmd),
					Class:       ExitUsage,
				}
			}
			cmd = next
		}
		_, err := fmt.Fprint(inv.IO.Out, c.Help(commandPath(cmd)[1:]...))
		return err
	}
}
