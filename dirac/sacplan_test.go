package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestSacrificeCostPaidAfterMana: gorge taps a cast's planned sources at
// priority, before any cost is paid, so a creature sacrificed for Arbiter
// of Woe's additional cost still counts for Elvish Archdruid's "{G} for
// each Elf you control" (CR 601.2g-h: mana abilities before costs). With
// Dwynen sacrificed, the Archdruid adds {G}{G} and the four lands pay the
// rest. Before, the kernel sacrificed Dwynen first and panicked "autopay
// found no plan" (pair 9100).
func TestSacrificeCostPaidAfterMana(t *testing.T) {
	deck := []cardset.Entry{{Name: "Arbiter of Woe", Count: 1}, {Name: "Elvish Archdruid", Count: 1}, {Name: "Dwynen, Gilt-Leaf Daen", Count: 1}, {Name: "Swamp", Count: 5}, {Name: "Forest", Count: 5}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	var lands []Obj
	for _, n := range []string{"Swamp", "Swamp", "Forest", "Forest"} {
		lands = append(lands, sc.perm(id[n], 0))
	}
	druid := sc.perm(id["Elvish Archdruid"], 0)
	dwynen := sc.perm(id["Dwynen, Gilt-Leaf Daen"], 0)
	arb := sc.hand(id["Arbiter of Woe"], 0)
	sc.do(t, AkCast, TCode(arb), scAny)
	for i := 0; i < 8 && !(sc.s.objs[arb].zone == kop.ZoneStack && sc.s.ctl == ctlPriority); i++ {
		d := sc.s.Step()
		a := d.Options[0]
		for j := 0; j < int(d.N); j++ {
			if o := d.Options[j]; o.Source() == TCode(uint8(dwynen)) || o.Target() == TCode(uint8(dwynen)) {
				a = o
				break
			}
		}
		sc.s.Apply(a)
	}
	if sc.s.objs[arb].zone != kop.ZoneStack || sc.s.objs[dwynen].zone != kop.ZoneGraveyard {
		t.Fatalf("Arbiter zone %d, Dwynen zone %d: want the stack and the graveyard", sc.s.objs[arb].zone, sc.s.objs[dwynen].zone)
	}
	for _, o := range append(lands, druid) {
		if !sc.tapped(o) {
			t.Fatalf("source %d untapped; want every land and the Archdruid tapped", o)
		}
	}
}

// TestCostObjectsChosenAfterMana: the cast goes on the stack first (CR
// 601.2a) and its cost objects are chosen at payment, after the mana
// abilities (CR 601.2g-h; gorge costObjectAsks after manaWindowAsk):
// when Arbiter of Woe's sacrifice is asked, the spell is already on the
// stack and the planner's sources are already tapped into the pool.
func TestCostObjectsChosenAfterMana(t *testing.T) {
	deck := []cardset.Entry{{Name: "Arbiter of Woe", Count: 1}, {Name: "Elvish Archdruid", Count: 1}, {Name: "Dwynen, Gilt-Leaf Daen", Count: 1}, {Name: "Swamp", Count: 5}, {Name: "Forest", Count: 5}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	var lands []Obj
	for _, n := range []string{"Swamp", "Swamp", "Forest", "Forest"} {
		lands = append(lands, sc.perm(id[n], 0))
	}
	druid := sc.perm(id["Elvish Archdruid"], 0)
	sc.perm(id["Dwynen, Gilt-Leaf Daen"], 0)
	arb := sc.hand(id["Arbiter of Woe"], 0)
	sc.do(t, AkCast, TCode(arb), scAny)
	for i := 0; i < 8; i++ {
		d := sc.s.Step()
		if d.Kind == DkChoose && d.Sub == ChCostSacrifice {
			if sc.s.objs[arb].zone != kop.ZoneStack {
				t.Fatalf("Arbiter zone %d at the sacrifice ask, want the stack", sc.s.objs[arb].zone)
			}
			for _, o := range append(lands, druid) {
				if !sc.tapped(o) {
					t.Fatalf("source %d untapped at the sacrifice ask; the mana abilities come first", o)
				}
			}
			return
		}
		sc.s.Apply(d.Options[0])
	}
	t.Fatal("no sacrifice ask")
}
