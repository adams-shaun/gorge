package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestXSweepCensusCountsAnAnyColourTapOnce: the X-bound-target sweep's census
// (xBoundPayable's potUnitsAvail record) must count each fixed any-colour
// source's tap once, as gorge's planner prices one mana per tap (CR 118.3c):
// three untapped Birds of Paradise give the sweep's record {ColAny:3}, in the
// sweep's bounded mode as in the multikicker census's unbounded one. The first
// caller to combine units with boundedAny (the sweep, filters_gen.go
// xBoundPayable) re-added the tap after the census branch had already recorded
// it, pricing three taps as six wildcard units, so the over-bound view passed
// {G}{G}{G}{G} off three taps and the kernel offered an X-bound-target cast
// gorge's payment planner refuses off three mana.
func TestXSweepCensusCountsAnAnyColourTapOnce(t *testing.T) {
	tbl, id := tapsDeferShapeTable(t, func(tbl *kop.Table, id map[string]CardID) {})
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	for i := 0; i < 3; i++ {
		sc.perm(id["Birds of Paradise"], 0)
	}
	birds := 0
	for m := sc.s.ctrlM[0]; m.any(); m = m.rest() {
		if sc.s.cardAt(m.first()) == id["Birds of Paradise"] {
			birds++
		}
	}
	if birds != 3 {
		t.Fatalf("precondition: %d untapped any-colour sources on the board, want 3", birds)
	}
	// the sweep's spell-payment descriptor (filters_gen.go xBoundPayable)
	d := payDesc{class: pdSpell, card: id["Judge of Currents"], src: NoSlot, og: restrOrigin{from: kop.ZoneHand}}
	tally := func(pu *[maxPotUnits]potUnit) map[uint8]int {
		t.Helper()
		if pu[0].n == 0 && pu[0].mask != 0 {
			t.Fatal("the census record is incomplete (the overflow marker), want a complete one")
		}
		tal := map[uint8]int{}
		for i := 0; i < maxPotUnits && pu[i].mask != 0; i++ {
			tal[pu[i].mask] += int(pu[i].n)
		}
		return tal
	}
	var pot avail
	var puSweep, puKick [maxPotUnits]potUnit
	sc.s.availPotentialFor(sc.k, 0, &pot, &d, true, &puSweep)
	if got := tally(&puSweep); len(got) != 1 || got[uint8(kop.ColAny)] != 3 {
		t.Fatalf("the sweep's census over three any-colour taps is %v, want {ColAny:3}", got)
	}
	// the multikicker arm's mode (boundedAny=false), the one the census was
	// written for, prices the same three taps at three
	sc.s.availPotentialFor(sc.k, 0, &pot, &d, false, &puKick)
	if got := tally(&puKick); len(got) != 1 || got[uint8(kop.ColAny)] != 3 {
		t.Fatalf("the multikicker census over the same taps is %v, want {ColAny:3}", got)
	}
	// the sweep's over-bound view beside the census prices three wildcards:
	// three coloured pips payable, a fourth not
	var sweep avail
	sc.s.availPotentialFor(sc.k, 0, &sweep, &d, true)
	three, four := cost(0, kop.G, kop.G, kop.G), cost(0, kop.G, kop.G, kop.G, kop.G)
	if !sweep.affords(&three, pipMaskOf(&three), 0) {
		t.Error("the sweep's bound does not reach three pips off three any-colour taps")
	}
	if sweep.affords(&four, pipMaskOf(&four), 0) {
		t.Error("the sweep's bound reaches a fourth pip off three any-colour taps: the census double-recorded")
	}
}
