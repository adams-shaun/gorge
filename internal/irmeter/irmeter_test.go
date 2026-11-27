package irmeter

import (
	"reflect"
	"testing"
)

// TestMeasureFixture pins the exact rows for the hand-written fixture tree:
// two of each counted shape, and one of each shape that must not count (a
// refuse call in a _test.go file, a Param read whose key is not cards.PK*).
func TestMeasureFixture(t *testing.T) {
	rows, err := Measure("testdata/fixture")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, r := range rows {
		got[r.Metric+"\t"+r.Scope] = r.Value
	}
	want := map[string]int{
		"go_files\tengine/effects":                     1,
		"go_lines\tengine/effects":                     21,
		"strings_calls\tengine/effects":                2,
		"forge_literals\tengine/effects":               4,
		"param_string_reads\tengine/effects":           4,
		"param_code_reads\tengine/effects":             2,
		"param_raw_reads\tengine/effects":              2,
		"param_string_reads_filter\tengine/effects":    2,
		"param_string_reads_reference\tengine/effects": 1,
		"param_string_reads_cost\tengine/effects":      0,
		"param_string_reads_condition\tengine/effects": 0,
		"param_string_reads_value\tengine/effects":     1,
		"param_string_reads_other\tengine/effects":     0,
		"go_files\tdirac/kgen":                         1,
		"go_lines\tdirac/kgen":                         12,
		"strings_calls\tdirac/kgen":                    0,
		"forge_literals\tdirac/kgen":                   0,
		"refuse_sites\tdirac/kgen":                     2,
		"refuse_sites_all\tdirac/kgen":                 3,
		"refuse_files\tdirac/kgen":                     1,
	}
	if got["go_files\tdirac/kgen"] != 1 {
		t.Fatalf("precondition: the fixture's dirac/kgen holds one non-test file, got %d", got["go_files\tdirac/kgen"])
	}
	for k, v := range want {
		g, ok := got[k]
		if !ok {
			t.Errorf("missing row %q", k)
			continue
		}
		if g != v {
			t.Errorf("%q = %d, want %d", k, g, v)
		}
	}
	// Scopes absent from the fixture still print, as zeros.
	for k, v := range got {
		if _, ok := want[k]; !ok && v != 0 {
			t.Errorf("unexpected nonzero row %q = %d", k, v)
		}
	}
	if len(rows) != 59 {
		t.Errorf("got %d rows, want 59 (4 per scope, 9 more per engine scope, 3 more for kgen)", len(rows))
	}
}

// TestMeasureIsDeterministic: two measurements of one tree are equal and
// sorted by metric then scope.
func TestMeasureIsDeterministic(t *testing.T) {
	a, err := Measure("testdata/fixture")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Measure("testdata/fixture")
	if err != nil {
		t.Fatal(err)
	}
	if len(a) == 0 || !reflect.DeepEqual(a, b) {
		t.Fatalf("two measurements differ or are empty: %d vs %d rows", len(a), len(b))
	}
	for i := 1; i < len(a); i++ {
		p, q := a[i-1], a[i]
		if p.Metric > q.Metric || (p.Metric == q.Metric && p.Scope >= q.Scope) {
			t.Fatalf("rows %d,%d out of order: %v %v", i-1, i, p, q)
		}
	}
}

// liveBaseline holds the counts measured at commit 1dc038d51c. A change that
// moves one past the 5% window updates this table in the same change.
var liveBaseline = []struct {
	metric string
	scopes []string // summed
	want   int
}{
	{"refuse_sites", []string{"dirac/kgen"}, 4706},
	{"refuse_files", []string{"dirac/kgen"}, 534},
	{"go_files", []string{"dirac/kgen"}, 847},
	{"param_string_reads", engineScopes, 2369},
	{"param_string_reads", []string{"engine/effects"}, 1046},
	{"param_string_reads", []string{"engine/rules"}, 1196},
	{"param_string_reads", []string{"engine/cards"}, 123},
	{"param_string_reads", []string{"engine/events"}, 4},
	{"param_code_reads", engineScopes, 26},
	{"param_raw_reads", engineScopes, 142},
}

var engineScopes = []string{"engine/effects", "engine/rules", "engine/cards", "engine/events"}

// TestMeterPrints measures the real module and holds the headline counts to
// within 5% of the 1dc038d51c baseline.
func TestMeterPrints(t *testing.T) {
	rows, err := Measure("../..")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, r := range rows {
		t.Logf("%s\t%s\t%d", r.Metric, r.Scope, r.Value)
		got[r.Metric+"\t"+r.Scope] = r.Value
	}
	for _, b := range liveBaseline {
		sum := 0
		for _, s := range b.scopes {
			v, ok := got[b.metric+"\t"+s]
			if !ok {
				t.Fatalf("no row %s %s", b.metric, s)
			}
			sum += v
		}
		lo, hi := b.want*95/100, b.want*105/100
		if b.want < 40 { // 5% of a tiny count is under one: allow one either side
			lo, hi = b.want-1, b.want+1
		}
		if sum < lo || sum > hi {
			t.Errorf("%s over %v = %d, want within 5%% of %d [%d, %d]", b.metric, b.scopes, sum, b.want, lo, hi)
		}
	}
	for _, s := range engineScopes {
		fam := 0
		for _, f := range families {
			fam += got["param_string_reads_"+f+"\t"+s]
		}
		if fam != got["param_string_reads\t"+s] {
			t.Errorf("%s: family rows sum to %d, param_string_reads is %d", s, fam, got["param_string_reads\t"+s])
		}
	}
}
