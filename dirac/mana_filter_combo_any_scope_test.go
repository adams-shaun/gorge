package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The repro's board (std-msh pair 166: Super-Skrull, Baxter Building's
// "{4}, {T}: Add four mana in any combination of colors", Pym Technologies,
// three Plains and a Swamp) holds one multi-unit fee gap, but that is a board
// property: gorge builds its feeOrder phased run only for a plain cast
// (engine/rules/potential_script.go build), and phasedOutOfScope leaves every
// other play kind to the plain order unless the plain run was limited on a
// repeatable fee-only filter. The feeOrder arm (feeOrderPlay) must therefore stay a plain
// cast's: an activation or a kicked cast on the same board keeps the plain
// answer, exactly as gorge's phased run does. The {4}{R} activation is still
// offered here -- the plain walk prices it -- so the control shows the gate
// withholds no play the plain order finds.
func TestComboAnyFilterPhasedFallbackStaysAPlainCast(t *testing.T) {
	sc, id := stdMshScen(t, "Super-Skrull", "Baxter Building", "Pym Technologies", "Plains", "Swamp")
	for i := 0; i < 3; i++ {
		sc.perm(id["Plains"], 0)
	}
	sc.perm(id["Swamp"], 0)
	sc.untappedPerm(id["Pym Technologies"], 0)
	baxter := sc.untappedPerm(id["Baxter Building"], 0)
	skrull := sc.perm(id["Super-Skrull"], 0)
	skrullHand := sc.hand(id["Super-Skrull"], 0) // the copy the cast repro prices, so the board is the repro's

	// Preconditions: Baxter is the board's only filter and a multi-unit fee
	// gap, and the board holds no repeatable fee-only filter (so the feeGap
	// arm, not plainLimited, is what could open the phased run).
	filt := sc.s.scriptFilt(sc.k, 0)
	if !filt.has(sc.slot(baxter)) || !sc.s.filtFeeGaps(sc.k, filt).has(sc.slot(baxter)) {
		t.Fatal("precondition: Baxter Building's {4},{T} Combo Any is not the board's multi-unit fee gap")
	}
	if sc.s.filtRepeatIn(sc.k, filt) {
		t.Fatal("precondition: the board holds a repeatable fee-only filter")
	}
	cd := &sc.k.t.Cards[id["Super-Skrull"]]
	dmg := -1
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind == kop.AbActivated && a.Cost.Mana.Generic == 4 && a.Cost.Mana.Pips[kop.R] == 1 {
			dmg = i
			break
		}
	}
	if dmg < 0 {
		t.Fatal("precondition: Super-Skrull has no {4}{R} activated ability")
	}

	// End to end: the feeOrder cast and the activation are both offered on the
	// board. The activation's offer is the plain walk's (its plain-order
	// funding prices it), so closing the phased fallback must not withhold it.
	d := sc.s.Step()
	cast, activation := false, false
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		switch {
		case a.Kind() == AkCast && a.Source() == TCode(uint8(skrullHand)):
			cast = true
		case a.Kind() == AkActivate && a.Source() == TCode(uint8(skrull)) && a.Mode() == uint8(dmg):
			activation = true
		}
	}
	if !cast {
		t.Fatal("precondition: the feeOrder plain cast is not offered")
	}
	if !activation {
		t.Fatal("precondition: the {4}{R} activation is not offered by the plain walk")
	}

	// The gate itself: the feeOrder plain cast opens the phased fallback, and
	// the non-feeOrder shapes on the same fee-gap board do not.
	feeGaps := sc.s.filtFeeGaps(sc.k, filt)
	fallback := func(a Action) bool {
		return sc.s.feeOrderPlay(sc.k, a, feeGaps, filt)
	}
	if !fallback(MakeAction(AkCast, TCode(uint8(skrullHand)), 0, 0)) {
		t.Fatal("the feeOrder plain cast did not open the phased fallback")
	}
	if fallback(MakeAction(AkActivate, TCode(uint8(skrull)), 0, uint8(dmg))) {
		t.Fatal("the phased fallback opened for an activation on the multi-unit fee-gap board")
	}
	if fallback(MakeAction(AkCast, TCode(uint8(skrullHand)), 0, 1)) {
		t.Fatal("the phased fallback opened for a kicked cast on the multi-unit fee-gap board")
	}
	// The plainLimited arm is not narrowed: a plain run limited on a repeatable
	// fee-only filter is in scope for every play kind (phasedOutOfScope).
	fs := filtSearch{a: MakeAction(AkActivate, TCode(uint8(skrull)), 0, uint8(dmg)), feeOnly: true, nodes: potentialScriptNodes}
	if !fs.phasedFallback() {
		t.Fatal("the plainLimited arm no longer opens for an activation")
	}
}
