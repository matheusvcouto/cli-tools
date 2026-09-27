package cli

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/matheusvcouto/cli-tools/cli/internal/model"
)

type CompiledApp struct {
	graph        *model.Graph
	handlers     map[string]Handler
	completers   map[string]Completer
	middleware   []Middleware
	builtins     Builtins
	capabilities map[CapabilityID]Capability
	interaction  Interaction
	doctorChecks []DoctorCheck
}

type valueAdapter struct{ Value }

func (v valueAdapter) Parse(s string) (any, error) { return v.Value.parse(s) }
func (v valueAdapter) TypeName() string            { return v.Value.typeName() }
func (v valueAdapter) Hint() string                { return string(v.Value.hint()) }
func (v valueAdapter) Sensitive() bool             { return v.Value.sensitive() }
func (v valueAdapter) Choices() []model.Choice {
	xs := v.Value.choices()
	out := make([]model.Choice, len(xs))
	for i, x := range xs {
		out[i] = model.Choice{Value: x.Value, Description: x.Description}
	}
	return out
}

func Compile(spec App) (*CompiledApp, error) {
	for _, m := range spec.Modules {
		if m == nil {
			continue
		}
		if err := m.Apply(&spec); err != nil {
			return nil, fmt.Errorf("apply module: %w", err)
		}
	}
	if spec.Name == "" {
		return nil, fmt.Errorf("cli: app name is required")
	}
	for i, middleware := range spec.Middleware {
		if middleware == nil {
			return nil, fmt.Errorf("cli: middleware %d is nil", i)
		}
	}
	if err := validateCLIToken("app name", spec.Name); err != nil {
		return nil, err
	}
	if err := validateStableID("app", spec.ID); err != nil {
		return nil, err
	}
	if spec.Root.Name == "" {
		spec.Root.Name = spec.Name
	}
	if err := validateStableID("root command", spec.Root.ID); err != nil {
		return nil, err
	}
	if spec.Builtins.Help {
		spec.Root.Commands = append(spec.Root.Commands, builtinHelpCommand())
	}
	if spec.Builtins.Version {
		spec.Root.Commands = append(spec.Root.Commands, builtinVersionCommand())
	}
	if spec.Builtins.Completion {
		spec.Root.Commands = append(spec.Root.Commands, builtinCompletionCommand())
	}
	if spec.Builtins.Doctor {
		spec.Root.Commands = append(spec.Root.Commands, builtinDoctorCommand())
	}
	c := &CompiledApp{handlers: map[string]Handler{}, completers: map[string]Completer{}, middleware: append([]Middleware(nil), spec.Middleware...), builtins: spec.Builtins, capabilities: map[CapabilityID]Capability{}, interaction: spec.Interaction, doctorChecks: append([]DoctorCheck(nil), spec.DoctorChecks...)}
	for _, cap := range spec.Capabilities {
		if err := validateStableID("capability", string(cap.ID)); err != nil {
			return nil, err
		}
		switch cap.Availability {
		case AvailabilityUnknown, AvailabilityAvailable, AvailabilityUnavailable:
		default:
			return nil, fmt.Errorf("cli: capability %q has invalid availability %d", cap.ID, cap.Availability)
		}
		if _, exists := c.capabilities[cap.ID]; exists {
			return nil, fmt.Errorf("cli: duplicate capability %q", cap.ID)
		}
		c.capabilities[cap.ID] = cap
	}
	doctorIDs := map[string]struct{}{}
	for _, check := range spec.DoctorChecks {
		if err := validateStableID("doctor check", check.ID); err != nil {
			return nil, err
		}
		if check.Check == nil {
			return nil, fmt.Errorf("cli: doctor check %q requires a checker", check.ID)
		}
		if _, exists := doctorIDs[check.ID]; exists {
			return nil, fmt.Errorf("cli: duplicate doctor check %q", check.ID)
		}
		doctorIDs[check.ID] = struct{}{}
	}
	ids := map[string]string{}
	root, err := compileCommand(spec.Root, nil, ids, c)
	if err != nil {
		return nil, err
	}
	if err := validateDeprecationReferences(spec.Root, ids); err != nil {
		return nil, err
	}
	c.graph = &model.Graph{AppID: spec.ID, Name: spec.Name, Summary: spec.Summary, Description: spec.Description, ProductVersion: spec.Product.Version, Stability: spec.Product.Stability, SuiteVersion: spec.Product.SuiteVersion, Root: root}
	if spec.Builtins.Help {
		c.installHelpHandler()
	}
	if spec.Builtins.Version {
		c.installVersionHandlers()
	}
	if spec.Builtins.Completion {
		c.installCompletionHandlers()
	}
	if spec.Builtins.Doctor {
		c.installDoctorHandler()
	}
	return c, nil
}

func compileCommand(in Command, parent *model.Command, ids map[string]string, c *CompiledApp) (*model.Command, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, fmt.Errorf("cli: command name is required")
	}
	if err := validateStableID("command", in.ID); err != nil {
		return nil, err
	}
	if err := validateCLIToken("command name", in.Name); err != nil {
		return nil, err
	}
	commandNames := map[string]struct{}{in.Name: {}}
	for _, alias := range in.Aliases {
		if err := validateCLIToken("command alias", alias); err != nil {
			return nil, err
		}
		if _, exists := commandNames[alias]; exists {
			return nil, fmt.Errorf("cli: command %s declares duplicate name/alias %q", in.Name, alias)
		}
		commandNames[alias] = struct{}{}
	}
	if in.Name == ReservedNamespace || strings.HasPrefix(in.Name, ReservedNamespace+".") {
		return nil, fmt.Errorf("cli: command %q uses reserved namespace %q", in.Name, ReservedNamespace)
	}
	if prev, ok := ids[in.ID]; ok {
		return nil, fmt.Errorf("cli: duplicate stable ID %q (%s and command %s)", in.ID, prev, in.Name)
	}
	ids[in.ID] = "command " + in.Name
	policy := in.OptionPolicy
	if policy == "" {
		policy = OptionsInterspersed
	}
	if policy != OptionsInterspersed && policy != OptionsBeforeArgs {
		return nil, fmt.Errorf("cli: command %s declares unknown option policy %q", in.Name, policy)
	}
	m := &model.Command{ID: in.ID, Name: in.Name, Aliases: dedupe(in.Aliases), Summary: in.Summary, Description: in.Description, Examples: append([]string(nil), in.Examples...), Parent: parent, Hidden: in.Hidden, Experimental: in.Experimental, Deprecated: modelDeprecation(in.Deprecated), OptionPolicy: string(policy), ChildByName: map[string]*model.Command{}, FlagByLong: map[string]*model.Flag{}, FlagByShort: map[rune]*model.Flag{}}
	seenOutputs := map[OutputFormat]struct{}{}
	for _, output := range in.Outputs {
		switch output {
		case OutputHuman, OutputJSON, OutputNDJSON, OutputRaw:
		default:
			return nil, fmt.Errorf("cli: command %s declares unknown output format %q", in.Name, output)
		}
		if _, exists := seenOutputs[output]; exists {
			return nil, fmt.Errorf("cli: command %s declares duplicate output format %q", in.Name, output)
		}
		seenOutputs[output] = struct{}{}
		m.Outputs = append(m.Outputs, string(output))
	}
	if in.Handler != nil {
		c.handlers[in.ID] = in.Handler
	}
	seenArgVariadic := false
	argNames := map[string]struct{}{}
	for idx, a := range in.Args {
		if a.Name == "" || a.Value == nil {
			return nil, fmt.Errorf("cli: command %s arg %d requires name and value", in.Name, idx)
		}
		if _, exists := argNames[a.Name]; exists {
			return nil, fmt.Errorf("cli: command %s declares duplicate argument name %q", in.Name, a.Name)
		}
		argNames[a.Name] = struct{}{}
		switch a.Mode {
		case ArgSingle, ArgVariadic, ArgOpaque:
		default:
			return nil, fmt.Errorf("cli: argument %s declares unknown mode %d", a.Name, a.Mode)
		}
		if err := validateStableID("argument", a.ID); err != nil {
			return nil, err
		}
		if _, ok := ids[a.ID]; ok {
			return nil, fmt.Errorf("cli: duplicate stable ID %q", a.ID)
		}
		ids[a.ID] = "arg " + a.Name
		if seenArgVariadic {
			return nil, fmt.Errorf("cli: command %s has argument after variadic/opaque arg", in.Name)
		}
		if a.Mode == ArgVariadic || a.Mode == ArgOpaque {
			seenArgVariadic = true
		}
		if a.Mode != ArgSingle && idx != len(in.Args)-1 {
			return nil, fmt.Errorf("cli: command %s variadic/opaque arg must be last", in.Name)
		}
		min, max := a.Min, a.Max
		if a.Required && min == 0 {
			min = 1
		}
		if a.Mode == ArgSingle && max == 0 {
			max = 1
		}
		if a.Mode != ArgSingle && max == 0 {
			max = -1
		}
		if min < 0 || max < -1 || (max >= 0 && min > max) {
			return nil, fmt.Errorf("cli: invalid arity for argument %s", a.Name)
		}
		if a.Mode == ArgSingle && (min > 1 || max > 1) {
			return nil, fmt.Errorf("cli: single argument %s cannot have arity greater than one", a.Name)
		}
		ma := &model.Arg{ID: a.ID, Name: a.Name, Summary: a.Summary, Value: valueAdapter{a.Value}, Required: min > 0, Mode: uint8(a.Mode), Min: min, Max: max, Hidden: a.Hidden, Sensitive: a.Sensitive || a.Value.sensitive()}
		m.Args = append(m.Args, ma)
		if a.Completer != nil {
			c.completers[a.ID] = a.Completer
		}
	}
	names := map[string]string{}
	for _, f := range in.Flags {
		if f.Long == "" {
			return nil, fmt.Errorf("cli: command %s flag requires long name", in.Name)
		}
		if err := validateStableID("flag", f.ID); err != nil {
			return nil, err
		}
		if err := validateFlagName("flag long name", f.Long); err != nil {
			return nil, err
		}
		for _, alias := range f.Aliases {
			if err := validateFlagName("flag alias", alias); err != nil {
				return nil, err
			}
		}
		if c.builtins.Help {
			if f.Long == "help" {
				return nil, fmt.Errorf("cli: flag --help is reserved by the help builtin")
			}
			for _, alias := range f.Aliases {
				if alias == "help" {
					return nil, fmt.Errorf("cli: flag alias --help is reserved by the help builtin")
				}
			}
			if f.Short == 'h' {
				return nil, fmt.Errorf("cli: short flag -h is reserved by the help builtin")
			}
		}
		if parent == nil && c.builtins.Version {
			if f.Long == "version" {
				return nil, fmt.Errorf("cli: root flag --version is reserved by the version builtin")
			}
			for _, alias := range f.Aliases {
				if alias == "version" {
					return nil, fmt.Errorf("cli: root flag alias --version is reserved by the version builtin")
				}
			}
		}
		if f.Short != 0 && !unicode.IsLetter(f.Short) && !unicode.IsDigit(f.Short) {
			return nil, fmt.Errorf("cli: invalid short flag %q", f.Short)
		}
		switch f.Action {
		case FlagSet, FlagSwitch, FlagAppend, FlagCount:
		default:
			return nil, fmt.Errorf("cli: flag --%s declares unknown action %d", f.Long, f.Action)
		}
		if _, ok := ids[f.ID]; ok {
			return nil, fmt.Errorf("cli: duplicate stable ID %q", f.ID)
		}
		ids[f.ID] = "flag --" + f.Long
		if prev, ok := names[f.Long]; ok {
			return nil, fmt.Errorf("cli: duplicate flag name --%s (%s)", f.Long, prev)
		}
		names[f.Long] = f.ID
		for _, alias := range f.Aliases {
			if prev, ok := names[alias]; ok {
				return nil, fmt.Errorf("cli: duplicate flag alias --%s (%s)", alias, prev)
			}
			names[alias] = f.ID
		}
		if f.Short != 0 {
			if prev := m.FlagByShort[f.Short]; prev != nil {
				return nil, fmt.Errorf("cli: duplicate short flag -%c", f.Short)
			}
		}
		value := f.Value
		if value == nil && f.Action == FlagSwitch {
			value = BoolValue()
		}
		if value == nil && f.Action == FlagCount {
			value = IntValue()
		}
		if value == nil {
			return nil, fmt.Errorf("cli: flag --%s requires a value codec", f.Long)
		}
		if f.Action == FlagSwitch && value.typeName() != "bool" {
			return nil, fmt.Errorf("cli: switch flag --%s requires a bool value codec", f.Long)
		}
		if f.Action == FlagCount && value.typeName() != "int" {
			return nil, fmt.Errorf("cli: count flag --%s requires an int value codec", f.Long)
		}
		if len(f.Default) > 1 && f.Action != FlagAppend {
			return nil, fmt.Errorf("cli: flag --%s cannot declare multiple defaults unless action is append", f.Long)
		}
		for p := parent; p != nil; p = p.Parent {
			for _, inherited := range p.Flags {
				if !inherited.Global {
					continue
				}
				if inherited.Long == f.Long {
					return nil, fmt.Errorf("cli: flag --%s conflicts with inherited global flag", f.Long)
				}
				for _, alias := range f.Aliases {
					if inherited.Long == alias {
						return nil, fmt.Errorf("cli: flag alias --%s conflicts with inherited global flag", alias)
					}
				}
				for _, alias := range inherited.Aliases {
					if alias == f.Long {
						return nil, fmt.Errorf("cli: flag --%s conflicts with inherited global alias", f.Long)
					}
					for _, own := range f.Aliases {
						if alias == own {
							return nil, fmt.Errorf("cli: flag alias --%s conflicts with inherited global alias", own)
						}
					}
				}
				if f.Short != 0 && inherited.Short == f.Short {
					return nil, fmt.Errorf("cli: short flag -%c conflicts with inherited global flag", f.Short)
				}
			}
		}
		mf := &model.Flag{ID: f.ID, Long: f.Long, Short: f.Short, Aliases: dedupe(f.Aliases), Summary: f.Summary, Value: valueAdapter{value}, Action: uint8(f.Action), Global: f.Global, Required: f.Required, Default: append([]string(nil), f.Default...), Hidden: f.Hidden, Sensitive: f.Sensitive || value.sensitive(), Deprecated: modelDeprecation(f.Deprecated)}
		providerIDs := map[string]struct{}{}
		for _, provider := range f.Providers {
			if !validProviderSource(provider.Source) || strings.TrimSpace(provider.Name) == "" || provider.Resolve == nil {
				return nil, fmt.Errorf("cli: invalid resolution provider for --%s", f.Long)
			}
			key := providerKey(provider)
			if _, exists := providerIDs[key]; exists {
				return nil, fmt.Errorf("cli: duplicate resolution provider %s/%s for --%s", provider.Source, provider.Name, f.Long)
			}
			providerIDs[key] = struct{}{}
			mf.Providers = append(mf.Providers, model.Provider{Source: string(provider.Source), Name: provider.Name, Resolve: provider.Resolve})
		}
		for _, raw := range f.Default {
			if f.Action == FlagSwitch || f.Action == FlagCount {
				return nil, fmt.Errorf("cli: flag --%s cannot declare textual defaults for switch/count action", f.Long)
			}
			if _, err := value.parse(raw); err != nil {
				return nil, fmt.Errorf("cli: invalid default for --%s: %w", f.Long, err)
			}
		}
		m.Flags = append(m.Flags, mf)
		m.FlagByLong[f.Long] = mf
		for _, a := range mf.Aliases {
			m.FlagByLong[a] = mf
		}
		if f.Short != 0 {
			m.FlagByShort[f.Short] = mf
		}
		if f.Completer != nil {
			c.completers[f.ID] = f.Completer
		}
	}
	sort.SliceStable(m.Flags, func(i, j int) bool { return m.Flags[i].Long < m.Flags[j].Long })
	constraintIDs := map[string]struct{}{}
	for _, a := range m.Args {
		constraintIDs[a.ID] = struct{}{}
	}
	for p := m; p != nil; p = p.Parent {
		for _, f := range p.Flags {
			if p == m || f.Global {
				constraintIDs[f.ID] = struct{}{}
			}
		}
	}
	for _, x := range in.Constraints {
		switch x.Kind {
		case Conflicts, Requires, ExactlyOne, AtLeastOne, AllOrNone:
			if len(x.IDs) < 2 {
				return nil, fmt.Errorf("cli: constraint %s requires at least two IDs", x.Kind)
			}
		case MinOccurrences:
			if len(x.IDs) != 1 || x.Min < 1 {
				return nil, fmt.Errorf("cli: min_occurrences requires one ID and min >= 1")
			}
		case MaxOccurrences:
			if len(x.IDs) != 1 || x.Max < 0 {
				return nil, fmt.Errorf("cli: max_occurrences requires one ID and max >= 0")
			}
		case ValuePredicate:
			if len(x.IDs) < 1 || strings.TrimSpace(x.PredicateID) == "" || x.Validate == nil {
				return nil, fmt.Errorf("cli: value_predicate requires IDs, predicate ID and validator")
			}
			if err := validateStableID("constraint predicate", x.PredicateID); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("cli: unknown constraint kind %q", x.Kind)
		}
		seen := map[string]struct{}{}
		for _, id := range x.IDs {
			if _, ok := constraintIDs[id]; !ok {
				return nil, fmt.Errorf("cli: constraint references unknown/non-value ID %q for command %s", id, in.Name)
			}
			if _, ok := seen[id]; ok {
				return nil, fmt.Errorf("cli: constraint %s repeats ID %q", x.Kind, id)
			}
			seen[id] = struct{}{}
		}
		mc := model.Constraint{Kind: string(x.Kind), IDs: append([]string(nil), x.IDs...), Message: x.Message, Min: x.Min, Max: x.Max, PredicateID: x.PredicateID}
		if x.Validate != nil {
			validator := x.Validate
			mc.Validate = func(values map[string][]any) error {
				return validator(ConstraintValues{values: values})
			}
		}
		m.Constraints = append(m.Constraints, mc)
	}
	for _, capID := range in.Capabilities {
		if _, ok := c.capabilities[capID]; !ok {
			return nil, fmt.Errorf("cli: command %s references unknown capability %q", in.Name, capID)
		}
		m.Capabilities = append(m.Capabilities, string(capID))
	}
	reqIDs := map[string]struct{}{}
	for _, r := range in.Requirements {
		if err := validateStableID("requirement", r.ID); err != nil {
			return nil, err
		}
		if r.Check == nil {
			return nil, fmt.Errorf("cli: invalid requirement on %s", in.Name)
		}
		if _, ok := reqIDs[r.ID]; ok {
			return nil, fmt.Errorf("cli: duplicate requirement %q", r.ID)
		}
		reqIDs[r.ID] = struct{}{}
		m.Requirements = append(m.Requirements, model.Requirement{ID: r.ID, Summary: r.Summary, Check: r.Check})
	}
	for p := parent; p != nil; p = p.Parent {
		for _, inherited := range p.Requirements {
			if _, exists := reqIDs[inherited.ID]; exists {
				return nil, fmt.Errorf("cli: requirement %q conflicts with inherited requirement", inherited.ID)
			}
		}
	}
	childNames := map[string]string{}
	for _, child := range in.Commands {
		mc, err := compileCommand(child, m, ids, c)
		if err != nil {
			return nil, err
		}
		all := append([]string{mc.Name}, mc.Aliases...)
		for _, name := range all {
			if name == ReservedNamespace {
				return nil, fmt.Errorf("cli: child command uses reserved namespace")
			}
			if prev, ok := childNames[name]; ok {
				return nil, fmt.Errorf("cli: duplicate command/alias %q (%s, %s)", name, prev, mc.ID)
			}
			childNames[name] = mc.ID
			m.ChildByName[name] = mc
		}
		m.Children = append(m.Children, mc)
	}
	sort.SliceStable(m.Children, func(i, j int) bool { return m.Children[i].Name < m.Children[j].Name })
	return m, nil
}

func validateStableID(kind, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("cli: %s stable ID is required", kind)
	}
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-' || r == ':' {
			continue
		}
		return fmt.Errorf("cli: invalid %s stable ID %q", kind, value)
	}
	if strings.HasPrefix(value, ".") || strings.HasSuffix(value, ".") || strings.Contains(value, "..") {
		return fmt.Errorf("cli: invalid %s stable ID %q", kind, value)
	}
	return nil
}

func validateCLIToken(kind, value string) error {
	if value == "" {
		return fmt.Errorf("cli: %s is required", kind)
	}
	if strings.HasPrefix(value, "-") {
		return fmt.Errorf("cli: invalid %s %q: must not start with '-'", kind, value)
	}
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' {
			continue
		}
		return fmt.Errorf("cli: invalid %s %q", kind, value)
	}
	return nil
}

func validateFlagName(kind, value string) error {
	if err := validateCLIToken(kind, value); err != nil {
		return err
	}
	if strings.ContainsRune(value, '=') {
		return fmt.Errorf("cli: invalid %s %q: '=' is reserved for values", kind, value)
	}
	return nil
}

func dedupe(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, x := range in {
		if x == "" {
			continue
		}
		if _, ok := seen[x]; ok {
			continue
		}
		seen[x] = struct{}{}
		out = append(out, x)
	}
	sort.Strings(out)
	return out
}

func modelDeprecation(d *Deprecation) *model.Deprecation {
	if d == nil {
		return nil
	}
	return &model.Deprecation{Message: d.Message, ReplacementID: d.ReplacementID, RemovalVersion: d.RemovalVersion}
}

func validateDeprecationReferences(cmd Command, ids map[string]string) error {
	check := func(owner string, d *Deprecation) error {
		if d == nil || d.ReplacementID == "" {
			return nil
		}
		if d.ReplacementID == owner {
			return fmt.Errorf("cli: %s deprecation replacement cannot reference itself", owner)
		}
		if _, ok := ids[d.ReplacementID]; !ok {
			return fmt.Errorf("cli: %s deprecation references unknown replacement ID %q", owner, d.ReplacementID)
		}
		return nil
	}
	if err := check(cmd.ID, cmd.Deprecated); err != nil {
		return err
	}
	for _, f := range cmd.Flags {
		if err := check(f.ID, f.Deprecated); err != nil {
			return err
		}
	}
	for _, child := range cmd.Commands {
		if err := validateDeprecationReferences(child, ids); err != nil {
			return err
		}
	}
	return nil
}
