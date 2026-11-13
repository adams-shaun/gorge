package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSubsGenMatchesHasSubGen: the projection's batch query (subsGen, one
// pass per slot) answers the same as the single-subtype query it replaced
// (hasSubGen) over every matchup subtype and the outside type, on slots
// with subtype-affecting permanents: printed subtypes, a self static grant
// (Skyknight Squire's Knight), a subtype-setting static that removes
// abilities (Witness Protection), a printed changeling, an
// all-creature-types animation (Soulstone Sanctuary, an override record)
// and an impending-dormant permanent.
func TestSubsGenMatchesHasSubGen(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Skyknight Squire", Count: 1}, {Name: "Witness Protection", Count: 1},
		{Name: "Soulstone Sanctuary", Count: 1}, {Name: "Plains", Count: 12},
	}
	tbl, da, _, err := cardsettest.Set(t, "fdn").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	knight := slices.Index(tbl.Subtypes, "Knight")
	citizen := slices.Index(tbl.Subtypes, "Citizen")
	if knight < 0 || citizen < 0 {
		t.Fatalf("matchup subtypes %v lack Knight or Citizen", tbl.Subtypes)
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	sq := sc.perm(id["Skyknight Squire"], 0)
	wp := sc.perm(id["Witness Protection"], 1)
	ss := sc.perm(id["Soulstone Sanctuary"], 0)
	for i := 0; i < 4; i++ {
		sc.perm(id["Plains"], 0) // the animate's {4}
	}

	// check pins subsGen against hasSubGen over every matchup subtype and
	// the outside type, and returns the mask for the precondition asserts.
	check := func(label string, o Obj) kop.SubMask {
		t.Helper()
		sl := sc.slot(o)
		subs, all := sc.s.subsGen(sc.k, sl)
		if all != sc.s.hasSubGen(sc.k, sl, subOutside) {
			t.Errorf("%s: subsGen outside %v, hasSubGen %v", label, all, sc.s.hasSubGen(sc.k, sl, subOutside))
		}
		for i := 0; i < len(sc.k.t.Subtypes) && i < kop.MaxSubtypes; i++ {
			if want := sc.s.hasSubGen(sc.k, sl, uint8(i)); subs.Has(i) != want {
				t.Errorf("%s: subtype %d (%s): subsGen %v, hasSubGen %v", label, i, sc.k.t.Subtypes[i], subs.Has(i), want)
			}
		}
		return subs
	}

	sqs := sc.slot(sq)
	check("printed Squire", sq)
	sss := sc.slot(ss)
	if subs := check("unanimated Sanctuary", ss); subs.Any() {
		t.Fatalf("precondition: the unanimated Sanctuary has subtypes %v", subs)
	}

	// the Squire's own static (three or more +1/+1 counters) grants Knight
	sc.s.placeCounters(sc.k, sqs, int(kop.CtrP1P1), 3, 0, ctrPutEffect)
	sc.s.refreshTy(sc.k)
	check("Squire static grant", sq)
	if !check("Squire static grant", sq).Has(knight) {
		t.Fatal("precondition: three counters did not grant Knight")
	}

	// the subtype-setting static (a lose-all Aura): a Citizen, no Knight, no Cat
	sc.s.attachTo(sc.k, sc.slot(wp), sqs)
	sc.s.refreshTy(sc.k)
	check("Witness Protection", sq)
	if !check("Witness Protection", sq).Has(citizen) || check("Witness Protection", sq).Has(knight) {
		t.Fatal("precondition: Witness Protection did not set a Citizen over the Squire's types")
	}

	// a printed changeling (CardDef.AllCreatureTypes): the outside type
	sc.k.t.Cards[id["Soulstone Sanctuary"]].AllCreatureTypes = true
	if !sc.s.hasSubGen(sc.k, sss, subOutside) {
		t.Fatal("precondition: the mutated Sanctuary is not a printed changeling")
	}
	check("printed changeling", ss)

	// the all-creature-types animation: an override record on the Sanctuary
	// (TestAllCreatureTypesHasOutsideType's scripted activation: push it,
	// then pass until it resolves)
	pushed := false
	sc.stepTo(t, 40, func() bool { return pushed && sc.s.sp == 0 }, func(d *Decision) (Action, bool) {
		pushed = pushed || sc.s.sp > 0
		for _, a := range d.Options[:d.N] {
			if !pushed && a.Kind() == AkActivate && a.Source() == uint8(ss) {
				return a, true // the tap resolves without the stack; the animate pushes
			}
		}
		for _, a := range d.Options[:d.N] {
			if a.Kind() == AkPass {
				return a, true
			}
		}
		return 0, false
	})
	if !pushed || !sc.s.isCreature(sc.k, sss) {
		t.Fatalf("the Sanctuary was not animated (pushed %v, types %v, sp %d, step %v)",
			pushed, sc.s.typesAt(sc.k, sss), sc.s.sp, sc.s.step)
	}
	if !sc.s.hasSubGen(sc.k, sss, subOutside) {
		t.Fatal("precondition: the animated Sanctuary has no outside creature type")
	}
	check("animated Sanctuary", ss)

	// an impending-dormant permanent: the creature subtypes and the outside
	// type go with the creature type (CR 205.1a); the non-dormant
	// Sanctuary's outside type stays
	sc.k.gen.anyImpending = true
	sc.k.t.Cards[id["Skyknight Squire"]].Impending = 1
	sc.s.placeCounters(sc.k, sqs, int(kop.CtrTime), 1, 0, ctrPutEffect)
	sc.s.refreshTy(sc.k)
	if sc.s.hasSubGen(sc.k, sqs, uint8(citizen)) {
		t.Fatal("precondition: the dormant Squire keeps Citizen")
	}
	check("dormant Squire", sq)
	if !sc.s.hasSubGen(sc.k, sss, subOutside) {
		t.Fatal("precondition: the Sanctuary lost its outside type without being dormant")
	}
	sc.k.t.Cards[id["Soulstone Sanctuary"]].Impending = 1
	sc.s.placeCounters(sc.k, sss, int(kop.CtrTime), 1, 0, ctrPutEffect)
	sc.s.refreshTy(sc.k)
	if sc.s.hasSubGen(sc.k, sss, subOutside) {
		t.Fatal("precondition: the dormant animated Sanctuary keeps its outside type")
	}
	check("dormant animated Sanctuary", ss)
}
