package rules

// cr-g85: CR 712.8a -- outside the battlefield and the stack a double-faced
// card has only its front face's characteristics. A modal DFC cast as its
// nonland back face is on the stack as the back face (CR 712.4d / 712.8); once
// it resolves, or is countered, into the graveyard it must be the front face
// again. The Modal reset in events.Apply used to run only when leaving the
// battlefield.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	awakenFront = "Extus, Oriq Overlord"
	awakenBack  = "Awaken the Blood Avatar"
)

// castAwakenBack puts Extus // Awaken in seat 0's hand with enough mana for
// the back face (no sacrifice fodder, so X=0 and the full {6}{B}{R} is
// paid), casts the back face and returns the engine and card with the spell
// on the stack as the back face.
func castAwakenBack(t *testing.T) (*Engine, state.ObjID) {
	t.Helper()
	e, spell, _ := sacXFixture(t, awakenBack, nil, "WWWWBBRR")
	if o := e.G.Obj(spell); o.Zone != state.ZHand || o.FaceIdx != 0 || o.Face().Name != awakenFront {
		t.Fatalf("precondition: card zone=%s face=%d %q, want the Extus front in hand", o.Zone, o.FaceIdx, o.Face().Name)
	}
	idx, ok := modalSpellOption(e, spell)
	if !ok {
		t.Fatalf("precondition: no modal_spell offer for the back face: %+v", e.Pending().Options)
	}
	submitChoices(t, e, idx)
	dx := e.Pending()
	if dx == nil || len(dx.Options) == 0 || dx.Options[0].Kind != "x" {
		t.Fatalf("precondition: no Sac<X> count ask: %+v", dx)
	}
	zero := -1
	for _, o := range dx.Options {
		if o.Amount == 0 {
			zero = o.Index
		}
	}
	if zero < 0 {
		t.Fatalf("precondition: X=0 not offered: %+v", dx.Options)
	}
	submitChoices(t, e, zero)
	// Precondition: the spell is on the stack as the back face, so the
	// graveyard assertions below actually test a reset.
	o := e.G.Obj(spell)
	if o.Zone != state.ZStack || o.FaceIdx != 1 || o.Face().Name != awakenBack {
		t.Fatalf("precondition: spell zone=%s face=%d %q, want back face on the stack",
			o.Zone, o.FaceIdx, o.Face().Name)
	}
	return e, spell
}

func assertFrontInGraveyard(t *testing.T, e *Engine, spell state.ObjID) {
	t.Helper()
	o := e.G.Obj(spell)
	if o.Zone != state.ZGraveyard {
		t.Fatalf("zone=%s, want graveyard", o.Zone)
	}
	if o.FaceIdx != 0 || o.Face().Name != awakenFront {
		t.Fatalf("graveyard object face=%d %q, want face 0 %q (CR 712.8a)",
			o.FaceIdx, o.Face().Name, awakenFront)
	}
	if mv := state.ManaValue(e.G, o); mv != 4 {
		t.Fatalf("graveyard mana value=%d, want the front face's 4", mv)
	}
}

func TestModalBackSpellFrontInGraveyard(t *testing.T) {
	t.Parallel()
	t.Run("resolved", func(t *testing.T) {
		t.Parallel()
		e, spell := castAwakenBack(t)
		drainResolution(t, e, 80)
		assertFrontInGraveyard(t, e, spell)
	})
	t.Run("countered", func(t *testing.T) {
		t.Parallel()
		e, spell := castAwakenBack(t)
		// What a counter effect does: the stack spell goes to its owner's
		// graveyard through the same MoveZone fold.
		e.emit(events.Event{Kind: events.MoveZone, Obj: spell, From: state.ZStack, To: state.ZGraveyard})
		assertFrontInGraveyard(t, e, spell)
	})
}
