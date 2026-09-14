package cli

import "context"

const ReservedNamespace = "__cli"

type ProductMetadata struct {
	Version      string
	Stability    string
	SuiteVersion string
}

type App struct {
	ID           string
	Name         string
	Summary      string
	Description  string
	Product      ProductMetadata
	Root         Command
	Modules      []Module
	Builtins     Builtins
	Middleware   []Middleware
	Capabilities []Capability
	Interaction  Interaction
	DoctorChecks []DoctorCheck
}

type Builtins struct {
	Help       bool
	Version    bool
	Completion bool
	Schema     bool
	Doctor     bool
}

type OptionPolicy string

const (
	// OptionsInterspersed keeps parsing options after positional arguments.
	OptionsInterspersed OptionPolicy = "interspersed"
	// OptionsBeforeArgs stops option parsing when the first positional begins.
	OptionsBeforeArgs OptionPolicy = "before_args"
)

type Command struct {
	ID           string
	Name         string
	Aliases      []string
	Summary      string
	Description  string
	Examples     []string
	Args         []Arg
	Flags        []Flag
	Commands     []Command
	Constraints  []Constraint
	Requirements []Requirement
	Capabilities []CapabilityID
	Outputs      []OutputFormat
	OptionPolicy OptionPolicy
	Handler      Handler
	Hidden       bool
	Experimental bool
	Deprecated   *Deprecation
}

type Deprecation struct {
	Message        string
	ReplacementID  string
	RemovalVersion string
}

type ArgMode uint8

const (
	ArgSingle ArgMode = iota
	ArgVariadic
	ArgOpaque
)

type Arg struct {
	ID        string
	Name      string
	Summary   string
	Value     Value
	Required  bool
	Mode      ArgMode
	Min       int
	Max       int
	Completer Completer
	Hidden    bool
	Sensitive bool
}

type FlagAction uint8

const (
	FlagSet FlagAction = iota
	FlagSwitch
	FlagAppend
	FlagCount
)

type Flag struct {
	ID         string
	Long       string
	Short      rune
	Aliases    []string
	Summary    string
	Value      Value
	Action     FlagAction
	Global     bool
	Required   bool
	Default    []string
	Providers  []ResolutionProvider
	Completer  Completer
	Hidden     bool
	Sensitive  bool
	Deprecated *Deprecation
}

type Module interface{ Apply(*App) error }
type ModuleFunc func(*App) error

func (f ModuleFunc) Apply(a *App) error { return f(a) }

type OutputFormat string

const (
	OutputHuman  OutputFormat = "human"
	OutputJSON   OutputFormat = "json"
	OutputNDJSON OutputFormat = "ndjson"
	OutputRaw    OutputFormat = "raw"
)

type Handler func(*Invocation) error

type Middleware func(Handler) Handler

// Requirement describes a safe preflight check. Check must not mutate state.
type Requirement struct {
	ID      string
	Summary string
	Check   func(context.Context) error
}
