package tsgen

import (
	"encoding/json"
	"go/types"
	"testing"
)

func TestJSONSchemaMapsEncodingJSON(t *testing.T) {
	l := NewLoader()
	pkg, err := l.CheckFiles(selfPkg, "schema_fixture_types_test.go")
	if err != nil {
		t.Fatal(err)
	}
	root, err := LookupIn(pkg, "card")
	if err != nil {
		t.Fatal(err)
	}
	comps, err := JSONSchema(SchemaOptions{Roots: []types.Type{root}, RefPrefix: "#/", Fset: l.FileSet()})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(comps)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"card":{"description":"card exercises the encoding/json rules the generator follows.","properties":{` +
		`"colour":{"$ref":"#/colour"},` +
		`"counts":{"additionalProperties":{"maximum":255,"minimum":0,"type":"integer"},"type":"object"},` +
		`"maybe":{"format":"int32","type":["integer","null"]},` +
		`"nested":{"$ref":"#/base"},` +
		`"omitted":{"format":"int32","type":"integer"},` +
		`"shared":{"description":"Shared is promoted into every embedding struct.","type":"string"},` +
		`"tags":{"items":{"type":"string"},"type":["array","null"]}},` +
		`"required":["colour","maybe","nested","shared","tags"],"type":"object","x-go-type":"github.com/adams-shaun/gorge/internal/tsgen.card"},` +
		`"colour":{"description":"colour is an enum: its constants become the schema's enum.","enum":["red","blue"],"type":"string","x-go-type":"github.com/adams-shaun/gorge/internal/tsgen.colour"},` +
		`"base":{"properties":{"shared":{"description":"Shared is promoted into every embedding struct.","type":"string"}},"required":["shared"],"type":"object","x-go-type":"github.com/adams-shaun/gorge/internal/tsgen.base"}}`
	var got, exp any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want), &exp); err != nil {
		t.Fatal(err)
	}
	g, _ := json.Marshal(got)
	e, _ := json.Marshal(exp)
	if string(g) != string(e) {
		t.Errorf("JSONSchema(card):\n got %s\nwant %s", g, e)
	}
}

func TestJSONSchemaRefusesUnstatedMarshaler(t *testing.T) {
	l := NewLoader()
	pkg, err := l.CheckFiles(selfPkg, "schema_fixture_types_test.go")
	if err != nil {
		t.Fatal(err)
	}
	ty, err := LookupIn(pkg, "selfMarshal")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := JSONSchema(SchemaOptions{Roots: []types.Type{ty}}); err == nil {
		t.Fatal("a json.Marshaler without an Override was documented as its struct")
	}
	ov := map[string]map[string]any{selfPkg + ".selfMarshal": {"type": "string"}}
	if _, err := JSONSchema(SchemaOptions{Roots: []types.Type{ty}, Overrides: ov}); err != nil {
		t.Fatalf("with an Override: %v", err)
	}
}
