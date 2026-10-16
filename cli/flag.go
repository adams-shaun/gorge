package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Value is a flag's value, set from its text form (a flag argument, an
// environment variable, or a config string). It is pflag's Value.
type Value interface {
	String() string
	Set(string) error
	// Type names the value in usage ("string", "int", "duration", ...).
	Type() string
}

// JSONValue is a Value that reads and writes its config-file form itself.
// Every built-in type implements it. A custom Value that does not is read
// from a JSON string (passed to Set) and printed as its String().
type JSONValue interface {
	Value
	// SetJSON sets the value from the raw JSON of its config key.
	SetJSON(raw json.RawMessage) error
	// JSON is the value --print-config writes (anything encoding/json can
	// marshal; SetJSON must accept it back).
	JSON() any
}

// boolFlag is a Value that needs no argument (--name means true).
type boolFlag interface{ IsBoolFlag() bool }

// resetter is a Value that appends (a slice): each source starts it afresh,
// so a flag replaces the config's list instead of extending it.
type resetter interface{ reset() }

// Flag is one defined flag.
type Flag struct {
	Name     string
	Short    byte // 0: none
	Usage    string
	Value    Value
	DefValue string // the default's text form, for usage
	// Env is the environment variable that sets it ("" for the global flags).
	Env string
	// level is the index in the command path of the command that defined it.
	level  int
	global bool
}

// FlagSet holds a command's flags: its own, those inherited from its
// ancestors (a parent's flags apply to its subcommands), and the global
// --config, --print-config and --help.
type FlagSet struct {
	byName  map[string]*Flag
	byShort map[byte]*Flag
	order   []*Flag
	// building is the command path being defined and its level.
	envPrefix string
	level     int
	global    bool
}

func newFlagSet() *FlagSet {
	return &FlagSet{byName: map[string]*Flag{}, byShort: map[byte]*Flag{}}
}

// Lookup is the named flag, or nil.
func (fs *FlagSet) Lookup(name string) *Flag { return fs.byName[name] }

// validName: lower-case letters, digits and '-', starting with a letter or digit.
func validName(name string) bool {
	if name == "" || name[0] == '-' {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}

// EnvName is the environment variable for a flag of the command at path
// (root first): GORGE_<PATH>_<FLAG>, upper case, '-' as '_'.
func EnvName(path []string, flag string) string {
	parts := append([]string{"GORGE"}, path...)
	parts = append(parts, flag)
	return envUpper(strings.Join(parts, "_"))
}

func envUpper(s string) string {
	return strings.ToUpper(strings.ReplaceAll(s, "-", "_"))
}

// Var defines a flag with a custom Value; its current value is the default.
func (fs *FlagSet) Var(v Value, name, usage string) {
	if !validName(name) {
		panic(fmt.Sprintf("cli: flag name %q: want lower-case letters, digits and '-'", name))
	}
	if _, dup := fs.byName[name]; dup {
		panic(fmt.Sprintf("cli: flag --%s defined twice (or it collides with a global flag)", name))
	}
	f := &Flag{Name: name, Usage: usage, Value: v, DefValue: v.String(), level: fs.level, global: fs.global}
	if !fs.global {
		f.Env = fs.envPrefix + "_" + envUpper(name)
	}
	fs.byName[name] = f
	fs.order = append(fs.order, f)
}

// Shorthand gives the flag name the one-letter alias -c.
func (fs *FlagSet) Shorthand(name string, c byte) {
	f := fs.byName[name]
	if f == nil {
		panic("cli: Shorthand of undefined flag --" + name)
	}
	if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
		panic(fmt.Sprintf("cli: shorthand %q for --%s: want a letter or digit", c, name))
	}
	if g := fs.byShort[c]; g != nil {
		panic(fmt.Sprintf("cli: shorthand -%c of --%s is taken by --%s", c, name, g.Name))
	}
	if g := fs.byName[string(c)]; g != nil && g != f {
		panic(fmt.Sprintf("cli: shorthand -%c of --%s collides with the flag -%s", c, name, g.Name))
	}
	f.Short = c
	fs.byShort[c] = f
}

// StringVar defines a string flag stored in *p.
func (fs *FlagSet) StringVar(p *string, name, def, usage string) {
	*p = def
	fs.Var((*stringValue)(p), name, usage)
}

// String defines a string flag.
func (fs *FlagSet) String(name, def, usage string) *string {
	p := new(string)
	fs.StringVar(p, name, def, usage)
	return p
}

// IntVar defines an int flag stored in *p.
func (fs *FlagSet) IntVar(p *int, name string, def int, usage string) {
	*p = def
	fs.Var((*intValue)(p), name, usage)
}

// Int defines an int flag.
func (fs *FlagSet) Int(name string, def int, usage string) *int {
	p := new(int)
	fs.IntVar(p, name, def, usage)
	return p
}

// Int64Var defines an int64 flag stored in *p.
func (fs *FlagSet) Int64Var(p *int64, name string, def int64, usage string) {
	*p = def
	fs.Var((*int64Value)(p), name, usage)
}

// Int64 defines an int64 flag.
func (fs *FlagSet) Int64(name string, def int64, usage string) *int64 {
	p := new(int64)
	fs.Int64Var(p, name, def, usage)
	return p
}

// Uint64Var defines a uint64 flag stored in *p.
func (fs *FlagSet) Uint64Var(p *uint64, name string, def uint64, usage string) {
	*p = def
	fs.Var((*uint64Value)(p), name, usage)
}

// Uint64 defines a uint64 flag.
func (fs *FlagSet) Uint64(name string, def uint64, usage string) *uint64 {
	p := new(uint64)
	fs.Uint64Var(p, name, def, usage)
	return p
}

// Float64Var defines a float64 flag stored in *p.
func (fs *FlagSet) Float64Var(p *float64, name string, def float64, usage string) {
	*p = def
	fs.Var((*float64Value)(p), name, usage)
}

// Float64 defines a float64 flag.
func (fs *FlagSet) Float64(name string, def float64, usage string) *float64 {
	p := new(float64)
	fs.Float64Var(p, name, def, usage)
	return p
}

// BoolVar defines a bool flag stored in *p.
func (fs *FlagSet) BoolVar(p *bool, name string, def bool, usage string) {
	*p = def
	fs.Var((*boolValue)(p), name, usage)
}

// Bool defines a bool flag.
func (fs *FlagSet) Bool(name string, def bool, usage string) *bool {
	p := new(bool)
	fs.BoolVar(p, name, def, usage)
	return p
}

// StringSliceVar defines a string-list flag stored in *p: repeatable, and
// each argument is split on commas (--x a,b --x c is [a b c]). A config file
// gives it as a JSON array, whose elements are not split.
func (fs *FlagSet) StringSliceVar(p *[]string, name string, def []string, usage string) {
	*p = append([]string(nil), def...)
	fs.Var(&sliceValue{p: p}, name, usage)
}

// StringSlice defines a string-list flag.
func (fs *FlagSet) StringSlice(name string, def []string, usage string) *[]string {
	p := new([]string)
	fs.StringSliceVar(p, name, def, usage)
	return p
}

// decodeJSON decodes raw into v with numbers kept as json.Number.
func decodeJSON(raw json.RawMessage) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

// jsonKind names a decoded JSON value's type for an error.
func jsonKind(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "a bool"
	case json.Number:
		return "a number"
	case string:
		return "a string"
	case []any:
		return "an array"
	case map[string]any:
		return "an object"
	}
	return fmt.Sprintf("%T", v)
}

// wantNumber returns raw's number text or a type error.
func wantNumber(raw json.RawMessage, what string) (string, error) {
	v, err := decodeJSON(raw)
	if err != nil {
		return "", err
	}
	n, ok := v.(json.Number)
	if !ok {
		return "", fmt.Errorf("want %s, got %s", what, jsonKind(v))
	}
	return string(n), nil
}

type stringValue string

func (v *stringValue) Set(s string) error { *v = stringValue(s); return nil }
func (v *stringValue) String() string     { return string(*v) }
func (v *stringValue) Type() string       { return "string" }
func (v *stringValue) JSON() any          { return string(*v) }
func (v *stringValue) SetJSON(raw json.RawMessage) error {
	x, err := decodeJSON(raw)
	if err != nil {
		return err
	}
	s, ok := x.(string)
	if !ok {
		return fmt.Errorf("want a string, got %s", jsonKind(x))
	}
	*v = stringValue(s)
	return nil
}

type intValue int

func (v *intValue) Set(s string) error {
	n, err := strconv.ParseInt(s, 0, strconv.IntSize)
	if err != nil {
		return numErr(err)
	}
	*v = intValue(n)
	return nil
}
func (v *intValue) String() string { return strconv.Itoa(int(*v)) }
func (v *intValue) Type() string   { return "int" }
func (v *intValue) JSON() any      { return int(*v) }
func (v *intValue) SetJSON(raw json.RawMessage) error {
	s, err := wantNumber(raw, "an integer")
	if err != nil {
		return err
	}
	n, err := strconv.ParseInt(s, 10, strconv.IntSize)
	if err != nil {
		return fmt.Errorf("want an integer, got %s", s)
	}
	*v = intValue(n)
	return nil
}

type int64Value int64

func (v *int64Value) Set(s string) error {
	n, err := strconv.ParseInt(s, 0, 64)
	if err != nil {
		return numErr(err)
	}
	*v = int64Value(n)
	return nil
}
func (v *int64Value) String() string { return strconv.FormatInt(int64(*v), 10) }
func (v *int64Value) Type() string   { return "int64" }
func (v *int64Value) JSON() any      { return int64(*v) }
func (v *int64Value) SetJSON(raw json.RawMessage) error {
	s, err := wantNumber(raw, "an integer")
	if err != nil {
		return err
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("want an integer, got %s", s)
	}
	*v = int64Value(n)
	return nil
}

type uint64Value uint64

func (v *uint64Value) Set(s string) error {
	n, err := strconv.ParseUint(s, 0, 64)
	if err != nil {
		return numErr(err)
	}
	*v = uint64Value(n)
	return nil
}
func (v *uint64Value) String() string { return strconv.FormatUint(uint64(*v), 10) }
func (v *uint64Value) Type() string   { return "uint64" }
func (v *uint64Value) JSON() any      { return uint64(*v) }
func (v *uint64Value) SetJSON(raw json.RawMessage) error {
	s, err := wantNumber(raw, "a non-negative integer")
	if err != nil {
		return err
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return fmt.Errorf("want a non-negative integer, got %s", s)
	}
	*v = uint64Value(n)
	return nil
}

type float64Value float64

func (v *float64Value) Set(s string) error {
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return numErr(err)
	}
	*v = float64Value(n)
	return nil
}
func (v *float64Value) String() string { return strconv.FormatFloat(float64(*v), 'g', -1, 64) }
func (v *float64Value) Type() string   { return "float" }
func (v *float64Value) JSON() any      { return float64(*v) }
func (v *float64Value) SetJSON(raw json.RawMessage) error {
	s, err := wantNumber(raw, "a number")
	if err != nil {
		return err
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("want a number, got %s", s)
	}
	*v = float64Value(n)
	return nil
}

type boolValue bool

func (v *boolValue) Set(s string) error {
	b, err := strconv.ParseBool(s)
	if err != nil {
		return fmt.Errorf("want true or false, got %q", s)
	}
	*v = boolValue(b)
	return nil
}
func (v *boolValue) String() string   { return strconv.FormatBool(bool(*v)) }
func (v *boolValue) Type() string     { return "bool" }
func (v *boolValue) IsBoolFlag() bool { return true }
func (v *boolValue) JSON() any        { return bool(*v) }
func (v *boolValue) SetJSON(raw json.RawMessage) error {
	x, err := decodeJSON(raw)
	if err != nil {
		return err
	}
	b, ok := x.(bool)
	if !ok {
		return fmt.Errorf("want a bool, got %s", jsonKind(x))
	}
	*v = boolValue(b)
	return nil
}

type sliceValue struct {
	p *[]string
	// fresh: the next Set replaces the list (a new source started).
	fresh bool
}

func (v *sliceValue) reset() { v.fresh = true }
func (v *sliceValue) Set(s string) error {
	if v.fresh {
		*v.p, v.fresh = nil, false
	}
	*v.p = append(*v.p, strings.Split(s, ",")...)
	return nil
}
func (v *sliceValue) String() string { return strings.Join(*v.p, ",") }
func (v *sliceValue) Type() string   { return "strings" }
func (v *sliceValue) JSON() any {
	if *v.p == nil {
		return []string{}
	}
	return *v.p
}
func (v *sliceValue) SetJSON(raw json.RawMessage) error {
	x, err := decodeJSON(raw)
	if err != nil {
		return err
	}
	arr, ok := x.([]any)
	if !ok {
		return fmt.Errorf("want an array of strings, got %s", jsonKind(x))
	}
	out := make([]string, 0, len(arr))
	for i, e := range arr {
		s, ok := e.(string)
		if !ok {
			return fmt.Errorf("element %d: want a string, got %s", i, jsonKind(e))
		}
		out = append(out, s)
	}
	*v.p, v.fresh = out, false
	return nil
}

// numErr strips strconv's function prefix: `parsing "x": invalid syntax`.
func numErr(err error) error {
	if ne, ok := err.(*strconv.NumError); ok {
		return fmt.Errorf("parsing %q: %v", ne.Num, ne.Err)
	}
	return err
}
