package schemacheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testSchema = `{
  "type": "object",
  "properties": {
    "n": {"$ref": "#/$defs/small"},
    "kind": {"enum": ["a", "b"]},
    "tag": {"const": "x"},
    "word": {"type": "string", "minLength": 2, "pattern": "^[a-z]+$"},
    "list": {"type": "array", "items": {"type": "integer"}, "minItems": 1, "maxItems": 2, "uniqueItems": true},
    "pair": {"type": "array", "prefixItems": [{"type": "string"}], "items": false},
    "one": {"oneOf": [{"type": "integer"}, {"type": "integer", "minimum": 5}]},
    "any": {"anyOf": [{"type": "null"}, {"type": "string"}]},
    "all": {"allOf": [{"type": "integer"}, {"minimum": 1}]},
    "ext": {"type": "object", "patternProperties": {"^x_": {"type": "string"}}, "additionalProperties": false}
  },
  "required": ["n"],
  "additionalProperties": false,
  "$defs": {"small": {"type": "integer", "minimum": 0, "maximum": 9}}
}`

func testSet(t *testing.T, schema string) (*Set, error) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "t.schema.json"), []byte(schema), 0o644); err != nil {
		t.Fatal(err)
	}
	return Load(dir)
}

func TestValidate(t *testing.T) {
	s, err := testSet(t, testSchema)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		doc, wantErr string
	}{
		{`{"n":3,"kind":"a","tag":"x","word":"ab","list":[1,2],"pair":["s"],"one":1,"any":null,"all":2,"ext":{"x_a":"b"}}`, ""},
		{`{}`, "missing n"},
		{`{"n":1,"extra":1}`, "$.extra: unexpected field"},
		{`{"n":10}`, "$.n: 10 is above 9"},
		{`{"n":-1}`, "$.n: -1 is below 0"},
		{`{"n":18446744073709551616}`, "is above 9"},
		{`{"n":1.5}`, "$.n: 1.5 is not of type integer"},
		{`{"n":1,"kind":"c"}`, `$.kind: "c" is not one of`},
		{`{"n":1,"tag":"y"}`, `$.tag: "y", want "x"`},
		{`{"n":1,"word":"a"}`, "shorter than 2"},
		{`{"n":1,"word":"AB"}`, "does not match"},
		{`{"n":1,"list":[]}`, "0 items"},
		{`{"n":1,"list":[1,2,3]}`, "3 items"},
		{`{"n":1,"list":[1,1]}`, "$.list[1]: repeats"},
		{`{"n":1,"pair":["s","t"]}`, "$.pair[1]: not allowed"},
		{`{"n":1,"one":7}`, "matches 2 branches"},
		{`{"n":1,"any":1}`, "$.any: 1 is not of type"},
		{`{"n":1,"all":0}`, "$.all: 0 is below 1"},
		{`{"n":1,"ext":{"y":"b"}}`, "$.ext.y: unexpected field"},
		{`{"n":1,"ext":{"x_a":1}}`, "$.ext.x_a: 1 is not of type"},
	} {
		doc, err := Decode([]byte(tc.doc))
		if err != nil {
			t.Fatal(err)
		}
		err = s.Validate("t.schema.json", doc)
		switch {
		case tc.wantErr == "" && err != nil:
			t.Errorf("%s: %v", tc.doc, err)
		case tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)):
			t.Errorf("%s: error %v, want %q", tc.doc, err, tc.wantErr)
		}
	}
}

func TestLoadRefusesAKeywordItCannotEvaluate(t *testing.T) {
	if _, err := testSet(t, `{"properties":{"a":{"multipleOf":2}}}`); err == nil || !strings.Contains(err.Error(), "multipleOf") {
		t.Fatalf("error %v, want one naming multipleOf", err)
	}
}
