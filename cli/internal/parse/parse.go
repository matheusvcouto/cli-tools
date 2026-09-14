package parse

import (
	"fmt"
	"strings"

	"github.com/matheusvcouto/cli-tools/cli/internal/model"
)

type Mode uint8

const (
	Strict Mode = iota
	Partial
)

type State struct {
	Command       *model.Command
	Path          []*model.Command
	Values        map[string][]any
	Present       map[string]int
	OptionParsing bool
	Opaque        bool
	CurrentArg    *model.Arg
	AwaitingFlag  *model.Flag
	TokenIndex    int
	Prefix        string
	ValuePrefix   string
}

type ParseError struct {
	Code, Message, RelatedID string
	Command                  *model.Command
	Unknown                  string
	Sensitive                bool
}

func (e *ParseError) Error() string { return e.Message }

func Run(root *model.Command, argv []string, mode Mode) (*State, error) {
	s := &State{Command: root, Path: []*model.Command{root}, Values: map[string][]any{}, Present: map[string]int{}, OptionParsing: true, TokenIndex: -1}
	positional := 0
	var globalsLong = map[string]*model.Flag{}
	var globalsShort = map[rune]*model.Flag{}
	collectGlobals(root, globalsLong, globalsShort)
	for i := 0; i < len(argv); i++ {
		tok := argv[i]
		s.TokenIndex = i
		s.Prefix = tok
		if s.AwaitingFlag != nil {
			if mode == Partial && i == len(argv)-1 {
				s.ValuePrefix = tok
				return s, nil
			}
			if err := bindFlag(s, s.AwaitingFlag, tok); err != nil {
				return nil, err
			}
			s.AwaitingFlag = nil
			continue
		}
		if s.Opaque {
			a := s.CurrentArg
			v, err := a.Value.Parse(tok)
			if err != nil {
				return nil, invalidValue(s.Command, a.ID, a.Name, tok, err, a.Sensitive)
			}
			s.Values[a.ID] = append(s.Values[a.ID], v)
			s.Present[a.ID]++
			continue
		}
		// Once an opaque trailing argument position is reached, parent option
		// parsing stops. Exact child commands still win before the first arg.
		if positional < len(s.Command.Args) && s.Command.Args[positional].Mode == 2 {
			if !(positional == 0 && s.Command.ChildByName[tok] != nil) {
				a := s.Command.Args[positional]
				s.CurrentArg = a
				s.Opaque = true
				s.OptionParsing = false
				v, err := a.Value.Parse(tok)
				if err != nil {
					return nil, invalidValue(s.Command, a.ID, a.Name, tok, err, a.Sensitive)
				}
				s.Values[a.ID] = append(s.Values[a.ID], v)
				s.Present[a.ID]++
				continue
			}
		}
		if s.OptionParsing && tok == "--" {
			s.OptionParsing = false
			continue
		}
		if s.OptionParsing && strings.HasPrefix(tok, "--") && tok != "--" {
			name, val, has := strings.Cut(strings.TrimPrefix(tok, "--"), "=")
			f := lookupLong(s.Command, name, globalsLong)
			if f == nil {
				if mode == Partial && i == len(argv)-1 {
					return s, nil
				}
				return nil, &ParseError{Code: "unknown_flag", Message: fmt.Sprintf("unknown flag --%s", name), Command: s.Command, Unknown: name}
			}
			if f.Action == 1 {
				if has {
					return nil, &ParseError{Code: "invalid_value", Message: fmt.Sprintf("flag --%s does not take a value", name), RelatedID: f.ID, Command: s.Command}
				}
				s.Values[f.ID] = append(s.Values[f.ID], true)
				s.Present[f.ID]++
				continue
			}
			if f.Action == 3 {
				if has {
					return nil, &ParseError{Code: "invalid_value", Message: fmt.Sprintf("flag --%s does not take a value", name), RelatedID: f.ID, Command: s.Command}
				}
				n := int64(s.Present[f.ID] + 1)
				s.Values[f.ID] = []any{n}
				s.Present[f.ID]++
				continue
			}
			if has {
				if mode == Partial && i == len(argv)-1 {
					s.AwaitingFlag = f
					s.ValuePrefix = val
					return s, nil
				}
				if err := bindFlag(s, f, val); err != nil {
					return nil, err
				}
				continue
			}
			if i+1 >= len(argv) {
				s.AwaitingFlag = f
				s.ValuePrefix = ""
				if mode == Partial {
					return s, nil
				}
				return nil, &ParseError{Code: "missing_value", Message: fmt.Sprintf("flag --%s requires a value", name), RelatedID: f.ID, Command: s.Command}
			}
			i++
			if err := bindFlag(s, f, argv[i]); err != nil {
				return nil, err
			}
			continue
		}
		if s.OptionParsing && strings.HasPrefix(tok, "-") && tok != "-" && positional < len(s.Command.Args) {
			a := s.Command.Args[positional]
			if leadingDashPositional(a, tok) {
				s.CurrentArg = a
				v, err := a.Value.Parse(tok)
				if err != nil {
					return nil, invalidValue(s.Command, a.ID, a.Name, tok, err, a.Sensitive)
				}
				s.Values[a.ID] = append(s.Values[a.ID], v)
				s.Present[a.ID]++
				if s.Command.OptionPolicy == "before_args" {
					s.OptionParsing = false
				}
				if a.Mode == 0 {
					positional++
				}
				continue
			}
		}
		if s.OptionParsing && strings.HasPrefix(tok, "-") && tok != "-" {
			rs := []rune(strings.TrimPrefix(tok, "-"))
			if len(rs) != 1 {
				if mode == Partial && i == len(argv)-1 {
					return s, nil
				}
				return nil, &ParseError{Code: "unknown_flag", Message: fmt.Sprintf("unsupported short flag syntax %q", tok), Command: s.Command, Unknown: tok}
			}
			f := lookupShort(s.Command, rs[0], globalsShort)
			if f == nil {
				if mode == Partial && i == len(argv)-1 {
					return s, nil
				}
				return nil, &ParseError{Code: "unknown_flag", Message: fmt.Sprintf("unknown flag -%c", rs[0]), Command: s.Command, Unknown: string(rs[0])}
			}
			if f.Action == 1 {
				s.Values[f.ID] = append(s.Values[f.ID], true)
				s.Present[f.ID]++
				continue
			}
			if f.Action == 3 {
				n := int64(s.Present[f.ID] + 1)
				s.Values[f.ID] = []any{n}
				s.Present[f.ID]++
				continue
			}
			if i+1 >= len(argv) {
				s.AwaitingFlag = f
				s.ValuePrefix = ""
				if mode == Partial {
					return s, nil
				}
				return nil, &ParseError{Code: "missing_value", Message: fmt.Sprintf("flag -%c requires a value", rs[0]), RelatedID: f.ID, Command: s.Command}
			}
			i++
			if err := bindFlag(s, f, argv[i]); err != nil {
				return nil, err
			}
			continue
		}
		if positional == 0 {
			if child := s.Command.ChildByName[tok]; child != nil {
				s.Command = child
				s.Path = append(s.Path, child)
				positional = 0
				continue
			}
		}
		if positional >= len(s.Command.Args) {
			if mode == Partial && i == len(argv)-1 {
				return s, nil
			}
			if len(s.Command.Children) > 0 && positional == 0 {
				return nil, &ParseError{Code: "unknown_command", Message: fmt.Sprintf("unknown command %q", tok), Command: s.Command, Unknown: tok}
			}
			return nil, &ParseError{Code: "unexpected_argument", Message: fmt.Sprintf("unexpected argument %q", tok), Command: s.Command}
		}
		a := s.Command.Args[positional]
		s.CurrentArg = a
		v, err := a.Value.Parse(tok)
		if err != nil {
			return nil, invalidValue(s.Command, a.ID, a.Name, tok, err, a.Sensitive)
		}
		s.Values[a.ID] = append(s.Values[a.ID], v)
		s.Present[a.ID]++
		if s.Command.OptionPolicy == "before_args" {
			s.OptionParsing = false
		}
		if a.Mode == 2 {
			s.Opaque = true
			s.OptionParsing = false
		} else if a.Mode == 0 {
			positional++
		}
	}
	if mode == Strict {
		if s.AwaitingFlag != nil {
			return nil, &ParseError{Code: "missing_value", Message: fmt.Sprintf("flag --%s requires a value", s.AwaitingFlag.Long), RelatedID: s.AwaitingFlag.ID, Command: s.Command}
		}
		for _, a := range s.Command.Args {
			n := s.Present[a.ID]
			min := a.Min
			if a.Required && min < 1 {
				min = 1
			}
			if n < min {
				return nil, &ParseError{Code: "missing_argument", Message: fmt.Sprintf("missing argument <%s>", a.Name), RelatedID: a.ID, Command: s.Command}
			}
			if a.Max >= 0 && n > a.Max {
				return nil, &ParseError{Code: "too_many_arguments", Message: fmt.Sprintf("too many values for <%s>", a.Name), RelatedID: a.ID, Command: s.Command}
			}
		}
	}
	return s, nil
}

func leadingDashPositional(a *model.Arg, token string) bool {
	switch a.Value.TypeName() {
	case "int", "float", "duration", "enum":
		_, err := a.Value.Parse(token)
		return err == nil
	default:
		return false
	}
}

func bindFlag(s *State, f *model.Flag, raw string) error {
	v, err := f.Value.Parse(raw)
	if err != nil {
		return invalidValue(s.Command, f.ID, "--"+f.Long, raw, err, f.Sensitive)
	}
	if f.Action == 2 {
		s.Values[f.ID] = append(s.Values[f.ID], v)
	} else {
		s.Values[f.ID] = []any{v}
	}
	s.Present[f.ID]++
	return nil
}
func invalidValue(c *model.Command, id, name, raw string, err error, sensitive bool) *ParseError {
	if sensitive {
		return &ParseError{Code: "invalid_value", Message: fmt.Sprintf("invalid value for %s", name), RelatedID: id, Command: c, Sensitive: true}
	}
	return &ParseError{Code: "invalid_value", Message: fmt.Sprintf("invalid value %q for %s: %v", raw, name, err), RelatedID: id, Command: c}
}
func collectGlobals(c *model.Command, l map[string]*model.Flag, s map[rune]*model.Flag) {
	for _, f := range c.Flags {
		if f.Global {
			l[f.Long] = f
			for _, a := range f.Aliases {
				l[a] = f
			}
			if f.Short != 0 {
				s[f.Short] = f
			}
		}
	}
}
func lookupLong(c *model.Command, n string, g map[string]*model.Flag) *model.Flag {
	for p := c; p != nil; p = p.Parent {
		if f := p.FlagByLong[n]; f != nil && (p == c || f.Global) {
			return f
		}
	}
	return g[n]
}
func lookupShort(c *model.Command, r rune, g map[rune]*model.Flag) *model.Flag {
	for p := c; p != nil; p = p.Parent {
		if f := p.FlagByShort[r]; f != nil && (p == c || f.Global) {
			return f
		}
	}
	return g[r]
}
func allFlags(c *model.Command) []*model.Flag {
	seen := map[string]struct{}{}
	var out []*model.Flag
	for p := c; p != nil; p = p.Parent {
		for _, f := range p.Flags {
			if p != c && !f.Global {
				continue
			}
			if _, ok := seen[f.ID]; ok {
				continue
			}
			seen[f.ID] = struct{}{}
			out = append(out, f)
		}
	}
	return out
}
