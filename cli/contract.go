package cli

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
)

const ContractVersion = 1

type Contract struct {
	Version            int           `json:"contract_version"`
	SchemaVersion      int           `json:"schema_version"`
	CompletionProtocol int           `json:"completion_protocol"`
	AppID              string        `json:"app_id"`
	Name               string        `json:"name"`
	Root               SchemaCommand `json:"root"`
}

func (c *CompiledApp) Contract() Contract {
	s := c.Schema()
	return Contract{Version: ContractVersion, SchemaVersion: SchemaVersion, CompletionProtocol: CompletionProtocol, AppID: s.AppID, Name: s.Name, Root: s.Root}
}
func (c *CompiledApp) ContractJSON() ([]byte, error) {
	b, err := json.MarshalIndent(c.Contract(), "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

type ChangeSeverity string

const (
	ChangeBreaking ChangeSeverity = "breaking"
	ChangeAdditive ChangeSeverity = "additive"
	ChangeMetadata ChangeSeverity = "metadata"
)

type ContractChange struct {
	Severity ChangeSeverity `json:"severity"`
	ID       string         `json:"id"`
	Message  string         `json:"message"`
}

// DiffContracts compares stable IDs and classifies syntax changes separately
// from metadata that requires explicit semantic review.
func DiffContracts(oldC, newC Contract) []ContractChange {
	var out []ContractChange
	if oldC.Version != newC.Version {
		out = append(out, change(ChangeBreaking, "$contract", fmt.Sprintf("changed contract format version from %d to %d", oldC.Version, newC.Version)))
	}
	if oldC.SchemaVersion != newC.SchemaVersion {
		out = append(out, change(ChangeBreaking, "$schema", fmt.Sprintf("changed CLI schema version from %d to %d", oldC.SchemaVersion, newC.SchemaVersion)))
	}
	if oldC.CompletionProtocol != newC.CompletionProtocol {
		out = append(out, change(ChangeBreaking, "$completion", fmt.Sprintf("changed completion protocol from %d to %d", oldC.CompletionProtocol, newC.CompletionProtocol)))
	}
	if oldC.AppID != newC.AppID {
		out = append(out, change(ChangeBreaking, "$app", fmt.Sprintf("changed app ID from %q to %q", oldC.AppID, newC.AppID)))
	}
	if oldC.Name != newC.Name {
		out = append(out, change(ChangeBreaking, "$app", fmt.Sprintf("changed app name from %q to %q", oldC.Name, newC.Name)))
	}
	oldItems := flattenContract(oldC.Root)
	newItems := flattenContract(newC.Root)
	for id, old := range oldItems {
		n, ok := newItems[id]
		if !ok {
			out = append(out, change(ChangeBreaking, id, "removed "+old.kind))
			continue
		}
		out = append(out, diffContractItem(id, old, n)...)
	}
	for id, n := range newItems {
		if _, ok := oldItems[id]; ok {
			continue
		}
		severity := ChangeAdditive
		if n.arg != nil && (n.arg.Required || n.arg.Min > 0) {
			severity = ChangeBreaking
		}
		if n.flag != nil && n.flag.Required {
			severity = ChangeBreaking
		}
		out = append(out, change(severity, id, "added "+n.kind))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		if out[i].Severity != out[j].Severity {
			return severityRank(out[i].Severity) < severityRank(out[j].Severity)
		}
		return out[i].Message < out[j].Message
	})
	return dedupeChanges(out)
}

func diffContractItem(id string, old, n contractItem) []ContractChange {
	var out []ContractChange
	if old.kind != n.kind {
		return []ContractChange{change(ChangeBreaking, id, fmt.Sprintf("changed kind from %s to %s", old.kind, n.kind))}
	}
	if old.parent != n.parent {
		out = append(out, change(ChangeBreaking, id, "moved "+old.kind+" to a different command"))
	}
	if old.name != n.name {
		out = append(out, change(ChangeBreaking, id, fmt.Sprintf("renamed %s from %q to %q", old.kind, old.name, n.name)))
	}
	switch {
	case old.command != nil && n.command != nil:
		out = append(out, diffCommand(id, *old.command, *n.command)...)
	case old.arg != nil && n.arg != nil:
		out = append(out, diffArg(id, old, n)...)
	case old.flag != nil && n.flag != nil:
		out = append(out, diffFlag(id, *old.flag, *n.flag)...)
	}
	return out
}

func diffCommand(id string, old, n SchemaCommand) []ContractChange {
	var out []ContractChange
	out = append(out, aliasChanges(id, old.Aliases, n.Aliases, "command alias")...)
	if !reflect.DeepEqual(old.Constraints, n.Constraints) {
		out = append(out, change(ChangeBreaking, id, "changed command constraints"))
	}
	if !reflect.DeepEqual(old.Requirements, n.Requirements) {
		out = append(out, change(ChangeMetadata, id, "changed runtime requirements; compatibility review required"))
	}
	if !reflect.DeepEqual(old.Capabilities, n.Capabilities) {
		out = append(out, change(ChangeMetadata, id, "changed capability declarations; compatibility review required"))
	}
	if !reflect.DeepEqual(old.Outputs, n.Outputs) {
		out = append(out, change(ChangeMetadata, id, "changed output formats; machine-output compatibility review required"))
	}
	if old.OptionPolicy != n.OptionPolicy {
		out = append(out, change(ChangeBreaking, id, "changed option parsing policy"))
	}
	if old.Hidden != n.Hidden || old.Experimental != n.Experimental {
		out = append(out, change(ChangeMetadata, id, "changed command visibility/stability metadata"))
	}
	if !reflect.DeepEqual(old.Deprecated, n.Deprecated) {
		out = append(out, change(ChangeMetadata, id, "changed command deprecation metadata"))
	}
	return out
}

func diffArg(id string, oldItem, newItem contractItem) []ContractChange {
	old, n := *oldItem.arg, *newItem.arg
	var out []ContractChange
	if old.Type != n.Type {
		out = append(out, change(ChangeBreaking, id, fmt.Sprintf("changed type from %q to %q", old.Type, n.Type)))
	}
	if old.Mode != n.Mode || old.Min != n.Min || old.Max != n.Max || old.Required != n.Required {
		out = append(out, change(ChangeBreaking, id, "changed argument arity/requiredness"))
	}
	if oldItem.position != newItem.position {
		out = append(out, change(ChangeBreaking, id, "changed positional argument order"))
	}
	out = append(out, choiceChanges(id, old.Choices, n.Choices)...)
	if old.Hint != n.Hint || old.Hidden != n.Hidden || old.Sensitive != n.Sensitive {
		out = append(out, change(ChangeMetadata, id, "changed argument completion/visibility metadata"))
	}
	return out
}

func diffFlag(id string, old, n SchemaFlag) []ContractChange {
	var out []ContractChange
	if old.Type != n.Type || old.Action != n.Action {
		out = append(out, change(ChangeBreaking, id, "changed flag type/action"))
	}
	if old.Required != n.Required {
		sev := ChangeAdditive
		if n.Required {
			sev = ChangeBreaking
		}
		out = append(out, change(sev, id, "changed flag requiredness"))
	}
	if old.Global != n.Global {
		out = append(out, change(ChangeBreaking, id, "changed flag scope"))
	}
	out = append(out, aliasChanges(id, old.Aliases, n.Aliases, "flag alias")...)
	if old.Short != n.Short {
		if old.Short != "" {
			out = append(out, change(ChangeBreaking, id, fmt.Sprintf("removed/changed short flag -%s", old.Short)))
		} else if n.Short != "" {
			out = append(out, change(ChangeAdditive, id, fmt.Sprintf("added short flag -%s", n.Short)))
		}
	}
	out = append(out, choiceChanges(id, old.Choices, n.Choices)...)
	if !reflect.DeepEqual(old.Defaults, n.Defaults) {
		out = append(out, change(ChangeMetadata, id, "changed default value; semantic impact review required"))
	}
	if !reflect.DeepEqual(old.Sources, n.Sources) {
		out = append(out, change(ChangeMetadata, id, "changed value resolution sources/precedence; semantic impact review required"))
	}
	if old.Hint != n.Hint || old.Hidden != n.Hidden || old.Sensitive != n.Sensitive {
		out = append(out, change(ChangeMetadata, id, "changed flag completion/visibility metadata"))
	}
	if !reflect.DeepEqual(old.Deprecated, n.Deprecated) {
		out = append(out, change(ChangeMetadata, id, "changed flag deprecation metadata"))
	}
	return out
}

func aliasChanges(id string, old, n []string, label string) []ContractChange {
	oldSet, newSet := stringSet(old), stringSet(n)
	var out []ContractChange
	for x := range oldSet {
		if _, ok := newSet[x]; !ok {
			out = append(out, change(ChangeBreaking, id, fmt.Sprintf("removed %s %q", label, x)))
		}
	}
	for x := range newSet {
		if _, ok := oldSet[x]; !ok {
			out = append(out, change(ChangeAdditive, id, fmt.Sprintf("added %s %q", label, x)))
		}
	}
	return out
}

func choiceChanges(id string, old, n []Choice) []ContractChange {
	oldSet, newSet := map[string]struct{}{}, map[string]struct{}{}
	for _, x := range old {
		oldSet[x.Value] = struct{}{}
	}
	for _, x := range n {
		newSet[x.Value] = struct{}{}
	}
	var out []ContractChange
	for x := range oldSet {
		if _, ok := newSet[x]; !ok {
			out = append(out, change(ChangeBreaking, id, fmt.Sprintf("removed accepted choice %q", x)))
		}
	}
	for x := range newSet {
		if _, ok := oldSet[x]; !ok {
			out = append(out, change(ChangeAdditive, id, fmt.Sprintf("added accepted choice %q", x)))
		}
	}
	return out
}

func stringSet(xs []string) map[string]struct{} {
	m := make(map[string]struct{}, len(xs))
	for _, x := range xs {
		m[x] = struct{}{}
	}
	return m
}
func change(s ChangeSeverity, id, msg string) ContractChange {
	return ContractChange{Severity: s, ID: id, Message: msg}
}
func severityRank(s ChangeSeverity) int {
	switch s {
	case ChangeBreaking:
		return 0
	case ChangeAdditive:
		return 1
	default:
		return 2
	}
}
func dedupeChanges(in []ContractChange) []ContractChange {
	out := make([]ContractChange, 0, len(in))
	last := ""
	for _, c := range in {
		key := string(c.Severity) + "\x00" + c.ID + "\x00" + c.Message
		if key == last {
			continue
		}
		last = key
		out = append(out, c)
	}
	return out
}

type contractItem struct {
	kind, name, parent string
	position           int
	command            *SchemaCommand
	arg                *SchemaArg
	flag               *SchemaFlag
}

func flattenContract(root SchemaCommand) map[string]contractItem {
	out := map[string]contractItem{}
	var walk func(SchemaCommand, string)
	walk = func(c SchemaCommand, parent string) {
		cc := c
		out[c.ID] = contractItem{kind: "command", name: c.Name, parent: parent, command: &cc}
		for i := range c.Args {
			a := c.Args[i]
			out[a.ID] = contractItem{kind: "argument", name: a.Name, parent: c.ID, position: i, arg: &a}
		}
		for i := range c.Flags {
			f := c.Flags[i]
			out[f.ID] = contractItem{kind: "flag", name: f.Long, parent: c.ID, position: i, flag: &f}
		}
		for _, x := range c.Commands {
			walk(x, c.ID)
		}
	}
	walk(root, "")
	return out
}
