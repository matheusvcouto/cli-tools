package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/matheusvcouto/cli-tools/cli/internal/model"
)

func (c *CompiledApp) Help(path ...string) string {
	cmd := c.graph.Root
	for _, name := range path {
		if x := cmd.ChildByName[name]; x != nil {
			cmd = x
		} else {
			break
		}
	}
	var b strings.Builder
	title := cmd.Name
	if cmd.Summary != "" {
		fmt.Fprintf(&b, "%s — %s\n\n", title, cmd.Summary)
	} else {
		fmt.Fprintf(&b, "%s\n\n", title)
	}
	fmt.Fprintf(&b, "Usage:\n  %s", strings.Join(commandPath(cmd), " "))
	if len(cmd.Children) > 0 {
		if c.handlers[cmd.ID] != nil {
			b.WriteString(" [command]")
		} else {
			b.WriteString(" <command>")
		}
	}
	for _, a := range cmd.Args {
		if a.Required {
			fmt.Fprintf(&b, " <%s>", a.Name)
		} else {
			fmt.Fprintf(&b, " [%s]", a.Name)
		}
		if a.Mode != 0 {
			b.WriteString("...")
		}
	}
	if len(cmd.Flags) > 0 || hasGlobal(cmd) {
		b.WriteString(" [options]")
	}
	b.WriteString("\n")
	if cmd.Description != "" {
		fmt.Fprintf(&b, "\n%s\n", cmd.Description)
	}
	if len(cmd.Children) > 0 {
		b.WriteString("\nCommands:\n")
		for _, x := range cmd.Children {
			if !x.Hidden {
				fmt.Fprintf(&b, "  %-18s %s\n", x.Name, x.Summary)
			}
		}
	}
	flags := visibleFlags(cmd)
	if len(flags) > 0 || c.builtins.Help || (cmd == c.graph.Root && c.builtins.Version) {
		b.WriteString("\nOptions:\n")
		for _, f := range flags {
			names := "--" + f.Long
			if f.Short != 0 {
				names = fmt.Sprintf("-%c, %s", f.Short, names)
			}
			if f.Action != 1 && f.Action != 3 {
				names += " <" + f.Value.TypeName() + ">"
			}
			fmt.Fprintf(&b, "  %-24s %s\n", names, f.Summary)
		}
		if c.builtins.Help {
			fmt.Fprintf(&b, "  %-24s %s\n", "-h, --help", "Show help")
		}
		if cmd == c.graph.Root && c.builtins.Version {
			fmt.Fprintf(&b, "  %-24s %s\n", "--version", "Show version")
		}
	}
	return b.String()
}
func hasGlobal(c *model.Command) bool {
	for p := c.Parent; p != nil; p = p.Parent {
		for _, f := range p.Flags {
			if f.Global && !f.Hidden {
				return true
			}
		}
	}
	return false
}
func visibleFlags(c *model.Command) []*model.Flag {
	seen := map[string]struct{}{}
	var out []*model.Flag
	for p := c; p != nil; p = p.Parent {
		for _, f := range p.Flags {
			if p != c && !f.Global {
				continue
			}
			if f.Hidden {
				continue
			}
			if _, ok := seen[f.ID]; ok {
				continue
			}
			seen[f.ID] = struct{}{}
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Long < out[j].Long })
	return out
}
