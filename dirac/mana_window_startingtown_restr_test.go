package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// emperorRestrScen: std-fin, p0 has Starting Town, Sharlayan, Nation of Scholars
// ({W} or {U}) and The Emperor of Palamecia, whose {U} floats restricted to a
// noncreature spell; a cast of Relm's Sketching ({2}{U}{U}) is being built, so the
// payment may spend it (CR 106.6). The pool is empty.
func emperorRestrScen(t *testing.T) (sc *scen, town, sharlayan Slot) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Starting Town", Count: 1}, {Name: "Sharlayan, Nation of Scholars", Count: 1},
		{Name: "The Emperor of Palamecia", Count: 1}, {Name: "Relm's Sketching", Count: 1}, {Name: "Island", Count: 16}}
	set := cardsettest.Synth(t, "std-fin")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Fatalf("%s not supported in std-fin", e.Name)
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
	sc = newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	town = sc.slot(sc.perm(id["Starting Town"], 0))
	sharlayan = sc.slot(sc.perm(id["Sharlayan, Nation of Scholars"], 0))
	emp := sc.slot(sc.perm(id["The Emperor of Palamecia"], 0))
	c := s.cardAt(emp)
	if !s.floatRestrAb(sc.k, 0, emp, &sc.k.t.Cards[c].Abilities[sc.k.gen.restr(c)], int(kop.U), 1) {
		t.Fatal("precondition: the Emperor's {U} did not float restricted")
	}
	*s.bld() = build{e: stackEntry{kind: seSpell, card: id["Relm's Sketching"], ctrl: 0, obj: s.perms[emp].obj}}
	s.ctl = ctlBuild
	if s.restrPoolAdmits(sc.k, 0) == 0 || s.restrOf(0, int(kop.U)) != 1 {
		t.Fatalf("precondition: restricted %v not admitted for Relm's Sketching", s.rb[0])
	}
	if s.pl[0].pool != ([kop.NumManaKinds]uint8{}) {
		t.Fatalf("precondition: pool %v, want empty", s.pl[0].pool)
	}
	return sc, town, sharlayan
}

// TestStartingTownWindowCountsRestrictedManaOnce: Starting Town's window answer
// (windowCol's painland arm) prices the floating restricted mana the payment may
// spend exactly once (CR 106.6). With the pool {U}{W} and {2}{U}{U} owed, the
// plain {U} and the restricted {U} cover both pips and the pool is one generic
// short, so Starting Town answers {C}. Counting the restricted {U} twice (once in
// the owed cost, again in windowUseful's pool) left no owed colour with the clamp
// decided, and the answer was index 8, past the pool.
func TestStartingTownWindowCountsRestrictedManaOnce(t *testing.T) {
	sc, town, _ := emperorRestrScen(t)
	s := sc.s
	if !sc.k.painMana {
		t.Fatal("precondition: Starting Town's painful ability is not modelled (k.painMana)")
	}
	s.pl[0].pool = [kop.NumManaKinds]uint8{kop.U: 1, kop.W: 1}
	cost := kop.ManaCost{Generic: 2, Pips: [kop.NumManaKinds]uint8{kop.U: 2}}
	pc := s.windowGrantPC(sc.k, town)
	if pc.painCol != kop.ColAny {
		t.Fatalf("precondition: Starting Town's painCol %v, want any colour", pc.painCol)
	}
	if col := s.windowCol(sc.k, 0, &cost, town, pc, false, false); col != kop.C {
		t.Fatalf("Starting Town answers %d, want {C} (%d)", col, kop.C)
	}
}

// TestTapXWindowColourCountsRestrictedMana: a tap-another window source's colour
// (windowTapXCol, as windowDry's) measures the owed colours against the cost less
// the floating restricted mana, like the paying window's (CR 106.6). The
// restricted {U} covers {U}, so nothing is owed and Sharlayan answers its lowest
// colour {W}; a raw-cost read would answer {U}.
func TestTapXWindowColourCountsRestrictedMana(t *testing.T) {
	sc, _, sharlayan := emperorRestrScen(t)
	cost := kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.U: 1}}
	if col := sc.s.windowTapXCol(sc.k, 0, &cost, sharlayan, false, false); col != kop.W {
		t.Fatalf("Sharlayan answers %d, want {W} (%d)", col, kop.W)
	}
	if col := sc.s.windowTapXCol(sc.k, 0, &kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.U: 2}}, sharlayan, false, false); col != kop.U {
		t.Fatalf("precondition: with {U}{U} owed Sharlayan answers %d, want {U} (%d)", col, kop.U)
	}
}
