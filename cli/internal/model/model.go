package model

import "context"

type Value interface {
	Parse(string) (any, error)
	TypeName() string
	Hint() string
	Choices() []Choice
	Sensitive() bool
}
type Choice struct{ Value, Description string }

type Flag struct {
	ID, Long, Summary                   string
	Short                               rune
	Aliases                             []string
	Value                               Value
	Action                              uint8
	Global, Required, Hidden, Sensitive bool
	Default                             []string
	Providers                           []Provider
	Deprecated                          *Deprecation
}

type Provider struct {
	Source, Name string
	Resolve      func(context.Context) ([]string, bool, error)
}

type Arg struct {
	ID, Name, Summary string
	Value             Value
	Required          bool
	Mode              uint8
	Min, Max          int
	Hidden, Sensitive bool
}

type Constraint struct {
	Kind        string
	IDs         []string
	Message     string
	Min, Max    int
	PredicateID string
	Validate    func(map[string][]any) error
}
type Requirement struct {
	ID, Summary string
	Check       func(context.Context) error
}

type Deprecation struct {
	Message        string
	ReplacementID  string
	RemovalVersion string
}

type Command struct {
	ID, Name, Summary, Description string
	Aliases                        []string
	Examples                       []string
	Args                           []*Arg
	Flags                          []*Flag
	Constraints                    []Constraint
	Requirements                   []Requirement
	Capabilities                   []string
	Outputs                        []string
	OptionPolicy                   string
	Deprecated                     *Deprecation
	Children                       []*Command
	ChildByName                    map[string]*Command
	FlagByLong                     map[string]*Flag
	FlagByShort                    map[rune]*Flag
	Parent                         *Command
	Hidden, Experimental           bool
	Handler                        any
}

type Graph struct {
	AppID, Name, Summary, Description string
	ProductVersion                    string
	Stability                         string
	SuiteVersion                      string
	Root                              *Command
}
