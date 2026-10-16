package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// ConfigSchema is the default config file version tag, the value of its
// optional "$schema" key, for a root Command that sets no ConfigSchema of its
// own. A file naming any other tag is refused; a file without the key is read
// as the current version.
const ConfigSchema = "gorge-cli-config-1"

// SchemaKey is the config key holding ConfigSchema.
const SchemaKey = "$schema"

// configFile is a loaded, checked config: each section's keys by its dotted
// command path below the root ("" for the root, "play", "a.b").
type configFile struct {
	path     string
	sections map[string][]member
}

// member is one key of a JSON object, in file order.
type member struct {
	key string
	raw json.RawMessage
}

// objectMembers decodes a JSON object's members in order, refusing a
// duplicate key and anything that is not an object.
func objectMembers(raw []byte) ([]member, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, fmt.Errorf("want a JSON object")
	}
	var out []member
	seen := map[string]bool{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, _ := tok.(string)
		if seen[key] {
			return nil, fmt.Errorf("duplicate key %q", key)
		}
		seen[key] = true
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, fmt.Errorf("key %q: %v", key, err)
		}
		out = append(out, member{key, v})
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("data after the top-level object")
	}
	return out, nil
}

// loadConfig reads and checks a config file against the whole command tree.
func (r *runner) loadConfig(path string) (*configFile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: %v", err)
	}
	ms, err := objectMembers(raw)
	if err != nil {
		return nil, fmt.Errorf("config %s: %v", path, err)
	}
	cf := &configFile{path: path, sections: map[string][]member{}}
	if err := r.checkSection(cf, []*Command{r.root}, ms); err != nil {
		return nil, fmt.Errorf("config %s: %v", path, err)
	}
	return cf, nil
}

func sectionKey(path []*Command) string { return strings.Join(pathWords(path)[1:], ".") }

func dotted(path []*Command, key string) string {
	if s := sectionKey(path); s != "" {
		return s + "." + key
	}
	return key
}

// checkSection checks one section's members: the schema tag (root only),
// subcommand sections (recursively) and this command's own flags, each value
// set on a throwaway flag set to check its type.
func (r *runner) checkSection(cf *configFile, path []*Command, ms []member) error {
	level := len(path) - 1
	cmd := path[level]
	fs := r.flagSet(path)
	var own []member
	for _, m := range ms {
		key := dotted(path, m.key)
		if m.key == SchemaKey {
			if level != 0 {
				return fmt.Errorf("key %q: %s belongs at the top level only", key, SchemaKey)
			}
			var tag string
			if want := schemaOf(r.root); json.Unmarshal(m.raw, &tag) != nil || tag != want {
				return fmt.Errorf("key %q: %s, this build reads %q", key, strings.TrimSpace(string(m.raw)), want)
			}
			continue
		}
		if sub := cmd.subNamed(m.key); sub != nil {
			subMs, err := objectMembers(m.raw)
			if err != nil {
				return fmt.Errorf("key %q (the %s command's section): %v", key, m.key, err)
			}
			if err := r.checkSection(cf, append(append([]*Command(nil), path...), sub), subMs); err != nil {
				return err
			}
			// The throwaway sets above rebound shared variables: rebuild ours.
			fs = r.flagSet(path)
			continue
		}
		f := fs.byName[m.key]
		switch {
		case f == nil || f.global:
			names := []string{}
			for _, g := range fs.order {
				if !g.global && g.level == level {
					names = append(names, g.Name)
				}
			}
			for _, s := range cmd.Sub {
				names = append(names, s.Name)
			}
			msg := fmt.Sprintf("unknown key %q", key)
			if s := suggest(m.key, names); s != "" {
				msg += fmt.Sprintf(" (did you mean %q?)", dotted(path, s))
			}
			return errors.New(msg)
		case f.level != level:
			return fmt.Errorf("key %q: --%s is a flag of %q; set it in that command's section", key, f.Name, pathString(path[:f.level+1]))
		}
		if err := setJSON(f, m.raw); err != nil {
			return fmt.Errorf("key %q: %v", key, err)
		}
		own = append(own, m)
	}
	cf.sections[sectionKey(path)] = own
	return nil
}

// apply sets the flags of every section on path from the config.
func (cf *configFile) apply(fs *FlagSet, path []*Command, sources map[string]Source) error {
	for i := range path {
		for _, m := range cf.sections[sectionKey(path[:i+1])] {
			f := fs.byName[m.key]
			if err := setJSON(f, m.raw); err != nil {
				return fmt.Errorf("config %s: key %q: %v", cf.path, dotted(path[:i+1], m.key), err)
			}
			sources[m.key] = FromConfig
		}
	}
	return nil
}

// setJSON sets a flag from its config value.
func setJSON(f *Flag, raw json.RawMessage) error {
	if jv, ok := f.Value.(JSONValue); ok {
		return jv.SetJSON(raw)
	}
	x, err := decodeJSON(raw)
	if err != nil {
		return err
	}
	s, ok := x.(string)
	if !ok {
		return fmt.Errorf("want a string (the flag's %s text form), got %s", f.Value.Type(), jsonKind(x))
	}
	return f.Value.Set(s)
}

// jsonOf is a flag's --print-config value.
func jsonOf(f *Flag) any {
	if jv, ok := f.Value.(JSONValue); ok {
		return jv.JSON()
	}
	return f.Value.String()
}

// configOf is the effective configuration of the flags in fs for the command
// at path, as --print-config writes it.
func configOf(fs *FlagSet, path []*Command) map[string]any {
	top := map[string]any{SchemaKey: schemaOf(path[0])}
	sections := []map[string]any{top}
	for i := 1; i < len(path); i++ {
		m := map[string]any{}
		sections[i-1][path[i].Name] = m
		sections = append(sections, m)
	}
	for _, f := range fs.order {
		if !f.global {
			sections[f.level][f.Name] = jsonOf(f)
		}
	}
	return top
}

// printConfig writes configOf as indented JSON (encoding/json sorts the keys).
func printConfig(w io.Writer, fs *FlagSet, path []*Command) error {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(configOf(fs, path)); err != nil {
		return err
	}
	_, err := w.Write(b.Bytes())
	return err
}
