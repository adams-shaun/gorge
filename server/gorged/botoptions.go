package gorged

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/bots"
)

// botOptionEnvPrefix is the environment variable family for bot options:
// GORGED_BOT_<POLICY>_<KEY>, with '-' turned into '_' in both names.
const botOptionEnvPrefix = "GORGED_BOT_"

func botOptionEnvName(policy, key string) string {
	upper := func(s string) string { return strings.ToUpper(strings.ReplaceAll(s, "-", "_")) }
	return botOptionEnvPrefix + upper(policy) + "_" + upper(key)
}

// resolveBotOptions merges every declared bot option from its default, the
// -bot-config files, the GORGED_BOT_<POLICY>_<KEY> environment and the
// -bot-opt flags, in that order, and validates the result against the
// policy's bots.OptionSpec declarations. getenv is injectable so tests do not
// mutate the process environment.
func resolveBotOptions(optArgs, configArgs []string, getenv func(string) string) (map[string]map[string]string, error) {
	return resolveBotOptionsFor(bots.Entries(), optArgs, configArgs, getenv)
}

func resolveBotOptionsFor(all []bots.Entry, optArgs, configArgs []string, getenv func(string) string) (map[string]map[string]string, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	entries := make(map[string]bots.Entry)
	resolved := make(map[string]map[string]string)
	for _, e := range all {
		entries[e.Name] = e
		if len(e.Options) == 0 {
			continue
		}
		values := make(map[string]string, len(e.Options))
		for _, spec := range e.Options {
			values[spec.Key] = spec.Default
		}
		resolved[e.Name] = values
	}

	for _, arg := range configArgs {
		policy, path, ok := strings.Cut(arg, "=")
		if !ok || policy == "" || path == "" {
			return nil, fmt.Errorf("-bot-config %q: want <policy>=<file.json>", arg)
		}
		e, ok := entries[policy]
		if !ok {
			return nil, fmt.Errorf("-bot-config %q: unknown bot policy %q (known: %s)", arg, policy, strings.Join(entryNames(all), ", "))
		}
		if len(e.Options) == 0 {
			return nil, fmt.Errorf("-bot-config %q: bot policy %q declares no options", arg, policy)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("-bot-config %q: %w", arg, err)
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		var object map[string]any
		if err := dec.Decode(&object); err != nil {
			return nil, fmt.Errorf("-bot-config %q: %s: %w", arg, path, err)
		}
		keys := make([]string, 0, len(object))
		for key := range object {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			spec, ok := bots.OptionSpecFor(e.Options, key)
			if !ok {
				return nil, unknownBotOption("-bot-config", policy, key, e.Options)
			}
			value, err := configValueString(object[key])
			if err != nil {
				return nil, fmt.Errorf("-bot-config %q: option %q: %w", arg, key, err)
			}
			if err := bots.ValidateOptionValue(spec, value); err != nil {
				return nil, fmt.Errorf("-bot-config %q: option %q: %w", arg, key, err)
			}
			resolved[policy][key] = value
		}
	}

	for _, e := range all {
		if len(e.Options) == 0 {
			continue
		}
		for _, spec := range e.Options {
			name := botOptionEnvName(e.Name, spec.Key)
			value := getenv(name)
			if value == "" {
				continue
			}
			if err := bots.ValidateOptionValue(spec, value); err != nil {
				return nil, fmt.Errorf("%s: option %q: %w", name, spec.Key, err)
			}
			resolved[e.Name][spec.Key] = value
		}
	}

	for _, arg := range optArgs {
		policy, key, value, err := parseBotOptArg(arg)
		if err != nil {
			return nil, err
		}
		e, ok := entries[policy]
		if !ok {
			return nil, fmt.Errorf("-bot-opt %q: unknown bot policy %q (known: %s)", arg, policy, strings.Join(entryNames(all), ", "))
		}
		if len(e.Options) == 0 {
			return nil, fmt.Errorf("-bot-opt %q: bot policy %q declares no options", arg, policy)
		}
		spec, ok := bots.OptionSpecFor(e.Options, key)
		if !ok {
			return nil, unknownBotOption("-bot-opt", policy, key, e.Options)
		}
		if err := bots.ValidateOptionValue(spec, value); err != nil {
			return nil, fmt.Errorf("-bot-opt %q: option %q: %w", arg, key, err)
		}
		resolved[policy][key] = value
	}

	// Required options are checked across every registered policy, not just
	// the offered set: this runs before the offered set is resolved (network
	// policies register during serve), and a policy a table names must already
	// be buildable. A linked policy that declares Required must therefore be
	// configurable on every server that links it.
	for _, e := range all {
		for _, spec := range e.Options {
			if !spec.Required || resolved[e.Name][spec.Key] != "" {
				continue
			}
			return nil, fmt.Errorf("bot policy %q requires option %q (set -bot-opt %s.%s=<value> or %s)",
				e.Name, spec.Key, e.Name, spec.Key, botOptionEnvName(e.Name, spec.Key))
		}
	}
	return resolved, nil
}

func parseBotOptArg(arg string) (policy, key, value string, err error) {
	left, value, ok := strings.Cut(arg, "=")
	if !ok {
		return "", "", "", fmt.Errorf("-bot-opt %q: want <policy>.<key>=<value>", arg)
	}
	policy, key, ok = strings.Cut(left, ".")
	if !ok || policy == "" || key == "" {
		return "", "", "", fmt.Errorf("-bot-opt %q: want <policy>.<key>=<value>", arg)
	}
	return policy, key, value, nil
}

func unknownBotOption(flagName, policy, key string, specs []bots.OptionSpec) error {
	keys := make([]string, 0, len(specs))
	for _, spec := range specs {
		keys = append(keys, spec.Key)
	}
	sort.Strings(keys)
	return fmt.Errorf("%s: bot policy %q has no option %q (known: %s)", flagName, policy, key, strings.Join(keys, ", "))
}

func configValueString(v any) (string, error) {
	switch value := v.(type) {
	case string:
		return value, nil
	case bool:
		return strconv.FormatBool(value), nil
	case json.Number:
		return value.String(), nil
	case nil:
		return "", nil
	default:
		return "", fmt.Errorf("want a string, number or bool, got %T", v)
	}
}

func entryNames(entries []bots.Entry) []string {
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name)
	}
	sort.Strings(names)
	return names
}

// printBotOptionHelp lists every registered policy that declares options,
// with each key's type, default, required marker and help text. It is called
// from the flag set's Usage so `gorged -help` is the discovery surface for a
// linked policy's options.
func printBotOptionHelp(w io.Writer) {
	entries := bots.Entries()
	printed := false
	for _, e := range entries {
		if len(e.Options) == 0 {
			continue
		}
		if !printed {
			fmt.Fprintln(w, "\nBot policy options (set with -bot-opt <policy>.<key>=<value>, -bot-config <policy>=<file.json>, or GORGED_BOT_<POLICY>_<KEY>):")
			printed = true
		}
		fmt.Fprintf(w, "  %s (%s):\n", e.Name, e.Label)
		for _, spec := range e.Options {
			defaultText := fmt.Sprintf("default %q", spec.Default)
			if spec.Default == "" {
				defaultText = `default ""`
			}
			required := ""
			if spec.Required {
				required = ", required"
			}
			fmt.Fprintf(w, "    %s.%s (%s; %s%s)  %s\n", e.Name, spec.Key, spec.Type, defaultText, required, spec.Help)
		}
	}
}
