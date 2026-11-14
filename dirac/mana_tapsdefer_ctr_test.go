package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTapsDeferRiderBesideCtrGapCastOffered: a Taps-deferred rider source
// (Peat Bog: "{T}, Remove a depletion counter from Peat Bog: Add {B}{B}. If
// there are no depletion counters on Peat Bog, sacrifice it.") beside a
// counter-cost gap (Wall of Roots' {G}) keeps the scriptDeadEndAny route,
// whose search covers both: a play the rider's mana funds is offered (CR
// 605.3b). scriptOffers' tapX branch and tapXPrefix used to swallow the
// board -- the tapX search's gap set has no rider step -- and the kernel
// listed no cast where gorge's prefix search, which visits every census gap
// in battlefield order, listed both (dr-mbc-bb pair 78). Two untapped
// Plains, the Wall's {G} and Peat Bog's {B}{B} pay {3}{G}; a third Plains
// pays {2}{W}{W} beside them.
func TestTapsDeferRiderBesideCtrGapCastOffered(t *testing.T) {
	set := cardsettest.Set(t, "mbc")
	deck := []cardset.Entry{{Name: "Wall of Roots", Count: 1}, {Name: "Irregular Cohort", Count: 1},
		{Name: "Drumhunter", Count: 1}, {Name: "Inspiring Overseer", Count: 1},
		{Name: "Plains", Count: 14}, {Name: "Forest", Count: 4}, {Name: "Peat Bog", Count: 1}}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	wall := sc.perm(id["Wall of Roots"], 0) // it resolves this turn: summoning sick, its {G} usable (CR 302.6)
	sc.s.perms[sc.slot(wall)].flags |= pfSick
	p28 := sc.perm(id["Plains"], 0)
	sc.s.perms[sc.slot(p28)].flags |= pfTapped
	sc.perm(id["Plains"], 0)
	sc.perm(id["Plains"], 0)
	pb := sc.perm(id["Peat Bog"], 0) // its enters program put it down tapped with two depletion counters
	sc.s.perms[sc.slot(pb)].flags &^= pfTapped
	f12 := sc.perm(id["Forest"], 0)
	sc.s.perms[sc.slot(f12)].flags |= pfTapped
	sc.perm(id["Inspiring Overseer"], 0)
	coh := sc.hand(id["Irregular Cohort"], 0)
	dru := sc.hand(id["Drumhunter"], 0)

	// Preconditions: the Bog is the live Taps-deferred rider gap and the Wall
	// the live counter-cost one, so the assertion below is about their route
	// and not about a plain census that would price the casts without either.
	if pbSl := sc.slot(pb); !sc.s.scriptTapsDefer(sc.k, 0).has(pbSl) {
		t.Fatal("precondition: Peat Bog is not a live Taps-deferred rider gap")
	}
	if n := sc.s.perms[sc.slot(pb)].ctrN(sc.s, int(kop.CtrDepletion)); n != 2 {
		t.Fatalf("precondition: Peat Bog holds %d depletion counters, want 2", n)
	}
	if wallSl := sc.slot(wall); !sc.s.scriptCtr(sc.k, 0).has(wallSl) {
		t.Fatal("precondition: Wall of Roots is not a live counter-cost gap")
	}

	d := sc.s.Step()
	var cohCast, druCast bool
	for _, a := range d.Options[:d.N] {
		if a.Kind() != AkCast {
			continue
		}
		cohCast = cohCast || a.Source() == TCode(coh)
		druCast = druCast || a.Source() == TCode(dru)
	}
	if !cohCast {
		t.Errorf("the {2}{W}{W} cast is not offered: %v", d.Options[:d.N])
	}
	if !druCast {
		t.Errorf("the {3}{G} cast is not offered: %v", d.Options[:d.N])
	}
}
