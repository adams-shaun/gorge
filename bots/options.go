package bots

import (
	"fmt"
	"regexp"
	"strconv"
)

var validOptionKey = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// OptionSpec declares one configuration key a bot policy accepts. The host
// reads these declarations to validate -bot-opt, -bot-config and
// GORGED_BOT_<POLICY>_<KEY> values before any seat is built, and to print
// them in the server's help output.
//
// Key must match ^[a-z][a-z0-9-]*$. Type is one of string, int, float, bool
// or path. Default is the value used when neither a flag, the environment
// nor a config file supplies one; it is parsed with the same rules as a
// supplied value. Required makes startup fail when no source supplies the
// key (an empty Default does not satisfy it).
type OptionSpec struct {
	Key      string
	Type     string
	Default  string
	Help     string
	Required bool
}

func validOptionType(t string) bool {
	switch t {
	case "string", "int", "float", "bool", "path":
		return true
	}
	return false
}

// ValidateOptionValue reports whether value parses as spec's declared Type.
// The host's flag/env/file resolution uses it so a supplied value is held to
// exactly the rule Register applied to the spec's default.
func ValidateOptionValue(spec OptionSpec, value string) error {
	switch spec.Type {
	case "string", "path":
		return nil
	case "int":
		if _, err := strconv.Atoi(value); err != nil {
			return fmt.Errorf("want int, got %q", value)
		}
	case "float":
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return fmt.Errorf("want float, got %q", value)
		}
	case "bool":
		if _, err := strconv.ParseBool(value); err != nil {
			return fmt.Errorf("want bool, got %q", value)
		}
	default:
		return fmt.Errorf("unknown type %q", spec.Type)
	}
	return nil
}

func validateOptionSpecs(name string, specs []OptionSpec) {
	seen := make(map[string]bool, len(specs))
	for _, spec := range specs {
		if !validOptionKey.MatchString(spec.Key) {
			panic(fmt.Sprintf("bots: entry %q: invalid option key %q", name, spec.Key))
		}
		if seen[spec.Key] {
			panic(fmt.Sprintf("bots: entry %q: duplicate option key %q", name, spec.Key))
		}
		seen[spec.Key] = true
		if !validOptionType(spec.Type) {
			panic(fmt.Sprintf("bots: entry %q: option %q: invalid type %q", name, spec.Key, spec.Type))
		}
		// A required option may omit its default entirely; "" is then the
		// marker for "not supplied", and New refuses to build the policy.
		if spec.Default != "" || !spec.Required {
			if err := ValidateOptionValue(spec, spec.Default); err != nil {
				panic(fmt.Sprintf("bots: entry %q: option %q: invalid default: %v", name, spec.Key, err))
			}
		}
	}
}

// OptionSpecFor returns the spec declared for key, if any. The host uses it
// to look up a supplied key against the same declarations Register validated.
func OptionSpecFor(specs []OptionSpec, key string) (OptionSpec, bool) {
	for _, spec := range specs {
		if spec.Key == key {
			return spec, true
		}
	}
	return OptionSpec{}, false
}

func (o Options) value(key string) string {
	v, ok := o.Config[key]
	if !ok {
		panic(fmt.Sprintf("bots: option %q is not declared by this policy", key))
	}
	return v
}

// String returns the configured string value of a declared option.
func (o Options) String(key string) string { return o.value(key) }

// Int returns the configured int value of a declared option.
func (o Options) Int(key string) int {
	value := o.value(key)
	v, err := strconv.Atoi(value)
	if err != nil {
		panic(fmt.Sprintf("bots: option %q is not an int: %q", key, value))
	}
	return v
}

// Float returns the configured float value of a declared option.
func (o Options) Float(key string) float64 {
	value := o.value(key)
	v, err := strconv.ParseFloat(value, 64)
	if err != nil {
		panic(fmt.Sprintf("bots: option %q is not a float: %q", key, value))
	}
	return v
}

// Bool returns the configured bool value of a declared option.
func (o Options) Bool(key string) bool {
	value := o.value(key)
	v, err := strconv.ParseBool(value)
	if err != nil {
		panic(fmt.Sprintf("bots: option %q is not a bool: %q", key, value))
	}
	return v
}
