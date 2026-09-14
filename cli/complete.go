package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/matheusvcouto/cli-tools/cli/internal/model"
	iparse "github.com/matheusvcouto/cli-tools/cli/internal/parse"
)

// Complete plans shell-neutral completion using the same parser as execution.
func (c *CompiledApp) Complete(ctx context.Context, req CompletionRequest) (CompletionResult, error) {
	if req.Protocol != CompletionProtocol {
		return CompletionResult{}, &Diagnostic{Code: CodeInvalidValue, Kind: "protocol", Message: fmt.Sprintf("unsupported completion protocol %d", req.Protocol), Class: ExitUsage}
	}
	if req.CursorArg < -1 || req.CursorOffset < 0 {
		return CompletionResult{}, &Diagnostic{Code: CodeInvalidValue, Kind: "protocol", Message: "invalid completion cursor position", Class: ExitUsage}
	}
	raw := append([]string(nil), req.Argv...)
	if req.CursorArg > len(raw) {
		return CompletionResult{}, &Diagnostic{Code: CodeInvalidValue, Kind: "protocol", Message: "completion cursor argument is out of range", Class: ExitUsage}
	}
	if req.CursorArg >= 0 && req.CursorArg < len(raw) && req.CursorOffset > utf8.RuneCountInString(raw[req.CursorArg]) {
		return CompletionResult{}, &Diagnostic{Code: CodeInvalidValue, Kind: "protocol", Message: "completion cursor offset is out of range", Class: ExitUsage}
	}
	if req.CursorArg == len(raw) && req.CursorOffset != 0 {
		return CompletionResult{}, &Diagnostic{Code: CodeInvalidValue, Kind: "protocol", Message: "completion cursor offset must be zero after the last argument", Class: ExitUsage}
	}
	cursor := req.CursorArg
	if cursor < 0 {
		if req.CursorOffset != 0 {
			return CompletionResult{}, &Diagnostic{Code: CodeInvalidValue, Kind: "protocol", Message: "completion cursor offset must be zero when cursor argument is omitted", Class: ExitUsage}
		}
		cursor = len(raw)
	}
	if len(raw) > 0 && raw[0] == c.graph.Name {
		if cursor == 0 {
			return CompletionResult{Protocol: CompletionProtocol, Directive: CompletionDirective{KeepOrder: true}}, nil
		}
		raw = raw[1:]
		cursor--
	}
	if cursor < 0 {
		cursor = 0
	}
	prefix := ""
	if cursor < len(raw) {
		prefix = runePrefix(raw[cursor], req.CursorOffset)
	}
	// Only tokens before the cursor are parsed as complete. The current token is
	// a prefix and must never be rejected by typed validation before completion.
	st, err := iparse.Run(c.graph.Root, raw[:cursor], iparse.Partial)
	if err != nil {
		return CompletionResult{}, c.diagFromParse(err)
	}
	if st.AwaitingFlag != nil {
		return c.completeValue(ctx, st.Command, st.Values, nil, st.AwaitingFlag, prefix)
	}
	if st.Opaque {
		return CompletionResult{Protocol: CompletionProtocol, Directive: CompletionDirective{KeepOrder: true}}, nil
	}
	argForPrefix := nextArg(st.Command, st.Present)
	positionalDashPrefix := st.OptionParsing && argForPrefix != nil && completionDashPositional(argForPrefix, prefix)
	if st.OptionParsing && !positionalDashPrefix && strings.HasPrefix(prefix, "--") {
		q := strings.TrimPrefix(prefix, "--")
		if name, value, ok := strings.Cut(q, "="); ok {
			if f := findLongFlag(st.Command, name); f != nil {
				r, err := c.completeValue(ctx, st.Command, st.Values, nil, f, value)
				if err != nil {
					return CompletionResult{}, err
				}
				for i := range r.Candidates {
					r.Candidates[i].Value = "--" + name + "=" + r.Candidates[i].Value
				}
				r.Directive.NoSpace = true
				return r, nil
			}
		}
		return CompletionResult{Protocol: CompletionProtocol, Candidates: dedupeCandidates(flagCandidates(st.Command, q)), Directive: CompletionDirective{KeepOrder: true}}, nil
	}
	if st.OptionParsing && !positionalDashPrefix && strings.HasPrefix(prefix, "-") && prefix != "-" {
		return CompletionResult{Protocol: CompletionProtocol, Candidates: dedupeCandidates(shortFlagCandidates(st.Command, strings.TrimPrefix(prefix, "-"))), Directive: CompletionDirective{KeepOrder: true}}, nil
	}

	arg := nextArg(st.Command, st.Present)
	var candidates []CompletionCandidate
	if arg == nil || firstPositionalUnbound(st.Command, st.Present) {
		for _, child := range st.Command.Children {
			if child.Hidden || !c.commandStaticallyAvailable(child) {
				continue
			}
			if strings.HasPrefix(child.Name, prefix) {
				candidates = append(candidates, CompletionCandidate{Value: child.Name, Description: child.Summary, Kind: CandidateCommand, ID: child.ID + ":name"})
			}
			for _, alias := range child.Aliases {
				if strings.HasPrefix(alias, prefix) {
					candidates = append(candidates, CompletionCandidate{Value: alias, Description: child.Summary, Kind: CandidateCommand, ID: child.ID + ":alias:" + alias})
				}
			}
		}
	}
	if arg != nil {
		r, err := c.completeValue(ctx, st.Command, st.Values, arg, nil, prefix)
		if err != nil {
			return CompletionResult{}, err
		}
		candidates = append(candidates, r.Candidates...)
		r.Candidates = dedupeCandidates(candidates)
		return r, nil
	}
	if prefix == "" && st.OptionParsing {
		candidates = append(candidates, flagCandidates(st.Command, "")...)
	}
	return CompletionResult{Protocol: CompletionProtocol, Candidates: dedupeCandidates(candidates), Directive: CompletionDirective{KeepOrder: true}}, nil
}

func (c *CompiledApp) completeValue(ctx context.Context, cmd *model.Command, values map[string][]any, arg *model.Arg, flag *model.Flag, prefix string) (CompletionResult, error) {
	var id string
	var v model.Value
	sensitive := false
	if arg != nil {
		id = arg.ID
		v = arg.Value
		sensitive = arg.Sensitive
	} else {
		id = flag.ID
		v = flag.Value
		sensitive = flag.Sensitive
	}
	if sensitive || v.Sensitive() {
		return CompletionResult{Protocol: CompletionProtocol}, nil
	}
	out := CompletionResult{Protocol: CompletionProtocol, Directive: CompletionDirective{KeepOrder: true}}
	switch ValueHint(v.Hint()) {
	case HintPath:
		out.Directive.Files = true
		out.Directive.Directories = true
	case HintFile:
		out.Directive.Files = true
	case HintDirectory:
		out.Directive.Directories = true
	case HintExecutable:
		out.Directive.Executables = true
	}
	for _, x := range v.Choices() {
		if strings.HasPrefix(x.Value, prefix) {
			out.Candidates = append(out.Candidates, CompletionCandidate{Value: x.Value, Description: x.Description, Kind: CandidateValue, ID: id + ":" + x.Value})
		}
	}
	if dynamic := c.completers[id]; dynamic != nil {
		if err := ctx.Err(); err != nil {
			return CompletionResult{}, err
		}
		cc := CompleteContext{CommandID: cmd.ID, Prefix: prefix, values: completionContextValues(cmd, values)}
		if arg != nil {
			cc.ArgID = id
		} else {
			cc.FlagID = id
		}
		xs, err := dynamic(ctx, cc)
		if err != nil {
			return CompletionResult{}, err
		}
		if err := ctx.Err(); err != nil {
			return CompletionResult{}, err
		}
		for _, x := range xs {
			if len(out.Candidates) >= MaxCompletionCandidates {
				break
			}
			if x.Value != "" && strings.HasPrefix(x.Value, prefix) && completionCandidateSafe(x) {
				out.Candidates = append(out.Candidates, x)
			}
		}
	}
	out.Candidates = dedupeCandidates(out.Candidates)
	return out, nil
}

func completionContextValues(cmd *model.Command, values map[string][]any) map[string][]any {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string][]any, len(values))
	for id, xs := range values {
		if completionValueSensitive(cmd, id) {
			continue
		}
		out[id] = append([]any(nil), xs...)
	}
	return out
}

func completionValueSensitive(cmd *model.Command, id string) bool {
	for p := cmd; p != nil; p = p.Parent {
		for _, arg := range p.Args {
			if arg.ID == id {
				return arg.Sensitive || arg.Value.Sensitive()
			}
		}
		for _, flag := range p.Flags {
			if flag.ID == id {
				return flag.Sensitive || flag.Value.Sensitive()
			}
		}
	}
	return false
}

func completionDashPositional(arg *model.Arg, prefix string) bool {
	if !strings.HasPrefix(prefix, "-") || prefix == "-" {
		return false
	}
	switch arg.Value.TypeName() {
	case "int", "float", "duration":
		r := []rune(strings.TrimPrefix(prefix, "-"))
		return len(r) > 0 && (r[0] >= '0' && r[0] <= '9' || r[0] == '.')
	case "enum":
		for _, choice := range arg.Value.Choices() {
			if strings.HasPrefix(choice.Value, prefix) {
				return true
			}
		}
	}
	return false
}

func (c *CompiledApp) commandStaticallyAvailable(cmd *model.Command) bool {
	for _, id := range commandCapabilities(cmd) {
		if cap, ok := c.capabilities[CapabilityID(id)]; ok && cap.Availability == AvailabilityUnavailable {
			return false
		}
	}
	return true
}

func nextArg(c *model.Command, present map[string]int) *model.Arg {
	for _, a := range c.Args {
		n := present[a.ID]
		if a.Mode != 0 {
			return a
		}
		if n == 0 {
			return a
		}
	}
	return nil
}
func firstPositionalUnbound(c *model.Command, p map[string]int) bool {
	if len(c.Args) == 0 {
		return true
	}
	return p[c.Args[0].ID] == 0
}
func findLongFlag(c *model.Command, name string) *model.Flag {
	for p := c; p != nil; p = p.Parent {
		if f := p.FlagByLong[name]; f != nil && (p == c || f.Global) {
			return f
		}
	}
	return nil
}
func flagCandidates(c *model.Command, prefix string) []CompletionCandidate {
	var out []CompletionCandidate
	for _, f := range visibleFlags(c) {
		if strings.HasPrefix(f.Long, prefix) {
			out = append(out, CompletionCandidate{Value: "--" + f.Long, Description: f.Summary, Kind: CandidateFlag, ID: f.ID + ":long"})
		}
		for _, alias := range f.Aliases {
			if strings.HasPrefix(alias, prefix) {
				out = append(out, CompletionCandidate{Value: "--" + alias, Description: f.Summary, Kind: CandidateFlag, ID: f.ID + ":alias:" + alias})
			}
		}
	}
	return out
}
func shortFlagCandidates(c *model.Command, prefix string) []CompletionCandidate {
	var out []CompletionCandidate
	for _, f := range visibleFlags(c) {
		if f.Short == 0 {
			continue
		}
		v := string(f.Short)
		if strings.HasPrefix(v, prefix) {
			out = append(out, CompletionCandidate{Value: "-" + v, Description: f.Summary, Kind: CandidateFlag, ID: f.ID})
		}
	}
	return out
}
func dedupeCandidates(in []CompletionCandidate) []CompletionCandidate {
	seen := map[string]struct{}{}
	capacity := len(in)
	if capacity > MaxCompletionCandidates {
		capacity = MaxCompletionCandidates
	}
	out := make([]CompletionCandidate, 0, capacity)
	for _, x := range in {
		if len(out) >= MaxCompletionCandidates || !completionCandidateSafe(x) {
			continue
		}
		k := x.ID
		if k == "" {
			k = x.Value
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, x)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Value < out[j].Value })
	return out
}

func runePrefix(s string, count int) string {
	if count <= 0 {
		return ""
	}
	runes := []rune(s)
	if count >= len(runes) {
		return s
	}
	return string(runes[:count])
}

func completionCandidateSafe(c CompletionCandidate) bool {
	return !strings.ContainsRune(c.Value, '\x00') &&
		!strings.ContainsRune(c.Label, '\x00') &&
		!strings.ContainsRune(c.Description, '\x00') &&
		!strings.ContainsRune(c.Group, '\x00') &&
		!strings.ContainsRune(c.ID, '\x00')
}
