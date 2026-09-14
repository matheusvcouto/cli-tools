package cli

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ValueHint describes shell-neutral value semantics.
type ValueHint string

const (
	HintNone       ValueHint = ""
	HintPath       ValueHint = "path"
	HintFile       ValueHint = "file"
	HintDirectory  ValueHint = "directory"
	HintExecutable ValueHint = "executable"
	HintHostname   ValueHint = "hostname"
	HintUsername   ValueHint = "username"
	HintURL        ValueHint = "url"
	HintEmail      ValueHint = "email"
	HintEnum       ValueHint = "enum"
)

// Choice is a finite static value exposed to help/completion/schema.
type Choice struct {
	Value       string
	Description string
}

// Codec is the typed extension point for command-line values.
type Codec[T any] interface {
	Parse(string) (T, error)
	Format(T) string
	TypeName() string
	Hint() ValueHint
	Choices() []Choice
}

// CodecFuncs makes small custom codecs easy without reflection.
type CodecFuncs[T any] struct {
	ParseFunc     func(string) (T, error)
	FormatFunc    func(T) string
	Name          string
	ValueHint     ValueHint
	StaticChoices []Choice
}

func (c CodecFuncs[T]) Parse(s string) (T, error) {
	if c.ParseFunc == nil {
		var zero T
		return zero, fmt.Errorf("codec %q has no parser", c.Name)
	}
	return c.ParseFunc(s)
}
func (c CodecFuncs[T]) Format(v T) string {
	if c.FormatFunc != nil {
		return c.FormatFunc(v)
	}
	return fmt.Sprint(v)
}
func (c CodecFuncs[T]) TypeName() string  { return c.Name }
func (c CodecFuncs[T]) Hint() ValueHint   { return c.ValueHint }
func (c CodecFuncs[T]) Choices() []Choice { return append([]Choice(nil), c.StaticChoices...) }

// Value is the heterogeneous value descriptor stored by args and flags.
type Value interface {
	parse(string) (any, error)
	format(any) string
	typeName() string
	hint() ValueHint
	choices() []Choice
	sensitive() bool
}

type typedValue[T any] struct {
	codec       Codec[T]
	isSensitive bool
}

func (v typedValue[T]) parse(s string) (any, error) { return v.codec.Parse(s) }
func (v typedValue[T]) format(x any) string {
	t, ok := x.(T)
	if !ok {
		return fmt.Sprint(x)
	}
	return v.codec.Format(t)
}
func (v typedValue[T]) typeName() string  { return v.codec.TypeName() }
func (v typedValue[T]) hint() ValueHint   { return v.codec.Hint() }
func (v typedValue[T]) choices() []Choice { return v.codec.Choices() }
func (v typedValue[T]) sensitive() bool   { return v.isSensitive }

// TypedValue erases a typed Codec for use in an Arg or Flag.
func TypedValue[T any](codec Codec[T]) Value { return typedValue[T]{codec: codec} }

// Sensitive marks a value as redacted and excluded from completion/schema data.
func Sensitive(v Value) Value {
	if v == nil {
		return nil
	}
	return sensitiveValue{Value: v}
}

type sensitiveValue struct{ Value }

func (v sensitiveValue) sensitive() bool { return true }

func StringValue() Value {
	return TypedValue(CodecFuncs[string]{ParseFunc: func(s string) (string, error) { return s, nil }, FormatFunc: func(s string) string { return s }, Name: "string"})
}
func BoolValue() Value {
	return TypedValue(CodecFuncs[bool]{ParseFunc: strconv.ParseBool, FormatFunc: strconv.FormatBool, Name: "bool"})
}
func IntValue() Value {
	return TypedValue(CodecFuncs[int64]{ParseFunc: func(s string) (int64, error) { return strconv.ParseInt(s, 10, 64) }, FormatFunc: func(v int64) string { return strconv.FormatInt(v, 10) }, Name: "int"})
}
func UintValue() Value {
	return TypedValue(CodecFuncs[uint64]{ParseFunc: func(s string) (uint64, error) { return strconv.ParseUint(s, 10, 64) }, FormatFunc: func(v uint64) string { return strconv.FormatUint(v, 10) }, Name: "uint"})
}
func FloatValue() Value {
	return TypedValue(CodecFuncs[float64]{ParseFunc: func(s string) (float64, error) { return strconv.ParseFloat(s, 64) }, FormatFunc: func(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }, Name: "float"})
}
func DurationValue() Value {
	return TypedValue(CodecFuncs[time.Duration]{ParseFunc: time.ParseDuration, FormatFunc: func(v time.Duration) string { return v.String() }, Name: "duration"})
}
func PathValue() Value       { return semanticString("path", HintPath, nil) }
func FileValue() Value       { return semanticString("file", HintFile, nil) }
func DirectoryValue() Value  { return semanticString("directory", HintDirectory, nil) }
func ExecutableValue() Value { return semanticString("executable", HintExecutable, nil) }
func HostnameValue() Value   { return semanticString("hostname", HintHostname, nil) }
func UsernameValue() Value   { return semanticString("username", HintUsername, nil) }
func EmailValue() Value      { return semanticString("email", HintEmail, nil) }
func URLValue() Value {
	return TypedValue(CodecFuncs[string]{ParseFunc: func(s string) (string, error) {
		u, err := url.ParseRequestURI(s)
		if err != nil || u.Scheme == "" {
			return "", fmt.Errorf("invalid URL")
		}
		return s, nil
	}, FormatFunc: func(s string) string { return s }, Name: "url", ValueHint: HintURL})
}
func EnumValue(values ...Choice) Value {
	allowed := make(map[string]struct{}, len(values))
	for _, v := range values {
		allowed[v.Value] = struct{}{}
	}
	return semanticString("enum", HintEnum, valuesWithParser(values, allowed))
}
func valuesWithParser(values []Choice, allowed map[string]struct{}) []Choice {
	return append([]Choice(nil), values...)
}
func semanticString(name string, hint ValueHint, choices []Choice) Value {
	allowed := map[string]struct{}{}
	for _, c := range choices {
		allowed[c.Value] = struct{}{}
	}
	parse := func(s string) (string, error) {
		if len(allowed) > 0 {
			if _, ok := allowed[s]; !ok {
				keys := make([]string, 0, len(choices))
				for _, c := range choices {
					keys = append(keys, c.Value)
				}
				return "", fmt.Errorf("must be one of %s", strings.Join(keys, ", "))
			}
		}
		return s, nil
	}
	return TypedValue(CodecFuncs[string]{ParseFunc: parse, FormatFunc: func(s string) string { return s }, Name: name, ValueHint: hint, StaticChoices: choices})
}
