package setdata_test

import (
	"testing"

	_ "github.com/adams-shaun/gorge/engine/rules"

	"github.com/adams-shaun/gorge/draft/limited/setdata"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// sheetEntries are the measured card entries (a card on one sheet of one
// collation counts once) of each embedded set across every collation.
var sheetEntries = map[string]int{"FDN": 1896, "BLB": 1809}

// TestSupportEverySheetCard is the support gate for the embedded specs:
// every card on every sheet of every collation resolves in the card corpus
// and is fully supported by the engine, so a generated pack never names a
// card the rules core cannot play. It runs the same check cmd/cardfuzz
// uses to build its pools: reg.Unsupported(c, effects.Supported()) is
// empty.
//
// The test lives in this external test package so the effects import never
// reaches the pure setdata package.
func TestSupportEverySheetCard(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	if reg.Len() == 0 {
		t.Fatal("corpus registry is empty: the support gate checked nothing (is .cards present?)")
	}
	sup := effects.Supported()
	if len(sup) == 0 {
		t.Fatal("effects.Supported() is empty: the support gate would pass anything")
	}
	// rules' package init registers the non-API primitives (kw:*, trig:*,
	// stat:*, repl:*); without it Supported() knows only the effect APIs
	// and this gate would reject every keyword card.
	if !sup["kw:Flying"] {
		t.Fatal("effects.Supported() lacks kw:Flying: the rules package did not register the non-API primitives")
	}
	checked := 0
	for _, code := range setdata.Codes() {
		collations := setdata.Collations(code)
		if len(collations) == 0 {
			t.Fatalf("%s exposes no collations", code)
		}
		for _, c := range collations {
			spec := mustLoadCollation(t, code, c)
			for sheet, sh := range spec.Config.Sheets {
				for _, sc := range sh.Cards {
					card, ok := reg.Lookup(sc.Name)
					if !ok {
						t.Errorf("%s %s %s: %q does not resolve in the corpus", code, c, sheet, sc.Name)
						continue
					}
					if unsupported := reg.Unsupported(card, sup); len(unsupported) > 0 {
						t.Errorf("%s %s %s: %q is unsupported: %v", code, c, sheet, sc.Name, unsupported)
					}
					checked++
				}
			}
		}
	}
	if want := sheetEntries["FDN"] + sheetEntries["BLB"]; checked != want {
		t.Errorf("checked %d sheet cards, want the %d FDN + %d BLB sheet entries", checked, sheetEntries["FDN"], sheetEntries["BLB"])
	}
}
