package tsgen

import (
	"fmt"
	"go/constant"
	"go/token"
	"go/types"
	"math"
	"sort"
	"strings"
)

// SchemaOptions drives one JSON Schema generation (JSONSchema). Roots are
// walked transitively, exactly as Generate walks them for TypeScript, so the
// published API reference (docs/site/api/openapi.json) is derived from the
// same source the client's protocol.ts is.
//
// RefPrefix is prepended to a component name in every $ref
// ("#/components/schemas/" for OpenAPI). Overrides replace the generated
// schema of one named type, keyed "<import path>.<Name>": they are how a type
// that marshals itself (MarshalText, MarshalJSON) states its real wire form.
// A type implementing json.Marshaler with no override is an error, so a
// custom encoding can never be documented as its Go struct by accident.
//
// Fset is the file set the roots were type-checked with (Loader.FileSet).
// It orders an enum's constants by file and offset: the source importer
// parses a package's files concurrently, so a raw token.Pos order is not
// deterministic across runs.
type SchemaOptions struct {
	Roots     []types.Type
	RefPrefix string
	Overrides map[string]map[string]any
	Fset      *token.FileSet
}

// GoTypeKey is the extension key every generated component carries: the Go
// type it was derived from, "<import path>.<Name>". A consumer that merges
// generated components into a hand-written document uses it to tell the two
// apart.
const GoTypeKey = "x-go-type"

type schemaGen struct {
	o     SchemaOptions
	comps map[string]map[string]any
	owner map[string]string // component name -> GoTypeKey value, for collisions
	cmts  *comments
	err   error
}

// JSONSchema renders every named struct (and every named string type that
// has declared constants, as an enum) reachable from the roots as a JSON
// Schema 2020-12 object, keyed by component name. The mapping follows
// encoding/json, not the Go type system:
//
//   - a field without omitempty is always emitted, so it is "required"; a
//     nil pointer, slice, map or interface in such a field marshals as null,
//     so its schema also admits null. With omitempty a nil value is omitted
//     instead and the schema does not admit null.
//   - an embedded struct without a json name is flattened into its parent.
//   - []byte is a base64 string; [N]T is an N-element array; a map is an
//     object; encoding/json.RawMessage and interfaces are any JSON value.
//   - a type with MarshalText is a string.
//
// Descriptions are the first paragraph of each declaration's doc comment.
func JSONSchema(o SchemaOptions) (map[string]map[string]any, error) {
	g := &schemaGen{o: o, comps: map[string]map[string]any{}, owner: map[string]string{}, cmts: newComments()}
	for _, r := range o.Roots {
		g.schema(r)
		if g.err != nil {
			return nil, g.err
		}
	}
	return g.comps, nil
}

func (g *schemaGen) fail(format string, a ...any) map[string]any {
	if g.err == nil {
		g.err = fmt.Errorf("tsgen: "+format, a...)
	}
	return map[string]any{}
}

func typeKey(n *types.Named) string {
	if p := n.Obj().Pkg(); p != nil {
		return p.Path() + "." + n.Obj().Name()
	}
	return n.Obj().Name()
}

// hasMethod reports whether t or *t has a method called name.
func hasMethod(t types.Type, name string) bool {
	for _, tt := range []types.Type{t, types.NewPointer(t)} {
		ms := types.NewMethodSet(tt)
		for i := 0; i < ms.Len(); i++ {
			if ms.At(i).Obj().Name() == name {
				return true
			}
		}
	}
	return false
}

// schema is the schema of one Go type in field position.
func (g *schemaGen) schema(t types.Type) map[string]any {
	t = types.Unalias(t)
	if g.err != nil {
		return map[string]any{}
	}
	if isRawMessage(t) {
		return map[string]any{}
	}
	if n, ok := t.(*types.Named); ok {
		key := typeKey(n)
		if ov, ok := g.o.Overrides[key]; ok {
			return copyMap(ov)
		}
		if hasMethod(n, "MarshalJSON") {
			return g.fail("%s implements json.Marshaler: give it an Override", key)
		}
		if hasMethod(n, "MarshalText") {
			return map[string]any{"type": "string"}
		}
		switch u := n.Underlying().(type) {
		case *types.Struct:
			return g.component(n, func() map[string]any { return g.object(n, u) })
		case *types.Basic:
			if u.Info()&types.IsString != 0 {
				if vals := enumValues(n, g.o.Fset); len(vals) > 0 {
					return g.component(n, func() map[string]any {
						s := map[string]any{"type": "string", "enum": vals}
						g.describe(s, g.cmts.typeDoc(n))
						return s
					})
				}
			}
		}
	}
	switch u := t.Underlying().(type) {
	case *types.Pointer:
		return nullable(g.schema(u.Elem()))
	case *types.Interface:
		return map[string]any{}
	case *types.Basic:
		return basicSchema(u)
	case *types.Slice:
		if b, ok := u.Elem().Underlying().(*types.Basic); ok && b.Kind() == types.Uint8 {
			return map[string]any{"type": "string", "contentEncoding": "base64"}
		}
		return map[string]any{"type": "array", "items": g.schema(u.Elem())}
	case *types.Array:
		return map[string]any{"type": "array", "items": g.schema(u.Elem()), "minItems": u.Len(), "maxItems": u.Len()}
	case *types.Map:
		return map[string]any{"type": "object", "additionalProperties": g.schema(u.Elem())}
	case *types.Struct:
		return g.object(nil, u)
	}
	return g.fail("no JSON Schema for %v", t)
}

// component registers n's schema under its name once and returns a $ref.
func (g *schemaGen) component(n *types.Named, build func() map[string]any) map[string]any {
	name, key := n.Obj().Name(), typeKey(n)
	if prev, ok := g.owner[name]; ok {
		if prev != key {
			return g.fail("two types named %s: %s and %s", name, prev, key)
		}
	} else {
		g.owner[name] = key
		g.comps[name] = nil // placeholder: a recursive type refers to itself
		s := build()
		s[GoTypeKey] = key
		g.comps[name] = s
	}
	return map[string]any{"$ref": g.o.RefPrefix + name}
}

// object is a struct's object schema; n is nil for an anonymous struct.
func (g *schemaGen) object(n *types.Named, st *types.Struct) map[string]any {
	props := map[string]any{}
	var required []string
	g.fields(n, st, props, &required, map[string]bool{})
	s := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		sort.Strings(required)
		s["required"] = required
	}
	if n != nil {
		g.describe(s, g.cmts.structDoc(n))
	}
	return s
}

// fields adds st's wire fields to props. seen holds names an outer struct
// already claimed: encoding/json lets the shallower field win.
func (g *schemaGen) fields(n *types.Named, st *types.Struct, props map[string]any, required *[]string, seen map[string]bool) {
	var embedded []*types.Var
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		tag, _ := lookupTag(st.Tag(i), "json")
		if tag == "-" {
			continue
		}
		name, opts, _ := strings.Cut(tag, ",")
		if f.Embedded() && name == "" {
			if _, ok := deref(f.Type()).Underlying().(*types.Struct); ok {
				embedded = append(embedded, f)
				continue
			}
		}
		if !f.Exported() {
			continue
		}
		if name == "" {
			name = f.Name()
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		omit := hasOpt(opts, "omitempty") || hasOpt(opts, "omitzero")
		var fs map[string]any
		if hasOpt(opts, "string") {
			fs = map[string]any{"type": "string"}
		} else {
			fs = g.schema(f.Type())
		}
		if g.err != nil {
			return
		}
		if !omit || alwaysEmitted(f.Type(), opts) {
			*required = append(*required, name)
			if !omit && nilable(f.Type()) {
				fs = nullable(fs)
			}
		}
		if omit {
			fs = stripNull(fs)
		}
		if n != nil {
			g.describe(fs, g.cmts.fieldDoc(n, f.Name()))
		}
		props[name] = fs
	}
	for _, f := range embedded {
		et := deref(f.Type())
		en, _ := types.Unalias(et).(*types.Named)
		g.fields(en, et.Underlying().(*types.Struct), props, required, seen)
	}
}

func hasOpt(opts, want string) bool {
	for _, o := range strings.Split(opts, ",") {
		if o == want {
			return true
		}
	}
	return false
}

// alwaysEmitted reports whether omitempty can never drop a value of t:
// encoding/json treats no struct (and no non-empty array) as empty.
func alwaysEmitted(t types.Type, opts string) bool {
	if hasOpt(opts, "omitzero") {
		return false
	}
	switch u := types.Unalias(t).Underlying().(type) {
	case *types.Struct:
		return true
	case *types.Array:
		return u.Len() > 0
	}
	return false
}

// nilable reports whether a value of t can marshal as null.
func nilable(t types.Type) bool {
	if isRawMessage(t) {
		return true
	}
	switch types.Unalias(t).Underlying().(type) {
	case *types.Pointer, *types.Slice, *types.Map, *types.Interface:
		return true
	}
	return false
}

// nullable widens s to also admit null.
func nullable(s map[string]any) map[string]any {
	if len(s) == 0 {
		return s // any value already includes null
	}
	if ty, ok := s["type"].(string); ok && s["enum"] == nil {
		out := copyMap(s)
		out["type"] = []any{ty, "null"}
		return out
	}
	if _, ok := s["type"].([]any); ok {
		return s
	}
	if ao, ok := s["anyOf"].([]any); ok && len(ao) == 2 {
		return s // already nullable
	}
	return map[string]any{"anyOf": []any{s, map[string]any{"type": "null"}}}
}

// stripNull undoes nullable for an omitempty field, whose nil is omitted.
func stripNull(s map[string]any) map[string]any {
	if ty, ok := s["type"].([]any); ok && len(ty) == 2 && ty[1] == "null" {
		out := copyMap(s)
		out["type"] = ty[0]
		return out
	}
	if ao, ok := s["anyOf"].([]any); ok && len(ao) == 2 {
		if nul, ok := ao[1].(map[string]any); ok && nul["type"] == "null" && len(nul) == 1 {
			return ao[0].(map[string]any)
		}
	}
	return s
}

func (g *schemaGen) describe(s map[string]any, doc string) {
	if p := firstParagraph(doc); p != "" {
		s["description"] = p
	}
}

// firstParagraph joins the doc comment's first paragraph into one line.
func firstParagraph(doc string) string {
	para, _, _ := strings.Cut(strings.TrimSpace(doc), "\n\n")
	return strings.Join(strings.Fields(para), " ")
}

func basicSchema(b *types.Basic) map[string]any {
	info := b.Info()
	switch {
	case info&types.IsBoolean != 0:
		return map[string]any{"type": "boolean"}
	case info&types.IsString != 0:
		return map[string]any{"type": "string"}
	case info&types.IsFloat != 0:
		return map[string]any{"type": "number"}
	case info&types.IsInteger != 0:
		s := map[string]any{"type": "integer"}
		switch b.Kind() {
		case types.Int8:
			s["minimum"], s["maximum"] = math.MinInt8, math.MaxInt8
		case types.Int16:
			s["minimum"], s["maximum"] = math.MinInt16, math.MaxInt16
		case types.Int32:
			s["format"] = "int32"
		case types.Int, types.Int64:
			s["format"] = "int64"
		case types.Uint8:
			s["minimum"], s["maximum"] = 0, math.MaxUint8
		case types.Uint16:
			s["minimum"], s["maximum"] = 0, math.MaxUint16
		case types.Uint32:
			s["minimum"], s["maximum"] = 0, uint64(math.MaxUint32)
		case types.Uint, types.Uint64, types.Uintptr:
			s["minimum"] = 0
			s["format"] = "uint64"
		}
		return s
	}
	return map[string]any{}
}

// enumValues lists the string constants n's package declares of type n, in
// declaration order (file name, then offset). Without a file set the order
// is by value.
func enumValues(n *types.Named, fset *token.FileSet) []any {
	pkg := n.Obj().Pkg()
	if pkg == nil {
		return nil
	}
	type kv struct {
		file string
		off  int
		val  string
	}
	var vals []kv
	for _, name := range pkg.Scope().Names() {
		c, ok := pkg.Scope().Lookup(name).(*types.Const)
		if !ok || !types.Identical(c.Type(), n) || c.Val().Kind() != constant.String {
			continue
		}
		v := kv{val: constant.StringVal(c.Val())}
		if fset != nil {
			pos := fset.Position(c.Pos())
			v.file, v.off = pos.Filename, pos.Offset
		}
		vals = append(vals, v)
	}
	sort.Slice(vals, func(i, j int) bool {
		a, b := vals[i], vals[j]
		if a.file != b.file {
			return a.file < b.file
		}
		if a.off != b.off {
			return a.off < b.off
		}
		return a.val < b.val
	})
	out := make([]any, 0, len(vals))
	seen := map[string]bool{}
	for _, v := range vals {
		if !seen[v.val] {
			seen[v.val] = true
			out = append(out, v.val)
		}
	}
	return out
}

func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
