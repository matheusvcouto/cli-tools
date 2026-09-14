package cli

import (
	"encoding/json"
	"sort"

	"github.com/matheusvcouto/cli-tools/cli/internal/model"
)

const SchemaVersion = 1

type Schema struct {
	Version        int           `json:"schema_version"`
	AppID          string        `json:"app_id"`
	Name           string        `json:"name"`
	ProductVersion string        `json:"product_version,omitempty"`
	Root           SchemaCommand `json:"root"`
}

type SchemaDeprecation struct {
	Message        string `json:"message,omitempty"`
	ReplacementID  string `json:"replacement_id,omitempty"`
	RemovalVersion string `json:"removal_version,omitempty"`
}

type SchemaConstraint struct {
	Kind        string   `json:"kind"`
	IDs         []string `json:"ids"`
	Message     string   `json:"message,omitempty"`
	Min         int      `json:"min,omitempty"`
	Max         int      `json:"max,omitempty"`
	PredicateID string   `json:"predicate_id,omitempty"`
}

type SchemaRequirement struct {
	ID      string `json:"id"`
	Summary string `json:"summary,omitempty"`
}

type SchemaValueSource struct {
	Source string `json:"source"`
	Name   string `json:"name"`
}

type SchemaCommand struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	Aliases      []string            `json:"aliases,omitempty"`
	Summary      string              `json:"summary,omitempty"`
	Description  string              `json:"description,omitempty"`
	Examples     []string            `json:"examples,omitempty"`
	Args         []SchemaArg         `json:"args,omitempty"`
	Flags        []SchemaFlag        `json:"flags,omitempty"`
	Constraints  []SchemaConstraint  `json:"constraints,omitempty"`
	Requirements []SchemaRequirement `json:"requirements,omitempty"`
	Capabilities []string            `json:"capabilities,omitempty"`
	Outputs      []string            `json:"outputs,omitempty"`
	OptionPolicy string              `json:"option_policy"`
	Commands     []SchemaCommand     `json:"commands,omitempty"`
	Hidden       bool                `json:"hidden,omitempty"`
	Experimental bool                `json:"experimental,omitempty"`
	Deprecated   *SchemaDeprecation  `json:"deprecated,omitempty"`
}

type SchemaArg struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Summary   string   `json:"summary,omitempty"`
	Type      string   `json:"type"`
	Hint      string   `json:"hint,omitempty"`
	Required  bool     `json:"required,omitempty"`
	Mode      string   `json:"mode,omitempty"`
	Min       int      `json:"min,omitempty"`
	Max       int      `json:"max,omitempty"`
	Choices   []Choice `json:"choices,omitempty"`
	Hidden    bool     `json:"hidden,omitempty"`
	Sensitive bool     `json:"sensitive,omitempty"`
}

type SchemaFlag struct {
	ID         string              `json:"id"`
	Long       string              `json:"long"`
	Summary    string              `json:"summary,omitempty"`
	Short      string              `json:"short,omitempty"`
	Aliases    []string            `json:"aliases,omitempty"`
	Type       string              `json:"type"`
	Hint       string              `json:"hint,omitempty"`
	Action     string              `json:"action,omitempty"`
	Global     bool                `json:"global,omitempty"`
	Required   bool                `json:"required,omitempty"`
	Defaults   []string            `json:"defaults,omitempty"`
	Sources    []SchemaValueSource `json:"sources,omitempty"`
	Choices    []Choice            `json:"choices,omitempty"`
	Hidden     bool                `json:"hidden,omitempty"`
	Sensitive  bool                `json:"sensitive,omitempty"`
	Deprecated *SchemaDeprecation  `json:"deprecated,omitempty"`
}

func (c *CompiledApp) Schema() Schema {
	return Schema{Version: SchemaVersion, AppID: c.graph.AppID, Name: c.graph.Name, ProductVersion: c.graph.ProductVersion, Root: schemaCommand(c.graph.Root)}
}

func (c *CompiledApp) SchemaJSON() ([]byte, error) {
	b, err := json.MarshalIndent(c.Schema(), "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func schemaCommand(c *model.Command) SchemaCommand {
	s := SchemaCommand{
		ID: c.ID, Name: c.Name, Aliases: append([]string(nil), c.Aliases...), Summary: c.Summary,
		Description: c.Description, Examples: append([]string(nil), c.Examples...), Hidden: c.Hidden,
		Experimental: c.Experimental, Deprecated: schemaDeprecation(c.Deprecated),
		Capabilities: append([]string(nil), c.Capabilities...), Outputs: append([]string(nil), c.Outputs...), OptionPolicy: c.OptionPolicy,
	}
	for _, a := range c.Args {
		sa := SchemaArg{ID: a.ID, Name: a.Name, Summary: a.Summary, Type: a.Value.TypeName(), Hint: a.Value.Hint(), Required: a.Required, Min: a.Min, Max: a.Max, Hidden: a.Hidden, Sensitive: a.Sensitive}
		switch a.Mode {
		case 1:
			sa.Mode = "variadic"
		case 2:
			sa.Mode = "opaque"
		}
		if !a.Sensitive {
			for _, x := range a.Value.Choices() {
				sa.Choices = append(sa.Choices, Choice{Value: x.Value, Description: x.Description})
			}
		}
		s.Args = append(s.Args, sa)
	}
	for _, f := range c.Flags {
		sf := SchemaFlag{ID: f.ID, Long: f.Long, Summary: f.Summary, Aliases: append([]string(nil), f.Aliases...), Type: f.Value.TypeName(), Hint: f.Value.Hint(), Action: flagActionName(f.Action), Global: f.Global, Required: f.Required, Hidden: f.Hidden, Sensitive: f.Sensitive, Deprecated: schemaDeprecation(f.Deprecated)}
		if f.Short != 0 {
			sf.Short = string(f.Short)
		}
		if !f.Sensitive {
			sf.Defaults = append([]string(nil), f.Default...)
			for _, x := range f.Value.Choices() {
				sf.Choices = append(sf.Choices, Choice{Value: x.Value, Description: x.Description})
			}
		}
		for _, provider := range f.Providers {
			sf.Sources = append(sf.Sources, SchemaValueSource{Source: provider.Source, Name: provider.Name})
		}
		s.Flags = append(s.Flags, sf)
	}
	for _, x := range c.Constraints {
		s.Constraints = append(s.Constraints, SchemaConstraint{Kind: x.Kind, IDs: append([]string(nil), x.IDs...), Message: x.Message, Min: x.Min, Max: x.Max, PredicateID: x.PredicateID})
	}
	for _, r := range c.Requirements {
		s.Requirements = append(s.Requirements, SchemaRequirement{ID: r.ID, Summary: r.Summary})
	}
	for _, x := range c.Children {
		s.Commands = append(s.Commands, schemaCommand(x))
	}
	sort.Strings(s.Capabilities)
	sort.Strings(s.Outputs)
	sort.SliceStable(s.Commands, func(i, j int) bool { return s.Commands[i].Name < s.Commands[j].Name })
	return s
}

func schemaDeprecation(d *model.Deprecation) *SchemaDeprecation {
	if d == nil {
		return nil
	}
	return &SchemaDeprecation{Message: d.Message, ReplacementID: d.ReplacementID, RemovalVersion: d.RemovalVersion}
}
func flagActionName(action uint8) string {
	switch FlagAction(action) {
	case FlagSwitch:
		return "switch"
	case FlagAppend:
		return "append"
	case FlagCount:
		return "count"
	default:
		return "set"
	}
}
