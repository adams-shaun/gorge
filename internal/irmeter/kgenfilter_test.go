package irmeter

import (
	"os"
	"path/filepath"
	"strings"
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
	// refuse_sites_all differs, so the share names its denominator.
	printed := func(k KgenFilter, refuseSites int) string {
		t.Helper()
		var b strings.Builder
		in := []Row{{"refuse_sites", kgenScope, refuseSites}, {"refuse_sites_all", kgenScope, 1000}}
		if err := Print(&b, in, k); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	if got, want := printed(k, 8), "refuse_sites\tdirac/kgen\t8\n"+
		"refuse_sites_all\tdirac/kgen\t1000\n"+
		"kgen_filter_refuse_shared\tdirac/kgen\t1\n"+
		"kgen_filter_refuse_string_only\tdirac/kgen\t3\n"+
		"kgen_filter_refuse_typed_only\tdirac/kgen\t1\n"+
		"kgen_filter_refuse_drop_pct\tdirac/kgen\t66.7\n"+
		"kgen_filter_refuse_sites\tdirac/kgen\t5\n"+
		"kgen_filter_refuse_site_share_pct\tdirac/kgen\t62.5\n"+
		"kgen_refuse_sites_removed_by_typed\tdirac/kgen\t2\n"; got != want {
		t.Errorf("printed %q, want %q", got, want)
	}
	noString := KgenFilter{TypedOnly: 2, Shared: 1}
	if noString.StringOnly != 0 || noString.TypedOnly <= noString.StringOnly {
		t.Fatalf("precondition: no string-only site and a negative difference to clamp: %+v", noString)
	}
	got := printed(noString, 0)
	for _, want := range []string{
		"\nkgen_filter_refuse_sites\tdirac/kgen\t3\n",
		"\nkgen_filter_refuse_site_share_pct\tdirac/kgen\t0.0\n",
		"\nkgen_refuse_sites_removed_by_typed\tdirac/kgen\t0\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("printed %q, want the row %q", got, want)
		}
	}
}
