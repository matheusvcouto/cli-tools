package cli

import (
	"fmt"
	"strings"

	"github.com/matheusvcouto/cli-tools/cli/internal/model"
)

// MarkdownReference renders deterministic reference documentation from the compiled graph.
func (c *CompiledApp) MarkdownReference() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", c.graph.Name)
	if c.graph.Summary != "" {
		fmt.Fprintf(&b, "%s\n\n", c.graph.Summary)
	}
	writeMarkdownCommand(&b, c, c.graph.Root, 1)
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func writeMarkdownCommand(b *strings.Builder, app *CompiledApp, cmd *model.Command, depth int) {
	if cmd != app.graph.Root {
		fmt.Fprintf(b, "%s `%s`\n\n", strings.Repeat("#", depth+1), strings.Join(commandPath(cmd), " "))
		if cmd.Summary != "" {
			fmt.Fprintf(b, "%s\n\n", cmd.Summary)
		}
	}
	fmt.Fprintf(b, "```text\n%s```\n\n", app.Help(commandPath(cmd)[1:]...))
	for _, child := range cmd.Children {
		if !child.Hidden {
			writeMarkdownCommand(b, app, child, depth+1)
		}
	}
}

// ManPage emits a minimal portable roff reference generated from the same graph.
func (c *CompiledApp) ManPage(section int) string {
	if section <= 0 {
		section = 1
	}
	var b strings.Builder
	fmt.Fprintf(&b, ".TH %s %d\n.SH NAME\n%s \\- %s\n.SH SYNOPSIS\n.B %s\n", strings.ToUpper(c.graph.Name), section, c.graph.Name, roffEscape(c.graph.Summary), c.graph.Name)
	fmt.Fprintf(&b, ".SH DESCRIPTION\n%s\n", roffEscape(c.graph.Description))
	fmt.Fprintln(&b, ".SH COMMANDS")
	for _, child := range c.graph.Root.Children {
		if child.Hidden {
			continue
		}
		fmt.Fprintf(&b, ".TP\n.B %s\n%s\n", child.Name, roffEscape(child.Summary))
	}
	return b.String()
}

func roffEscape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	if strings.HasPrefix(s, ".") || strings.HasPrefix(s, "'") {
		s = "\\&" + s
	}
	return s
}
