package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fixture holds only synthetic names and invented variants.
const fixtureData = `{"_OracleCard":"Card","Name":"Zz One","Rules":[{"_Rule":"Alpha","args":{"_Action":"Beta"}}]}
{"_OracleCard":"Card","Name":"Zz Two","Rules":[{"_Rule":"Alpha","args":{"_Action":"Gamma"}}]}
{"_OracleCard":"Card","Name":"Zz Three","Rules":[{"_Rule":"Alpha","args":{"_Action":"Beta"}}]}
`

const fixtureSchema = `type Rule =
| { "_Rule": "Alpha", "args": Action };
type Action =
| { "_Action": "Beta" }
| { "_Action": "Gamma" }
| { "_OracleCard": "Card", Name: string, Rules: Array<Rule>, };
`

func TestReportOnFixture(t *testing.T) {
	dir := t.TempDir()
	data, schema := filepath.Join(dir, "d.json"), filepath.Join(dir, "s.d.ts")
	if err := os.WriteFile(data, []byte(fixtureData), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(schema, []byte(fixtureSchema), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := run([]string{"report", "-slice", "all", "-data", data, "-schema", schema}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	want := "Action.Beta\t2\tno\tyes\n" +
		"Action.Gamma\t1\tno\tyes\n" +
		"OracleCard.Card\t3\tno\tyes\n" +
		"Rule.Alpha\t3\tno\tyes\n" +
		"total\tslice=all\tcards=3\tvariants=4\tregistered=0\tnot-in-schema=0\n"
	if out.String() != want {
		t.Errorf("got\n%s\nwant\n%s", out.String(), want)
	}
	// A variant missing from the schema fails the report.
	if err := os.WriteFile(schema, []byte(strings.Replace(fixtureSchema, `| { "_Action": "Gamma" }`+"\n", "", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := run([]string{"report", "-slice", "all", "-data", data, "-schema", schema}, &out, &errb); code != 1 {
		t.Errorf("exit %d, want 1 for a variant missing from the schema", code)
	}
}
