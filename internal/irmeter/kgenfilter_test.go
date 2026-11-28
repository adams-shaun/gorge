package irmeter

import (
	"os"
	"path/filepath"
	"testing"
)

// TestKgenFilterSection pins the kgen-filter section on a three-file
// fixture: one typed-only site (errTwoColours; the errNoTypedReading
// sentinel the typed reader matches with errors.Is is a fallback, not a
// site), one shared setter, three string-only sites (the fallback call from
// the typed reader does not make parseFilter's own site shared), and one
// site no reader reaches.
func TestKgenFilterSection(t *testing.T) {
	root := "testdata/kgenfilter"
	files, err := filepath.Glob(filepath.Join(root, "dirac/kgen/*.go"))
	if err != nil || len(files) != 3 {
		t.Fatalf("precondition: the fixture holds three Go files, got %d (%v)", len(files), err)
	}
	if _, err := os.Stat(filepath.Join(root, "dirac/kgen/filter_ir.go")); err != nil {
		t.Fatalf("precondition: the fixture has a typed reader file: %v", err)
	}
	k, err := MeasureKgenFilter(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := (KgenFilter{TypedOnly: 1, StringOnly: 3, Shared: 1}); k != want {
		t.Fatalf("got %+v, want %+v", k, want)
	}
	if got, want := k.DropRow(), "kgen_filter_refuse_drop_pct\tdirac/kgen\t66.7"; got != want {
		t.Errorf("drop row %q, want %q", got, want)
	}
	rows := k.Rows()
	if len(rows) != 3 || rows[0].Metric != "kgen_filter_refuse_shared" || rows[2].Value != 1 {
		t.Errorf("rows %v", rows)
	}
	if (KgenFilter{}).DropPct() != 0 {
		t.Error("no string-only site: want a 0 drop")
	}
}
