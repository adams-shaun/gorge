package oracle

import (
	"testing"
)

// identityPotentialOptionsAt is the decision the recording diverged at before
// the kernel's census held a Produced$ ColorIdentity source with a rider.
const identityPotentialOptionsAt = 168

// TestIdentityPotentialOptionsLockstep: Path of Ancestry ("Add one mana of any
// color in your commander's color identity", scry 1 when spent on a creature
// sharing a type with the commander) funds Tegwyll and Faerie Bladecrafter at
// decision 168. gorge prices the source in the potential fold at the payer's
// identity (CR 903.4) and offers both casts; the kernel's censuses held no
// Path mana and offered only the land drops.
func TestIdentityPotentialOptionsLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	res := replayStdReproResult(t, synthReg(t), "custom", "identity_potential_options", true)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if res.Decisions <= identityPotentialOptionsAt {
		t.Fatalf("%d decisions, the game must go past decision %d where Path's mana is priced", res.Decisions, identityPotentialOptionsAt)
	}
}
