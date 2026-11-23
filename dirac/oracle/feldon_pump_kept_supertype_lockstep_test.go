package oracle

import "testing"

// Feldon of the Third Path's copy "gains haste" (PumpKeywords$ Haste, CR
// 707.9b: a grant, not a copy exception). kgen keeps a printed unmodelled
// supertype on every override record since #3188, so in a pod whose table
// gives such a supertype a slot the keyword-only record carries a kept slot
// and copyPumpEnter found no record: a StallLowering at decision ~3127 of
// this 4-seat pod (kfuzz -kernel edh -seats 4 -seed 2, pair 52). The record
// match ignores the kept slot (83a0a7b951).
//
// The recording is the four full 100-card precons; the game must play on.
func TestFeldonPumpHasteBesideKeptSupertypeLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg := synthReg(t)
	const name = "feldon_pump_haste_beside_kept_supertype_pod52"
	res := gimbalStatuaryReplay(t, reg, name)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if res.StallUnmodelled || res.Stall != "" {
		t.Fatalf("stall %q (unmodelled=%v) after %d decisions", res.Stall, res.StallUnmodelled, res.Decisions)
	}
	if res.Decisions < 3127 {
		t.Errorf("%d decisions, the game must pass the Feldon copy at 3127", res.Decisions)
	}
}
