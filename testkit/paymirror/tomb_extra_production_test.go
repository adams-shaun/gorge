package paymirror

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestWitnessReadsAncientTombDamageBeforeTapExtra pins the production window
// over a pain land whose own damage rider sits before a TapsForMana extra's
// mana. Ancient Tomb's "{T}: Add {C}{C}. Ancient Tomb deals 2 damage to you"
// under Forsaken Monument makes {C}{C}{C}, and the engine emits the extra
// ManaAdd after the damage. The witness must read the whole activation, not
// stop at the damage (paymirror seed 4038 seq 12553, It That Herald's End).
func TestWitnessReadsAncientTombDamageBeforeTapExtra(t *testing.T) {
	const tomb, payer = state.ObjID(4), state.PlayerID(0)
	mana := func(counter string, amount int32) events.Event {
		return events.Event{Kind: events.ManaAdd, Player: payer, Amount: amount, Counter: counter}
	}
	evs := []events.Event{
		{Kind: events.Tap, Obj: tomb},
		mana("C", 2),
		{Kind: events.Damage, Player: payer, Amount: 2},
		{Kind: events.DamageProvenance, Obj: tomb, Amount: 2},
		mana("ArtifactC", 1),
		mana("C", -1), // the payment's first spend closes the window
	}
	// Precondition: the damage is the tomb's own rider (its provenance names
	// the tomb) and the extra mana follows it.
	if evs[2].Kind != events.Damage || evs[3].Kind != events.DamageProvenance ||
		evs[3].Obj != tomb || evs[3].Amount != evs[2].Amount || evs[4].Kind != events.ManaAdd {
		t.Fatalf("fixture is not the tomb damage-then-extra shape: %+v", evs)
	}
	want := decision.ManaAmount{}
	want[state.MC] = 3
	if got, ok := activationProduction(evs, tomb, payer); !ok || got != want {
		t.Fatalf("activationProduction = %v, %v; want %v, true", got, ok, want)
	}

	// A damage event another source dealt is not the tomb's rider: it still
	// ends the window.
	other := append([]events.Event(nil), evs...)
	other[3].Obj = tomb + 1
	want = decision.ManaAmount{}
	want[state.MC] = 2
	if got, ok := activationProduction(other, tomb, payer); !ok || got != want {
		t.Fatalf("unrelated damage: activationProduction = %v, %v; want %v, true", got, ok, want)
	}
}
