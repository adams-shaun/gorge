package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestWindowDryFilterFee: the dry window's first-only filter fee is spent as
// the real window spends it (filtDo -> filtFeeSpend: the plain pool in C W U B
// R G order), so the dry probe agrees with the walk that actually pays.
//
// The board is the one the 480-game edh scan reached at pods[3] seed 2: a
// Spitting Image ({4}{G/U}{G/U}) with both pips announced U costs {4}{U}{U},
// the pool floats {U}{B}, and the untapped sources include a Golgari Signet
// ({1}, {T}: Add {B}{G}), an Opulent Palace, a Swamp and a Plains. The Signet
// is first in the window's entry order, so the real walk activates it before
// any white mana floats; filtFeeSpend spends the pool's only U on the {1} fee,
// leaving one U short of the two pips. The dry probe used to spare the pip
// (spending the B instead) while the real walk spent the U.
//
// The face itself stays listed, as gorge lists it: its face gate does not
// charge a filter's fee, and a face it lists that the payment strands is
// reversed (CR 733.1) by both engines (TestHybridFaceThroughFilterLockstep).
// The kernel's job here is only that the dry probe stops promising a payment
// the real walk cannot make, so tapForHyb falls through to the reversal
// instead of the old autopay panic.
func TestWindowDryFilterFee(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Spitting Image", Count: 1},
		{Name: "Golgari Signet", Count: 1},
		{Name: "Opulent Palace", Count: 1},
		{Name: "Swamp", Count: 1},
		{Name: "Plains", Count: 1},
		{Name: "Island", Count: 8},
	}
	set := cardsettest.Set(t, "edh")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
	// Entry order is the window's battlefield order: the Signet must come
	// before any source that makes white, or the fee would spend the W.
	sc.perm(id["Golgari Signet"], 0)
	opulent := sc.perm(id["Opulent Palace"], 0)
	sc.s.perms[sc.slot(opulent)].flags &^= pfTapped // it enters tapped
	sc.perm(id["Swamp"], 0)
	sc.perm(id["Plains"], 0)
	sc.s.pl[0].pool = [kop.NumManaKinds]uint8{kop.U: 1, kop.B: 1}
	spell := sc.hand(id["Spitting Image"], 0)
	k := sc.s.k()

	// Start the cast's build directly (the priority offer's potential-play
	// prefix would tap a source before the pips; the window's own read is
	// what this test prices).
	sc.s.beginCast(k, spell, 0, 0)

	// The first pip answered U leaves the later pip G or U payable, so U is
	// a legal first face and must stay offered.
	var first Decision
	if !sc.s.pipOptions(k, &first) {
		t.Fatalf("first pip: no hybrid pip ask")
	}
	if !optionFor(first, kop.U) {
		t.Fatalf("first pip: U face not offered: %v", optionModes(first))
	}
	sc.s.bld().setPipCol(0, kop.U)
	sc.s.bld().pipN = 1

	// Both pips answered U make the cost {4}{U}{U}: the real window cannot
	// pay it. The dry probe must agree, and the face stays listed as gorge
	// lists it (the reversal is the payment's, CR 733.1).
	var second Decision
	if !sc.s.pipOptions(k, &second) {
		t.Fatalf("second pip: no hybrid pip ask")
	}
	cost := sc.s.buildCost(k).Resolve(kop.U).AtX(int(sc.s.bld().e.x))
	dry, known := sc.s.windowDry(k, 0, &cost, false)
	if !known {
		t.Fatalf("both-U cost %+v: the dry walk stopped unknown, want a priced walk", cost)
	}
	probe := sc.s.fork()
	real, _ := probe.windowTapRider(k, 0, &cost, true, false)
	sc.s.unfork()
	if dry || real {
		t.Fatalf("both-U cost %+v: dry=%v known=%v real=%v, want both false", cost, dry, known, real)
	}
	if dry != real {
		t.Fatalf("both-U cost %+v: dry=%v real=%v, want the dry probe to match the real walk", cost, dry, real)
	}
	if !optionFor(second, kop.U) {
		t.Fatalf("second pip dropped U: gorge's face gate lists it (TestHybridFaceThroughFilterLockstep): %v", optionModes(second))
	}
	if !optionFor(second, kop.G) {
		t.Fatalf("second pip: G face not offered: %v", optionModes(second))
	}
}

// optionFor reports whether d lists mode as a choice.
func optionFor(d Decision, mode uint8) bool {
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Mode() == mode {
			return true
		}
	}
	return false
}

// optionModes lists d's choice modes.
func optionModes(d Decision) []uint8 {
	out := make([]uint8, 0, d.N)
	for i := 0; i < int(d.N); i++ {
		out = append(out, d.Options[i].Mode())
	}
	return out
}
