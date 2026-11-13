package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A deferred mana source (Pyromancer's Goggles) beside other census gaps
// (mana_mixgaps.go mixDeferred, mana_filterfirst.go filtOtherMana,
// script_gen.go scriptDeadEnd). gorge's one prefix search activates every
// census gap in battlefield order (CR 601.2g-h, 605.3a), so a play the gaps
// together pay is offered.

func deferredGapScen(t *testing.T, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	set := cardsettest.Set(t, "deferredgap")
	var deck []cardset.Entry
	for _, nm := range append([]string{"Plains"}, names...) {
		if i, ok := set.Lookup(nm); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the deferredgap table", nm)
		}
		deck = append(deck, cardset.Entry{Name: nm, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	return newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true}), id
}

func deferGapOffered(d *Decision, kind ActionKind, o Obj) bool {
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == kind && a.Source() == TCode(uint8(o)) {
			return true
		}
	}
	return false
}

// TestDeferredGapBesideFilterOffersPlay: a Plains, Three Tree Mascot and
// Pyromancer's Goggles, no blue source: one of the Plains' {W} and the
// Goggles' {R} pays the Mascot's {1} for {U}, the other Elementalist Adept's
// generic, so its {1}{U} cast is offered. Then the Goggles alone beside the
// Mascot: its {R} funds the Mascot's fee, so the Mascot is a filter gap.
func TestDeferredGapBesideFilterOffersPlay(t *testing.T) {
	sc, id := deferredGapScen(t, "Pyromancer's Goggles", "Three Tree Mascot", "Elementalist Adept")
	gog := sc.perm(id["Pyromancer's Goggles"], 0)
	mascot := sc.perm(id["Three Tree Mascot"], 0)
	if !sc.k.gen.has(id["Pyromancer's Goggles"], gfDeferred) || !sc.k.gen.has(id["Three Tree Mascot"], gfFilt) {
		t.Fatal("want the Goggles a deferred source and the Mascot a filter")
	}
	if got := sc.s.scriptFilt(sc.k, 0); !got.has(sc.slot(mascot)) {
		t.Fatalf("filter gaps %v: the Goggles' {R} alone funds the Mascot's {1}", got)
	}
	if !sc.s.scriptDeferred(sc.k, 0).has(sc.slot(gog)) || !sc.s.mixBoard(sc.k, 0) {
		t.Fatal("want the Goggles a live deferred gap and the board a mixed-gap board")
	}
	sc.perm(id["Plains"], 0)
	adept := sc.hand(id["Elementalist Adept"], 0)
	if d := sc.s.Step(); !deferGapOffered(&d, AkCast, adept) {
		t.Fatalf("Elementalist Adept not offered: %v", d.Options[:d.N])
	}
}

// TestCounterGapBesideDeferredOffersPlay: Ramos, Dragon Engine with five
// +1/+1 counters beside Pyromancer's Goggles: removing them adds
// {W}{W}{U}{U}{B}{B}{R}{R}{G}{G}, which pays Zetalpa, Primal Dawn's {6}{W}{W}.
// The deferred script's dead-end check activates the counter gap as the
// prefix does, so the cast is offered.
func TestCounterGapBesideDeferredOffersPlay(t *testing.T) {
	sc, id := deferredGapScen(t, "Pyromancer's Goggles", "Ramos, Dragon Engine", "Zetalpa, Primal Dawn")
	sc.perm(id["Pyromancer's Goggles"], 0)
	ramos := sc.perm(id["Ramos, Dragon Engine"], 0)
	if !sc.s.perms[sc.slot(ramos)].setCtrN(sc.s, int(kop.CtrP1P1), 5) {
		t.Fatal("cannot put five counters on Ramos")
	}
	if !sc.s.scriptCtr(sc.k, 0).has(sc.slot(ramos)) || !sc.s.scriptRestr(sc.k, 0).any() {
		t.Fatal("want Ramos a counter-cost gap beside the Goggles' deferred gap")
	}
	zet := sc.hand(id["Zetalpa, Primal Dawn"], 0)
	if d := sc.s.Step(); !deferGapOffered(&d, AkCast, zet) {
		t.Fatalf("Zetalpa not offered: %v", d.Options[:d.N])
	}
}
