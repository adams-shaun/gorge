package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestFilterFeeKeepsOneColourSourceConsistent: a filter's fee taken out of
// the groups must not leave a one-colour source recorded at its full amount
// while its group has lost a unit. A source making three mana of one colour
// plus Evendo's filter (fee paid from that source) cannot pay {U}{R}: the two
// units left are one colour. A board of three independent any-colour units can
// (the fee takes one, the other two pay the pips).
func TestFilterFeeKeepsOneColourSourceConsistent(t *testing.T) {
	sc, evendo, _, _ := evendoScen(t, 12, -1)
	sl := sc.slot(evendo)
	if fee := sc.s.filtFee(sc.k, sl); fee != 1 {
		t.Fatalf("Evendo's filter fee %d, want 1", fee)
	}
	var cost kop.ManaCost
	cost.Pips[kop.U], cost.Pips[kop.R] = 1, 1
	filt := [maxFaceFilters]Slot{sl}

	var one avail
	one.addUnits(uint8(kop.ColAny), 3)
	one.one[0], one.nOne = 3, 1
	if one.affords(&cost, pipMaskOf(&cost), 0) {
		t.Fatal("a three-mana one-colour source pays {U}{R} before any filter")
	}
	if sc.s.faceReachSet(sc.k, 0, &cost, 0, one, &filt, 0, 1, 1) {
		t.Error("one-colour source + filter fee: {U}{R} reported payable, the source's remaining two mana are one colour")
	}

	var indep avail
	indep.addUnits(uint8(kop.ColAny), 3)
	if !sc.s.faceReachSet(sc.k, 0, &cost, 0, indep, &filt, 0, 1, 1) {
		t.Error("three independent any-colour units + filter fee: {U}{R} reported unpayable")
	}
}
