package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/matheusvcouto/cli-tools/cli/internal/model"
	iparse "github.com/matheusvcouto/cli-tools/cli/internal/parse"
)

func (c *CompiledApp) Run(ctx context.Context, argv []string, streams IO) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if streams.In == nil {
		streams.In = strings.NewReader("")
	}
	if streams.Out == nil {
		streams.Out = io.Discard
	}
	if streams.Err == nil {
		streams.Err = io.Discard
	}
	if len(argv) > 0 && argv[0] == ReservedNamespace {
		return c.runMachine(ctx, argv[1:], streams)
	}
	if len(argv) > 0 && argv[0] == "--version" && c.builtins.Version {
		if len(argv) != 1 {
			return &Diagnostic{Code: CodeInvalidValue, Kind: "usage", Message: "--version does not accept arguments", Class: ExitUsage}
		}
		_, err := fmt.Fprintln(streams.Out, c.versionLine())
		return err
	}
	if c.builtins.Help {
		cmd, ok, err := c.helpCommand(argv)
		if err != nil {
			return err
		}
		if ok {
			_, err = fmt.Fprint(streams.Out, c.Help(commandPath(cmd)[1:]...))
			return err
		}
	}
	st, err := iparse.Run(c.graph.Root, argv, iparse.Strict)
	if err != nil {
		return c.diagFromParse(err)
	}
	if err := validateConstraints(st.Command, st.Present); err != nil {
		return err
	}
	raw, err := resolveInvocationValues(ctx, st.Command, st.Values, st.Present)
	if err != nil {
		return err
	}
	if err := validateResolvedConstraints(st.Command, raw); err != nil {
		return err
	}
	for _, capID := range commandCapabilities(st.Command) {
		cap := c.capabilities[CapabilityID(capID)]
		if cap.Availability != AvailabilityAvailable {
			msg := fmt.Sprintf("capability %s is unavailable", capID)
			if cap.Summary != "" {
				msg += ": " + cap.Summary
			}
			return &Diagnostic{Code: CodeUnavailable, Kind: "capability", Message: msg, RelatedID: capID, Class: ExitUnavailable, CommandPath: pathNames(st.Path)}
		}
	}
	for _, req := range commandRequirements(st.Command) {
		if err := req.Check(ctx); err != nil {
			return &Diagnostic{Code: CodeRequirement, Kind: "requirement", Message: fmt.Sprintf("requirement %s is not satisfied: %v", req.ID, err), RelatedID: req.ID, Class: ExitUnavailable, Cause: err, CommandPath: pathNames(st.Path)}
		}
	}
	h := c.handlers[st.Command.ID]
	if h == nil {
		if len(st.Command.Children) > 0 {
			return &Diagnostic{Code: CodeMissingArgument, Kind: "usage", Message: "missing command", Hint: "use --help to list available commands", Class: ExitUsage, CommandPath: pathNames(st.Path)}
		}
		return nil
	}
	for i := len(c.middleware) - 1; i >= 0; i-- {
		h = c.middleware[i](h)
	}
	interaction := c.interaction
	if interaction == nil {
		interaction = TextInteraction{In: streams.In, Out: streams.Err, Interactive: streams.Terminal.StdinTTY}
	}
	inv := &Invocation{Context: ctx, IO: streams, CommandID: st.Command.ID, CommandPath: pathNames(st.Path), Terminal: streams.Terminal, Interaction: interaction, raw: raw, capabilities: c.capabilities}
	return h(inv)
}

func resolveInvocationValues(ctx context.Context, command *model.Command, parsed map[string][]any, present map[string]int) (map[string][]ResolvedValue, error) {
	resolved := map[string][]ResolvedValue{}
	for id, xs := range parsed {
		for _, value := range xs {
			resolved[id] = append(resolved[id], ResolvedValue{Value: value, Source: SourceCLI})
		}
	}
	for _, flag := range effectiveFlags(command) {
		if present[flag.ID] > 0 {
			continue
		}
		found := false
		for _, provider := range flag.Providers {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			rawValues, ok, err := provider.Resolve(ctx)
			if err != nil {
				if flag.Sensitive {
					return nil, &Diagnostic{Code: CodeResolution, Kind: "resolution", Message: fmt.Sprintf("failed to resolve sensitive value for --%s from %s %q", flag.Long, provider.Source, provider.Name), RelatedID: flag.ID, Class: ExitExecution, Sensitive: true, CommandPath: commandPath(command)}
				}
				return nil, &Diagnostic{Code: CodeResolution, Kind: "resolution", Message: fmt.Sprintf("resolve --%s from %s %q: %v", flag.Long, provider.Source, provider.Name, err), RelatedID: flag.ID, Class: ExitExecution, Cause: err, CommandPath: commandPath(command)}
			}
			if !ok {
				continue
			}
			values, err := parseResolvedFlagValues(flag, rawValues)
			if err != nil {
				if flag.Sensitive {
					return nil, &Diagnostic{Code: CodeInvalidValue, Kind: "resolution", Message: fmt.Sprintf("invalid sensitive value for --%s from %s %q", flag.Long, provider.Source, provider.Name), RelatedID: flag.ID, Class: ExitUsage, Sensitive: true, CommandPath: commandPath(command)}
				}
				return nil, &Diagnostic{Code: CodeInvalidValue, Kind: "resolution", Message: fmt.Sprintf("invalid value for --%s from %s %q: %v", flag.Long, provider.Source, provider.Name, err), RelatedID: flag.ID, Class: ExitUsage, Cause: err, CommandPath: commandPath(command)}
			}
			for _, value := range values {
				resolved[flag.ID] = append(resolved[flag.ID], ResolvedValue{Value: value, Source: ValueSource(provider.Source), SourceName: provider.Name})
			}
			found = true
			break
		}
		if !found && len(flag.Default) > 0 {
			values, err := parseResolvedFlagValues(flag, flag.Default)
			if err != nil {
				return nil, fmt.Errorf("invalid compiled default for --%s: %w", flag.Long, err)
			}
			for _, value := range values {
				resolved[flag.ID] = append(resolved[flag.ID], ResolvedValue{Value: value, Source: SourceDefault, SourceName: "--" + flag.Long})
			}
		}
		if flag.Required && len(resolved[flag.ID]) == 0 {
			return nil, &Diagnostic{Code: CodeMissingValue, Kind: "usage", Message: fmt.Sprintf("required flag --%s is missing", flag.Long), RelatedID: flag.ID, Class: ExitUsage, CommandPath: commandPath(command)}
		}
	}
	return resolved, nil
}

func effectiveFlags(c *model.Command) []*model.Flag {
	seen := map[string]struct{}{}
	var out []*model.Flag
	for p := c; p != nil; p = p.Parent {
		for _, flag := range p.Flags {
			if p != c && !flag.Global {
				continue
			}
			if _, exists := seen[flag.ID]; exists {
				continue
			}
			seen[flag.ID] = struct{}{}
			out = append(out, flag)
		}
	}
	return out
}

func parseResolvedFlagValues(flag *model.Flag, raw []string) ([]any, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("provider returned no values")
	}
	if flag.Action != uint8(FlagAppend) && len(raw) != 1 {
		return nil, fmt.Errorf("expected exactly one value, got %d", len(raw))
	}
	values := make([]any, 0, len(raw))
	for _, text := range raw {
		value, err := flag.Value.Parse(text)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func (c *CompiledApp) helpCommand(argv []string) (*model.Command, bool, error) {
	for i, tok := range argv {
		if tok != "-h" && tok != "--help" {
			continue
		}
		st, err := iparse.Run(c.graph.Root, argv[:i], iparse.Partial)
		if err != nil {
			return nil, false, c.diagFromParse(err)
		}
		// Help is only active while the wrapper is still parsing options. An
		// opaque/passthrough argument or a pending flag value owns the token.
		next := nextArg(st.Command, st.Present)
		if !st.OptionParsing || st.Opaque || st.AwaitingFlag != nil || (next != nil && next.Mode == uint8(ArgOpaque)) {
			return nil, false, nil
		}
		return st.Command, true, nil
	}
	return nil, false, nil
}

func commandCapabilities(c *model.Command) []string {
	seen := map[string]struct{}{}
	var rev []*model.Command
	for p := c; p != nil; p = p.Parent {
		rev = append(rev, p)
	}
	var out []string
	for i := len(rev) - 1; i >= 0; i-- {
		for _, id := range rev[i].Capabilities {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out
}
func commandRequirements(c *model.Command) []model.Requirement {
	seen := map[string]struct{}{}
	var rev []*model.Command
	for p := c; p != nil; p = p.Parent {
		rev = append(rev, p)
	}
	var out []model.Requirement
	for i := len(rev) - 1; i >= 0; i-- {
		for _, r := range rev[i].Requirements {
			if _, ok := seen[r.ID]; ok {
				continue
			}
			seen[r.ID] = struct{}{}
			out = append(out, r)
		}
	}
	return out
}

func (c *CompiledApp) diagFromParse(err error) error {
	p, ok := err.(*iparse.ParseError)
	if !ok {
		return &Diagnostic{Code: CodeInternal, Kind: "internal", Message: err.Error(), Class: ExitExecution, Cause: err}
	}
	d := &Diagnostic{Message: p.Message, RelatedID: p.RelatedID, Class: ExitUsage, Cause: err, Sensitive: p.Sensitive}
	if p.Command != nil {
		d.CommandPath = commandPath(p.Command)
	}
	switch p.Code {
	case "unknown_command":
		d.Code = CodeUnknownCommand
		d.Kind = "command"
		d.Hint = suggestionHint(p.Unknown, childNames(p.Command))
	case "unknown_flag":
		d.Code = CodeUnknownFlag
		d.Kind = "flag"
		d.Hint = suggestionHint(strings.TrimLeft(p.Unknown, "-"), flagNames(p.Command))
	case "missing_value":
		d.Code = CodeMissingValue
		d.Kind = "usage"
	case "missing_argument":
		d.Code = CodeMissingArgument
		d.Kind = "usage"
	case "invalid_value":
		d.Code = CodeInvalidValue
		d.Kind = "value"
	default:
		d.Code = CodeInvalidValue
		d.Kind = "usage"
	}
	return d
}

func validateConstraints(c *model.Command, present map[string]int) error {
	for _, x := range c.Constraints {
		n := 0
		for _, id := range x.IDs {
			if present[id] > 0 {
				n++
			}
		}
		bad := false
		switch ConstraintKind(x.Kind) {
		case Conflicts:
			bad = n > 1
		case Requires:
			bad = present[x.IDs[0]] > 0 && n < len(x.IDs)
		case ExactlyOne:
			bad = n != 1
		case AtLeastOne:
			bad = n < 1
		case AllOrNone:
			bad = n != 0 && n != len(x.IDs)
		case MinOccurrences:
			bad = present[x.IDs[0]] < x.Min
		case MaxOccurrences:
			bad = present[x.IDs[0]] > x.Max
		case ValuePredicate:
			continue
		}
		if bad {
			return &Diagnostic{Code: CodeConstraint, Kind: "constraint", Message: constraintMessage(ConstraintKind(x.Kind), x.IDs, x.Message), Class: ExitUsage, CommandPath: commandPath(c)}
		}
	}
	return nil
}

func validateResolvedConstraints(c *model.Command, resolved map[string][]ResolvedValue) error {
	for _, x := range c.Constraints {
		if ConstraintKind(x.Kind) != ValuePredicate || x.Validate == nil {
			continue
		}
		raw := make(map[string][]any, len(resolved))
		for id, xs := range resolved {
			for _, rv := range xs {
				raw[id] = append(raw[id], rv.Value)
			}
		}
		if err := x.Validate(raw); err != nil {
			if constraintReferencesSensitiveValue(c, x.IDs) {
				return &Diagnostic{Code: CodeConstraint, Kind: "constraint", Message: fmt.Sprintf("constraint %s failed for sensitive value", x.PredicateID), RelatedID: x.PredicateID, Class: ExitUsage, Sensitive: true, CommandPath: commandPath(c)}
			}
			msg := x.Message
			if msg == "" {
				msg = fmt.Sprintf("constraint %s failed: %v", x.PredicateID, err)
			}
			return &Diagnostic{Code: CodeConstraint, Kind: "constraint", Message: msg, RelatedID: x.PredicateID, Class: ExitUsage, Cause: err, CommandPath: commandPath(c)}
		}
	}
	return nil
}

func constraintReferencesSensitiveValue(c *model.Command, ids []string) bool {
	wanted := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wanted[id] = struct{}{}
	}
	for _, arg := range c.Args {
		if _, ok := wanted[arg.ID]; ok && arg.Sensitive {
			return true
		}
	}
	for _, flag := range effectiveFlags(c) {
		if _, ok := wanted[flag.ID]; ok && flag.Sensitive {
			return true
		}
	}
	return false
}
func pathNames(xs []*model.Command) []string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = x.Name
	}
	return out
}
func commandPath(c *model.Command) []string {
	var rev []string
	for p := c; p != nil; p = p.Parent {
		rev = append(rev, p.Name)
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev
}
func childNames(c *model.Command) []string {
	if c == nil {
		return nil
	}
	out := make([]string, 0, len(c.Children))
	for _, x := range c.Children {
		if !x.Hidden {
			out = append(out, x.Name)
		}
	}
	return out
}
func flagNames(c *model.Command) []string {
	if c == nil {
		return nil
	}
	var out []string
	seen := map[string]struct{}{}
	for p := c; p != nil; p = p.Parent {
		for _, f := range p.Flags {
			if p != c && !f.Global {
				continue
			}
			if f.Hidden {
				continue
			}
			if _, ok := seen[f.Long]; ok {
				continue
			}
			seen[f.Long] = struct{}{}
			out = append(out, f.Long)
		}
	}
	return out
}
func suggestionHint(bad string, candidates []string) string {
	best := ""
	score := 3
	for _, x := range candidates {
		d := editDistance(bad, x)
		if strings.HasPrefix(x, bad) || strings.HasPrefix(bad, x) {
			d = 1
		}
		if d < score {
			score = d
			best = x
		}
	}
	if best == "" {
		return ""
	}
	return fmt.Sprintf("did you mean %q?", best)
}
func editDistance(a, b string) int {
	ar, br := []rune(a), []rune(b)
	prev := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i, x := range ar {
		cur := make([]int, len(br)+1)
		cur[0] = i + 1
		for j, y := range br {
			cost := 0
			if x != y {
				cost = 1
			}
			cur[j+1] = min3(cur[j]+1, prev[j+1]+1, prev[j]+cost)
		}
		prev = cur
	}
	return prev[len(br)]
}
func min3(a, b, c int) int {
	if a > b {
		a = b
	}
	if a > c {
		a = c
	}
	return a
}
