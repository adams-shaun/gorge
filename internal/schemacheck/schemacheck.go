// Package schemacheck validates JSON documents against a directory of JSON
// Schema files that refer to each other by relative name. It implements the
// keywords gorge's published SpellBench v2 schemas use, listed in keywords, and
// Load refuses a schema using any other, so a keyword it can't evaluate is never
// silently passed. The standard library has no JSON Schema support and gorge
// takes no third-party modules.
package schemacheck

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// keywords are what Load accepts. The annotations and $defs validate nothing.
var keywords = map[string]bool{
	"$ref": true, "type": true, "const": true, "enum": true,
	"anyOf": true, "oneOf": true, "allOf": true,
	"properties": true, "patternProperties": true, "additionalProperties": true, "required": true,
	"items": true, "prefixItems": true, "minItems": true, "maxItems": true, "uniqueItems": true,
	"minLength": true, "pattern": true, "minimum": true, "maximum": true,
	"$schema": true, "$id": true, "$defs": true, "title": true, "description": true,
}

// Set is a directory of schema files.
type Set struct {
	docs     map[string]any
	patterns map[string]*regexp.Regexp
}

// Load reads every *.schema.json in dir.
func Load(dir string) (*Set, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.schema.json"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("schemacheck: no schemas in %s", dir)
	}
	s := &Set{docs: map[string]any{}, patterns: map[string]*regexp.Regexp{}}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		doc, err := Decode(raw)
		if err != nil {
			return nil, fmt.Errorf("schemacheck: %s: %w", f, err)
		}
		if err := s.compile(doc, filepath.Base(f)); err != nil {
			return nil, err
		}
		s.docs[filepath.Base(f)] = doc
	}
	return s, nil
}

// compile holds a schema to keywords and compiles its patterns.
func (s *Set) compile(schema any, at string) error {
	m, ok := schema.(map[string]any)
	if !ok {
		if _, isBool := schema.(bool); isBool {
			return nil
		}
		return fmt.Errorf("schemacheck: %s: a schema must be an object or a boolean", at)
	}
	for key, v := range m {
		if !keywords[key] {
			return fmt.Errorf("schemacheck: %s: keyword %q is not implemented", at, key)
		}
		var subs []any
		switch key {
		case "pattern":
			if err := s.addPattern(v.(string)); err != nil {
				return err
			}
		case "properties", "$defs":
			for _, sub := range v.(map[string]any) {
				subs = append(subs, sub)
			}
		case "patternProperties":
			for p, sub := range v.(map[string]any) {
				if err := s.addPattern(p); err != nil {
					return err
				}
				subs = append(subs, sub)
			}
		case "anyOf", "oneOf", "allOf", "prefixItems":
			subs = v.([]any)
		case "items", "additionalProperties":
			subs = []any{v}
		}
		for _, sub := range subs {
			if err := s.compile(sub, at+"/"+key); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Set) addPattern(p string) error {
	re, err := regexp.Compile(p)
	if err != nil {
		return fmt.Errorf("schemacheck: pattern %q: %w", p, err)
	}
	s.patterns[p] = re
	return nil
}

// Decode reads one JSON value, keeping numbers exact.
func Decode(raw []byte) (any, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

// Validate checks doc, a value from Decode, against the schema ref names: a file,
// with an optional JSON pointer ("responses.schema.json#/$defs/decision"). The
// error names the path of the deepest failure.
func (s *Set) Validate(ref string, doc any) error {
	schema, file, err := s.resolve(ref, "")
	if err != nil {
		return err
	}
	if f := s.check(schema, file, doc, "$"); f != nil {
		return f
	}
	return nil
}

func (s *Set) resolve(ref, from string) (any, string, error) {
	file, pointer, _ := strings.Cut(ref, "#")
	if file == "" {
		file = from
	}
	v, ok := s.docs[file]
	if !ok {
		return nil, "", fmt.Errorf("schemacheck: no schema file %q", file)
	}
	for _, part := range strings.Split(strings.TrimPrefix(pointer, "/"), "/") {
		if part == "" {
			continue
		}
		m, ok := v.(map[string]any)
		if !ok {
			return nil, "", fmt.Errorf("schemacheck: bad reference %q", ref)
		}
		if v, ok = m[part]; !ok {
			return nil, "", fmt.Errorf("schemacheck: bad reference %q", ref)
		}
	}
	return v, file, nil
}

// failure is a validation error at a path.
type failure struct {
	path, msg string
}

func (f *failure) Error() string { return f.path + ": " + f.msg }

func failf(path, format string, args ...any) *failure {
	return &failure{path, fmt.Sprintf(format, args...)}
}

// deeper is the more useful of two branch failures: the one that got further.
func deeper(a, b *failure) *failure {
	if a == nil || (b != nil && len(b.path) > len(a.path)) {
		return b
	}
	return a
}

func (s *Set) check(schema any, file string, v any, path string) *failure {
	m, ok := schema.(map[string]any)
	if !ok {
		if schema == false {
			return failf(path, "not allowed")
		}
		return nil
	}
	if ref, ok := m["$ref"].(string); ok {
		target, targetFile, err := s.resolve(ref, file)
		if err != nil {
			return failf(path, "%v", err)
		}
		if f := s.check(target, targetFile, v, path); f != nil {
			return f
		}
	}
	if t, ok := m["type"]; ok {
		if f := checkType(t, v, path); f != nil {
			return f
		}
	}
	if c, ok := m["const"]; ok && !reflect.DeepEqual(c, v) {
		return failf(path, "%s, want %s", show(v), show(c))
	}
	if e, ok := m["enum"].([]any); ok {
		found := false
		for _, want := range e {
			found = found || reflect.DeepEqual(want, v)
		}
		if !found {
			return failf(path, "%s is not one of %s", show(v), show(e))
		}
	}
	if subs, ok := m["allOf"].([]any); ok {
		for _, sub := range subs {
			if f := s.check(sub, file, v, path); f != nil {
				return f
			}
		}
	}
	if subs, ok := m["anyOf"].([]any); ok {
		if _, f := s.count(subs, file, v, path); f != nil {
			return f
		}
	}
	if subs, ok := m["oneOf"].([]any); ok {
		n, f := s.count(subs, file, v, path)
		if f != nil {
			return f
		}
		if n != 1 {
			return failf(path, "matches %d branches of a oneOf", n)
		}
	}
	switch x := v.(type) {
	case map[string]any:
		return s.checkObject(m, file, x, path)
	case []any:
		return s.checkArray(m, file, x, path)
	case string:
		if n, ok := m["minLength"]; ok && int64(len([]rune(x))) < integer(n) {
			return failf(path, "%q is shorter than %v", x, n)
		}
		if p, ok := m["pattern"].(string); ok && !s.patterns[p].MatchString(x) {
			return failf(path, "%q does not match %s", x, p)
		}
	case json.Number:
		n, ok := new(big.Int).SetString(x.String(), 10)
		if !ok {
			return nil // not an integer; the type keyword rules on that
		}
		if b, ok := m["minimum"]; ok && n.Cmp(big.NewInt(integer(b))) < 0 {
			return failf(path, "%s is below %v", x, b)
		}
		if b, ok := m["maximum"]; ok && n.Cmp(big.NewInt(integer(b))) > 0 {
			return failf(path, "%s is above %v", x, b)
		}
	}
	return nil
}

// count evaluates a list of branches. With none matching, it returns the failure
// of the branch that got furthest.
func (s *Set) count(subs []any, file string, v any, path string) (int, *failure) {
	n := 0
	var best *failure
	for _, sub := range subs {
		f := s.check(sub, file, v, path)
		if f == nil {
			n++
		}
		best = deeper(best, f)
	}
	if n > 0 {
		return n, nil
	}
	return 0, best
}

func (s *Set) checkObject(m map[string]any, file string, x map[string]any, path string) *failure {
	if req, ok := m["required"].([]any); ok {
		for _, name := range req {
			if _, ok := x[name.(string)]; !ok {
				return failf(path, "missing %v", name)
			}
		}
	}
	props, _ := m["properties"].(map[string]any)
	patterns, _ := m["patternProperties"].(map[string]any)
	additional, hasAdditional := m["additionalProperties"]
	keys := make([]string, 0, len(x))
	for k := range x {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		at := path + "." + k
		matched := false
		if sub, ok := props[k]; ok {
			matched = true
			if f := s.check(sub, file, x[k], at); f != nil {
				return f
			}
		}
		for p, sub := range patterns {
			if s.patterns[p].MatchString(k) {
				matched = true
				if f := s.check(sub, file, x[k], at); f != nil {
					return f
				}
			}
		}
		if !matched && hasAdditional {
			if additional == false {
				return failf(at, "unexpected field")
			}
			if f := s.check(additional, file, x[k], at); f != nil {
				return f
			}
		}
	}
	return nil
}

func (s *Set) checkArray(m map[string]any, file string, x []any, path string) *failure {
	if n, ok := m["minItems"]; ok && int64(len(x)) < integer(n) {
		return failf(path, "%d items, want at least %v", len(x), n)
	}
	if n, ok := m["maxItems"]; ok && int64(len(x)) > integer(n) {
		return failf(path, "%d items, want at most %v", len(x), n)
	}
	prefix, _ := m["prefixItems"].([]any)
	for i, item := range x {
		at := path + "[" + strconv.Itoa(i) + "]"
		sub, ok := any(nil), false
		if i < len(prefix) {
			sub, ok = prefix[i], true
		} else {
			sub, ok = m["items"]
		}
		if ok {
			if f := s.check(sub, file, item, at); f != nil {
				return f
			}
		}
	}
	if m["uniqueItems"] == true {
		seen := map[string]bool{}
		for i, item := range x {
			b, err := json.Marshal(item) // sorts object keys, so equal values are equal bytes
			if err != nil {
				return failf(path, "%v", err)
			}
			if seen[string(b)] {
				return failf(path+"["+strconv.Itoa(i)+"]", "repeats an earlier item")
			}
			seen[string(b)] = true
		}
	}
	return nil
}

func checkType(t, v any, path string) *failure {
	names, ok := t.([]any)
	if !ok {
		names = []any{t}
	}
	for _, name := range names {
		if isType(name.(string), v) {
			return nil
		}
	}
	return failf(path, "%s is not of type %v", show(v), t)
}

func isType(name string, v any) bool {
	switch x := v.(type) {
	case nil:
		return name == "null"
	case bool:
		return name == "boolean"
	case string:
		return name == "string"
	case []any:
		return name == "array"
	case map[string]any:
		return name == "object"
	case json.Number:
		return name == "number" || (name == "integer" && !strings.ContainsAny(x.String(), ".eE"))
	}
	return false
}

func integer(v any) int64 {
	n, _ := v.(json.Number).Int64()
	return n
}

// show renders a value in an error, cut to a readable length.
func show(v any) string {
	b, _ := json.Marshal(v)
	if len(b) > 120 {
		return string(b[:120]) + "..."
	}
	return string(b)
}
