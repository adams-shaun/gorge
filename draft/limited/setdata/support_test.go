package setdata_test

import (
	"os"
	"sort"
	"strings"
	"testing"

	_ "github.com/adams-shaun/gorge/engine/rules"

	"github.com/adams-shaun/gorge/draft/limited/setdata"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// sheetEntries are the measured card entries (a card on one sheet of one
// collation counts once) of each embedded set across every collation; for a
// substitute set (MBC) the entries off its substitute list.
var sheetEntries = map[string]int{"FDN": 1896, "BLB": 1809, "MBC": 2370}

// TestSupportEverySheetCard is the support gate for the embedded specs:
// every card on every sheet of every collation resolves in the card corpus
// and is fully supported by the engine, so a generated pack never names a
// card the rules core cannot play. It runs the same check cmd/cardfuzz
// uses to build its pools: reg.Unsupported(c, effects.Supported()) is
// empty. A card on a set's substitute list (spec.Unplayable) is skipped:
// the booster never deals it, and TestPendingSetCards holds the list to
// exactly the cards the corpus cannot play.
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
		if _, pending := pendingSets[code]; pending {
			continue // TestPendingSetCards holds these to a shrinking list
		}
		collations := setdata.Collations(code)
		if len(collations) == 0 {
			t.Fatalf("%s exposes no collations", code)
		}
		for _, c := range collations {
			spec := mustLoadCollation(t, code, c)
			for sheet, sh := range spec.Config.Sheets {
				for _, sc := range sh.Cards {
					if spec.Unplayable[sc.Name] {
						continue
					}
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
	if want := sheetEntries["FDN"] + sheetEntries["BLB"] + sheetEntries["MBC"]; checked != want {
		t.Errorf("checked %d sheet cards, want the %d FDN + %d BLB + %d MBC sheet entries", checked, sheetEntries["FDN"], sheetEntries["BLB"], sheetEntries["MBC"])
	}
}

// pendingSets are the embedded sets drafted before Forge has scripted all
// of their cards (the set releases after the pinned corpus). Their sheets
// may name cards the corpus cannot play; the pack generator deals them, the
// deck builders leave them in the sideboard, and
// testdata/<SET>-unplayable.txt lists exactly which, so the list only
// shrinks as scripts land: TestPendingSetCards fails on a card that newly
// breaks and on a stale entry. Rewrite the list with GORGE_UPDATE_PENDING=1.
var pendingSets = map[string]bool{"TRK": true}

// substituteSets are the embedded sets whose sheets draw from a wider pool
// than the engine plays (MBC: Alchemy acorn cards and a few unsupported
// mechanics; its playtest sheet is left out by the generator). Their unplayable cards are
// the embedded sets/<SET>.substitute.txt, which the booster deals a
// playable same-sheet substitute for, every swap recorded.
// TestPendingSetCards holds that file to exactly the cards the corpus
// cannot play, in both directions; GORGE_UPDATE_PENDING=1 rewrites it.
var substituteSets = map[string]bool{"MBC": true}

// unplayableListPath is the file TestPendingSetCards holds to the set's
// unplayable sheet cards: the embedded substitute list of a substitute set,
// the testdata list of a pending one.
func unplayableListPath(code string) string {
	if substituteSets[code] {
		return "sets/" + code + ".substitute.txt"
	}
	return "testdata/" + code + "-unplayable.txt"
}

func TestPendingSetCards(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	if reg.Len() == 0 {
		t.Fatal("corpus registry is empty")
	}
	sup := effects.Supported()
	if !sup["kw:Flying"] {
		t.Fatal("effects.Supported() lacks kw:Flying: the rules package did not register the non-API primitives")
	}
	codes := make([]string, 0, len(pendingSets)+len(substituteSets))
	for code := range pendingSets {
		codes = append(codes, code)
	}
	for code := range substituteSets {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	for _, code := range codes {
		bad := map[string]bool{}
		for _, c := range setdata.Collations(code) {
			spec := mustLoadCollation(t, code, c)
			for _, sh := range spec.Config.Sheets {
				for _, sc := range sh.Cards {
					card, ok := reg.Lookup(sc.Name)
					if !ok || len(reg.Unsupported(card, sup)) > 0 {
						bad[sc.Name] = true
					}
				}
			}
		}
		names := make([]string, 0, len(bad))
		for n := range bad {
			names = append(names, n)
		}
		sort.Strings(names)
		got := strings.Join(names, "\n") + "\n"
		path := unplayableListPath(code)
		if os.Getenv("GORGE_UPDATE_PENDING") != "" {
			if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(want) != got {
			t.Errorf("%s: unplayable sheet cards moved (a script landed, or one broke); rerun with GORGE_UPDATE_PENDING=1 and review the diff", code)
		}
	}
}
