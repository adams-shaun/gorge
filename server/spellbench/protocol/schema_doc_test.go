package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The docs site publishes the candidate kinds and vocabularies as JSON Schema
// (docs/site/api/schemas/spellbench-v2/semantic.schema.json) and the error
// codes in responses.schema.json. These tests hold the published schemas to
// kindTable, Vocab and errors.go in both directions, so a kind, field, type or
// word added here (or there) without the other fails the build.

const schemaDir = "docs/site/api/schemas/spellbench-v2"

// moduleRoot walks up from the test's working directory to go.mod.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test directory")
		}
		dir = parent
	}
}

func loadSchema(t *testing.T, name string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(moduleRoot(t), schemaDir, name))
	if err != nil {
		t.Fatal(err)
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var doc map[string]any
	if err := d.Decode(&doc); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return doc
}

// schemaRefs maps the $refs the semantic schema uses for kindTable's
// structural types back to their kindTable names.
var schemaRefs = map[string]string{
	"common.schema.json#/$defs/ObjectRef": "R",
	"common.schema.json#/$defs/TargetRef": "T",
	"common.schema.json#/$defs/OrderItem": "item",
	"common.schema.json#/$defs/u32":       "u32",
	"common.schema.json#/$defs/i32":       "i32",
	"common.schema.json#/$defs/seat":      "seat",
	"common.schema.json#/$defs/snake":     "snake",
}

// schemaType renders a field's schema as a kindTable type string: R, T, item,
// u32, i32, seat, snake, bool, string, a Vocab name, [X], and a |null suffix.
func schemaType(node any) (string, error) {
	n, ok := node.(map[string]any)
	if !ok {
		return "", fmt.Errorf("%v is not a schema object", node)
	}
	if alts, ok := n["anyOf"].([]any); ok && len(n) == 1 && len(alts) == 2 {
		for i, alt := range alts {
			if a, ok := alt.(map[string]any); ok && len(a) == 1 && a["type"] == "null" {
				inner, err := schemaType(alts[1-i])
				if err != nil || strings.HasSuffix(inner, "|null") {
					return "", fmt.Errorf("nullable %v: %v", alts[1-i], err)
				}
				return inner + "|null", nil
			}
		}
	}
	if r, ok := n["$ref"].(string); ok && len(n) == 1 {
		if name, ok := schemaRefs[r]; ok {
			return name, nil
		}
		if name, ok := strings.CutPrefix(r, "#/$defs/vocab_"); ok {
			return name, nil
		}
		return "", fmt.Errorf("unmapped $ref %q", r)
	}
	switch n["type"] {
	case "array":
		if len(n) != 2 {
			return "", fmt.Errorf("array schema %v carries more than items", n)
		}
		inner, err := schemaType(n["items"])
		if err != nil {
			return "", err
		}
		return "[" + inner + "]", nil
	case "boolean":
		if len(n) == 1 {
			return "bool", nil
		}
	case "string":
		if len(n) == 1 {
			return "string", nil
		}
	}
	return "", fmt.Errorf("unmapped schema %v", n)
}

func strs(v any) ([]string, bool) {
	a, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, len(a))
	for i, e := range a {
		if out[i], ok = e.(string); !ok {
			return nil, false
		}
	}
	return out, true
}

func TestSemanticSchemaMatchesKindTable(t *testing.T) {
	doc := loadSchema(t, "semantic.schema.json")
	defs, ok := doc["$defs"].(map[string]any)
	if !ok {
		t.Fatal("semantic.schema.json has no $defs")
	}
	kinds := slices.Sorted(maps.Keys(kindTable))

	for _, kind := range kinds {
		slots := kindTable[kind]
		def, ok := defs["kind_"+kind].(map[string]any)
		if !ok {
			t.Errorf("kind %s: no $defs/kind_%s in the schema", kind, kind)
			continue
		}
		if def["type"] != "object" || def["additionalProperties"] != false {
			t.Errorf("kind %s: want type object and additionalProperties false", kind)
		}
		props, _ := def["properties"].(map[string]any)
		if c, _ := props["kind"].(map[string]any); c == nil || c["const"] != kind || len(c) != 1 {
			t.Errorf("kind %s: properties.kind is not {\"const\": %q}", kind, kind)
		}
		want := []string{"kind"}
		for _, s := range slots {
			want = append(want, s.name)
		}
		if req, _ := strs(def["required"]); !slices.Equal(req, want) {
			t.Errorf("kind %s: required %v, kindTable has %v", kind, req, want)
		}
		if got := slices.Sorted(maps.Keys(props)); !slices.Equal(got, slices.Sorted(slices.Values(want))) {
			t.Errorf("kind %s: properties %v, kindTable has %v", kind, got, want)
		}
		for _, s := range slots {
			if got, err := schemaType(props[s.name]); err != nil || got != s.typ {
				t.Errorf("kind %s field %s: schema type %q (%v), kindTable %q", kind, s.name, got, err, s.typ)
			}
		}
	}
	for name := range defs {
		if k, ok := strings.CutPrefix(name, "kind_"); ok && name != "kind_name" {
			if _, ok := kindTable[k]; !ok {
				t.Errorf("schema defines kind %s, which kindTable does not have", k)
			}
		}
	}

	var oneOf []string
	alts, _ := doc["oneOf"].([]any)
	for _, a := range alts {
		r, _ := a.(map[string]any)["$ref"].(string)
		oneOf = append(oneOf, strings.TrimPrefix(r, "#/$defs/kind_"))
	}
	sort.Strings(oneOf)
	if !slices.Equal(oneOf, kinds) {
		t.Errorf("top-level oneOf lists %v, kindTable has %v", oneOf, kinds)
	}
	names, _ := strs(defs["kind_name"].(map[string]any)["enum"])
	sort.Strings(names)
	if !slices.Equal(names, kinds) {
		t.Errorf("$defs/kind_name enum %v, kindTable has %v", names, kinds)
	}

	for name, words := range Vocab {
		def, ok := defs["vocab_"+name].(map[string]any)
		if !ok {
			t.Errorf("vocabulary %s: no $defs/vocab_%s in the schema", name, name)
			continue
		}
		if got, _ := strs(def["enum"]); !slices.Equal(got, words) || len(def) != 1 {
			t.Errorf("vocabulary %s: schema enum %v, Vocab has %v", name, got, words)
		}
	}
	for name := range defs {
		if v, ok := strings.CutPrefix(name, "vocab_"); ok {
			if _, ok := Vocab[v]; !ok {
				t.Errorf("schema defines vocabulary %s, which Vocab does not have", v)
			}
		}
	}
}

// TestSchemaErrorCodesMatchErrorsGo holds responses.schema.json's
// engine_error_code enum to errors.go's Code constants, in order.
func TestSchemaErrorCodesMatchErrorsGo(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "errors.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var codes []string
	for _, decl := range f.Decls {
		g, ok := decl.(*ast.GenDecl)
		if !ok || g.Tok != token.CONST {
			continue
		}
		for _, spec := range g.Specs {
			vs := spec.(*ast.ValueSpec)
			for i, name := range vs.Names {
				lit, ok := vs.Values[i].(*ast.BasicLit)
				if !strings.HasPrefix(name.Name, "Code") || !ok || lit.Kind != token.STRING {
					continue
				}
				s, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatal(err)
				}
				codes = append(codes, s)
			}
		}
	}
	if len(codes) == 0 {
		t.Fatal("no Code constants in errors.go")
	}
	doc := loadSchema(t, "responses.schema.json")
	def, _ := doc["$defs"].(map[string]any)["engine_error_code"].(map[string]any)
	got, _ := strs(def["enum"])
	if !slices.Equal(got, codes) {
		t.Errorf("responses.schema.json engine_error_code %v, errors.go has %v", got, codes)
	}
}
