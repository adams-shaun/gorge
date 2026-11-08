package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestWindowOwedCostCountsAdmittedRestrictedMana: a window source's colour ask
// prices the pool AvailableFor holds, which includes the floating restricted
// mana the payment may spend (CR 106.6). The Heirloom's restricted {U} covers
// the {U} pip, so only {R} stays owed; a raw pool read would ask for {U} first
// and strand a hybrid cast (Head of the Homestead beside Helga, Skittish Seer).
func TestWindowOwedCostCountsAdmittedRestrictedMana(t *testing.T) {
	sc, _, _ := restrDemandScen(t)
	s := sc.s
	cost := kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.U: 1, kop.R: 1}}
	var owed kop.ManaCost
	got := s.windowOwedCost(sc.k, 0, &cost, &owed)
	if got.Pips[kop.U] != 0 {
		t.Fatalf("owed {U} pips %d, want 0: the admitted restricted {U} covers it", got.Pips[kop.U])
	}
	if got.Pips[kop.R] != 1 {
		t.Fatalf("owed {R} pips %d, want 1: the restricted {U} pays no {R}", got.Pips[kop.R])
	}
}
