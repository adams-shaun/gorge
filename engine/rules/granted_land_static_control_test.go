package rules

// granted_land_static_control_test.go pins CR 113.8 / 611.3a for a land-play
// grant a permanent gained from an Animate (Nog, Ambitious Cadet's Officer
// step: "You may play lands from your graveyard" and "You may play an
// additional land on each of your turns"): "you" is the permanent's current
// controller, so the permission and the extra drop move with a control
// change, and a later "loses all abilities" (CR 613.1f) removes both.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func TestGrantedLandStaticFollowsControllerAndAbilityLoss(t *testing.T) {
	t.Parallel()
	e, nog := nogOfficer(t)
	g0 := graveCard(e, card(t, landSrc("Forest")), 0, 0)
	g1 := graveCard(e, card(t, landSrc("Island")), 1, 1)

	// Precondition: under its owner the grant is seat 0's alone.
	if got := e.adjustLandPlays(0); got != 1 {
		t.Fatalf("precondition: seat 0 extra land drops = %d, want 1", got)
	}
	if e.adjustLandPlays(1) != 0 || !graveLandOffered(e, 0, g0) || graveLandOffered(e, 1, g1) {
		t.Fatalf("precondition: the grant is not seat 0's alone")
	}

	// Control passes to seat 1: the permission and the drop follow Nog.
	e.emit(events.Event{Kind: events.ControlChange, Obj: nog, Player: 1})
	if c := e.G.Obj(nog).Controller; c != 1 {
		t.Fatalf("precondition: Nog's controller = %d, want 1", c)
	}
	if e.adjustLandPlays(0) != 0 || e.adjustLandPlays(1) != 1 {
		t.Errorf("extra drops after the steal = %d/%d, want 0/1", e.adjustLandPlays(0), e.adjustLandPlays(1))
	}
	if graveLandOffered(e, 0, g0) {
		t.Error("the old controller is still offered a land from its graveyard")
	}
	if !graveLandOffered(e, 1, g1) {
		t.Error("the new controller is not offered a land from its graveyard")
	}

	// A newer Humility removes the grants whoever controls Nog.
	moveCorpusCard(t, e, "Humility", 1, state.ZBattlefield)
	if e.adjustLandPlays(1) != 0 || graveLandOffered(e, 1, g1) {
		t.Errorf("Nog under Humility still grants land plays (drops %d)", e.adjustLandPlays(1))
	}
}
