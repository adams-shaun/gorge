package ogram

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

// texpr is one type expression of the schema's type description.
type texpr struct {
	kind  texprKind
	ref   string   // tRef: a type name
	names []string // tFields: field names, in schema order
	elems []*texpr // tFields: field types; tTuple: element types; tOption, tVec: elems[0]
}

type texprKind uint8

const (
	tUnit texprKind = iota
	tRef
	tFields
	tTuple
	tOption
	tVec
)

// tdef is one named type: a struct, or an enum with its variants.
type tdef struct {
	enum, adjacent bool
	body           *texpr            // struct
	variants       map[string]*texpr // enum
}

// Schema is the type description of the readings (193.json).
type Schema struct{ types map[string]*tdef }

// LoadSchema reads the schema file in dir.
func LoadSchema(dir string) (*Schema, error) {
	b, err := os.ReadFile(filepath.Join(dir, schemaFile))
	if err != nil {
		return nil, err
	}
	var doc struct {
		Version int `json:"schema_version"`
		Schema  struct {
			Types map[string]map[string]json.RawMessage `json:"types"`
		} `json:"schema"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", schemaFile, err)
	}
	if doc.Version != schemaVersion {
		return nil, fmt.Errorf("%s: schema_version %d, want %d", schemaFile, doc.Version, schemaVersion)
	}
	s := &Schema{types: map[string]*tdef{}}
	for name, d := range doc.Schema.Types {
		td, err := parseDef(d)
		if err != nil {
			return nil, fmt.Errorf("%s: type %s: %w", schemaFile, name, err)
		}
		s.types[name] = td
	}
	if s.types["ObjectFilter"] == nil || s.types["Card"] == nil {
		return nil, fmt.Errorf("%s: no Card or ObjectFilter type", schemaFile)
	}
	return s, nil
}

func parseDef(d map[string]json.RawMessage) (*tdef, error) {
	if raw, ok := d["Struct"]; ok {
		body, err := parseTexpr(raw)
		return &tdef{body: body}, err
	}
	raw, ok := d["Enum"]
	if !ok {
		return nil, fmt.Errorf("neither Struct nor Enum")
	}
	var e struct {
		Adjacent bool                 `json:"adjacent"`
		Variants [][2]json.RawMessage `json:"variants"`
	}
	if err := json.Unmarshal(raw, &e); err != nil {
		return nil, err
	}
	td := &tdef{enum: true, adjacent: e.Adjacent, variants: map[string]*texpr{}}
	for _, v := range e.Variants {
		var name string
		if err := json.Unmarshal(v[0], &name); err != nil {
			return nil, err
		}
		body, err := parseTexpr(v[1])
		if err != nil {
			return nil, fmt.Errorf("variant %s: %w", name, err)
		}
		td.variants[name] = body
	}
	return td, nil
}

func parseTexpr(raw json.RawMessage) (*texpr, error) {
	var unit string
	if json.Unmarshal(raw, &unit) == nil {
		if unit != "Unit" {
			return nil, fmt.Errorf("unknown type expression %q", unit)
		}
		return &texpr{kind: tUnit}, nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil || len(m) != 1 {
		return nil, fmt.Errorf("unknown type expression %s", raw)
	}
	for k, v := range m {
		switch k {
		case "Option", "Vec":
			e, err := parseTexpr(v)
			kind := tOption
			if k == "Vec" {
				kind = tVec
			}
			return &texpr{kind: kind, elems: []*texpr{e}}, err
		case "Tuple":
			var raws []json.RawMessage
			if err := json.Unmarshal(v, &raws); err != nil {
				return nil, err
			}
			t := &texpr{kind: tTuple}
			for _, r := range raws {
				e, err := parseTexpr(r)
				if err != nil {
					return nil, err
				}
				t.elems = append(t.elems, e)
			}
			return t, nil
		case "Named":
			if json.Unmarshal(v, &unit) == nil {
				return &texpr{kind: tRef, ref: unit}, nil
			}
			var fields [][2]json.RawMessage
			if err := json.Unmarshal(v, &fields); err != nil {
				return nil, err
			}
			t := &texpr{kind: tFields}
			for _, f := range fields {
				var name string
				if err := json.Unmarshal(f[0], &name); err != nil {
					return nil, err
				}
				e, err := parseTexpr(f[1])
				if err != nil {
					return nil, fmt.Errorf("field %s: %w", name, err)
				}
				t.names = append(t.names, name)
				t.elems = append(t.elems, e)
			}
			return t, nil
		}
	}
	return nil, fmt.Errorf("unknown type expression %s", raw)
}

// Operand is one object filter of a reading: an ObjectFilter value that is
// not itself part of an enclosing ObjectFilter (AllOf's members are part of
// the AllOf; a filter inside a count inside a filter is an operand of its
// own). Group is the mtgish filter group enum its context maps to.
type Operand struct {
	Group string
	Node  json.RawMessage
}

// Lower lowers the operand in its group.
func (o Operand) Lower() (*ir.Filter, error) { return LowerFilterIn(o.Group, o.Node) }

var (
	defaultOnce   sync.Once
	defaultSchema *Schema
	defaultErr    error
)

// FilterOperands lists the object filter operands of tree, a reading
// (Face.Tree), in document order, typed by the schema in DataDir().
func FilterOperands(tree json.RawMessage) ([]Operand, error) {
	defaultOnce.Do(func() { defaultSchema, defaultErr = LoadSchema(DataDir()) })
	if defaultErr != nil {
		return nil, defaultErr
	}
	return defaultSchema.FilterOperands(tree)
}

// FilterOperands is the package function with this schema.
func (s *Schema) FilterOperands(tree json.RawMessage) ([]Operand, error) {
	var v any
	if err := json.Unmarshal(tree, &v); err != nil {
		return nil, err
	}
	w := &walker{s: s}
	if err := w.value(&texpr{kind: tRef, ref: "Card"}, v, false, "", nil); err != nil {
		return nil, err
	}
	return w.out, nil
}

type walker struct {
	s   *Schema
	out []Operand
}

// value walks v of type t. inside is set within an ObjectFilter's own
// structure; field is the struct field v hangs from; dom is the object domain
// a sibling field of that struct declares (TargetSpec's kind), or nil.
func (w *walker) value(t *texpr, v any, inside bool, field string, dom any) error {
	switch t.kind {
	case tUnit:
		return nil
	case tOption:
		if v == nil {
			return nil
		}
		return w.value(t.elems[0], v, inside, field, dom)
	case tVec:
		l, ok := v.([]any)
		if !ok && v != nil {
			return fmt.Errorf("field %s: want a list, got %T", field, v)
		}
		for _, e := range l {
			if err := w.value(t.elems[0], e, inside, field, dom); err != nil {
				return err
			}
		}
		return nil
	case tTuple:
		if len(t.elems) == 1 {
			return w.value(t.elems[0], v, inside, field, dom)
		}
		l, ok := v.([]any)
		if !ok || len(l) != len(t.elems) {
			return fmt.Errorf("field %s: want a %d-tuple, got %T", field, len(t.elems), v)
		}
		for i, e := range l {
			if err := w.value(t.elems[i], e, inside, field, dom); err != nil {
				return err
			}
		}
		return nil
	case tFields:
		return w.fields(t, v, inside)
	}
	// tRef.
	if t.ref == "ObjectFilter" && !inside {
		node, err := json.Marshal(v)
		if err != nil {
			return err
		}
		w.out = append(w.out, Operand{Group: group(dom, field), Node: node})
	}
	d := w.s.types[t.ref]
	if d == nil {
		return nil // a leaf: String, u32, CardType, ...
	}
	return w.def(t.ref, d, v, t.ref == "ObjectFilter", field)
}

func (w *walker) fields(t *texpr, v any, inside bool) error {
	m, ok := v.(map[string]any)
	if !ok {
		return fmt.Errorf("want an object, got %T", v)
	}
	var dom any
	for i, name := range t.names {
		if t.elems[i].kind != tRef {
			continue
		}
		switch t.elems[i].ref {
		case "ObjectDomain":
			dom = m[name]
		case "TargetKind":
			if o, ok := m[name].(map[string]any); ok && o["Object"] != nil {
				dom = o["Object"]
			}
		}
	}
	for i, name := range t.names {
		if x, ok := m[name]; ok {
			if err := w.value(t.elems[i], x, inside, name, dom); err != nil {
				return err
			}
		}
	}
	return nil
}

func (w *walker) def(name string, d *tdef, v any, inside bool, field string) error {
	if !d.enum {
		return w.value(d.body, v, inside, field, nil)
	}
	var variant string
	var payload any
	switch x := v.(type) {
	case string:
		variant = x
	case map[string]any:
		if d.adjacent {
			variant, _ = x["kind"].(string)
			payload = x["args"]
		} else if len(x) == 1 {
			for k, p := range x {
				variant, payload = k, p
			}
		}
	}
	body, ok := d.variants[variant]
	if !ok {
		return fmt.Errorf("%s: unknown variant %q", name, variant)
	}
	return w.value(body, payload, inside, field, nil)
}

// group maps an operand's context to an mtgish filter group enum: the object
// domain its target declares, else the field it hangs from. "" is a domain
// mtgish has no group for (abilities).
func group(dom any, field string) string {
	if d, ok := dom.(map[string]any); ok {
		var zones []string
		zl, _ := d["zones"].([]any)
		for _, z := range zl {
			if s, ok := z.(string); ok {
				zones = append(zones, s)
			}
		}
		one := ""
		if len(zones) == 1 {
			one = zones[0]
		}
		switch d["kind"] {
		case "Permanent":
			return "Permanents"
		case "Spell":
			return "Spells"
		case "Ability":
			return ""
		}
		switch one {
		case "Battlefield":
			return "Permanents"
		case "Stack":
			return "Spells"
		case "Graveyard":
			return "CardsInGraveyards"
		case "Exile":
			return "CardsInExile"
		case "Library":
			return "CardsInLibrary"
		case "Hand":
			return "CardsInHand"
		}
		return "Cards"
	}
	switch field {
	case "spells":
		return "Spells"
	case "cards":
		return "Cards"
	}
	return "Permanents"
}
