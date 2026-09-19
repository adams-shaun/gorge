package effects

import (
	"testing"
)

// The Count$CastTotalManaSpent branch head (task castprov1): the TOTAL mana
// actually spent to cast the resolving spell, carried by the pay-time
// CastInfo's FlagManaSpent Amount (rules/cast.go's payCast capture -- the
// converge/replicate/multikick pattern). Unit-tested at the eval level the
// Count$Converge / Count$wasCastFromYourHandByYou heads use; the provenance
// itself is pinned end to end on the real engine in rules (the Freestrider
// Commando corpus tests).

func TestCastTotalManaSpentHeadReadsTheCapturedSpend(t *testing.T) {
	h, c := fixtureHost(t)
	// A cheated-in permanent (no CastInfo ever stamped): 0.
	if got := EvalCount(h, c, "Count$CastTotalManaSpent"); got != 0 {
		t.Errorf("uncast CastTotalManaSpent = %d, want 0", got)
	}
	// The pay-time capture: four mana spent to cast the source.
	h.g.Obj(c.Source).ManaSpent = 4
	if got := EvalCount(h, c, "Count$CastTotalManaSpent"); got != 4 {
		t.Errorf("cast CastTotalManaSpent = %d, want 4", got)
	}
	// A zero is a real zero (a convoke-only cast), not an absent one.
	h.g.Obj(c.Source).ManaSpent = 0
	h.g.Obj(c.Source).CastFlags = 1 << 20 // some provenance, not the absence of a cast
	if got := EvalCount(h, c, "Count$CastTotalManaSpent"); got != 0 {
		t.Errorf("zero-spend CastTotalManaSpent = %d, want 0", got)
	}
	// A source object that is gone: 0.
	c2 := &Ctx{Source: 999, Controller: 0}
	if got := EvalCount(h, c2, "Count$CastTotalManaSpent"); got != 0 {
		t.Errorf("absent-source CastTotalManaSpent = %d, want 0", got)
	}
}
