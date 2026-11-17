package dirac

import "testing"

// TestWindowPaidLifePainlandCountsItsColour: gorge's unless/ward window
// appends a paid-life activation's mana (paid_mana_census.go
// appendPaidManaUnits), so a painland whose second ability is "{T}, Pay 1 life:
// Add <colour>" funds the cost in its paid colour too. The kernel's window
// mask listed a non-pain-only painland by its normal ability alone, so an
// unless {G}{G} that only Hashep Oasis's paid life could complete was offered
// as the decline alone. The paid alternative still keeps gorge's LifeKeep
// guard: at 1 life it would take the payer to 0, so only the normal ability
// counts.
func TestWindowPaidLifePainlandCountsItsColour(t *testing.T) {
	sc, id, _, _ := paidLifeScen(t)
	town := sc.perm(id("Starting Town"), 0)
	pc := sc.s.manaPCAt(sc.k, sc.slot(town))
	if !pc.painLife || pc.painOnly || pc.painCol == 0 || pc.manaCol == 0 {
		t.Fatalf("precondition: Starting Town is not a paid-life painland: %+v", *pc)
	}

	sc.s.pl[0].life = 5
	want := uint8(pc.manaCol)&0x3f | uint8(pc.painCol)&0x3f
	if got := sc.windowMask(town); got != want {
		t.Errorf("window mask %#x, want %#x (the normal ability and the paid-life colours)", got, want)
	}

	sc.s.pl[0].life = 1
	if got := sc.windowMask(town); got != uint8(pc.manaCol)&0x3f {
		t.Errorf("window mask %#x at 1 life, want %#x (the paid alternative must leave the payer alive)", got, uint8(pc.manaCol)&0x3f)
	}
}
