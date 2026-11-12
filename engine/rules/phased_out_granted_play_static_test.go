package rules

// phased_out_granted_play_static_test.go pins CR 702.26b for the play grants
// a permanent gained through an Animate staticAbilities$ body (Nog, Ambitious
// Cadet's Officer step: "You may play lands from your graveyard" and "You may
// play an additional land on each of your turns"): while the permanent that
// gained the statics is phased out they do not function, and phasing back in
// restores them. A newer "loses all abilities" removes them outright
// (CR 613.1f).

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPhasedOutGainedPlayGrantStops pins CR 702.26b: while Nog is phased out,
// the "play lands from your graveyard" and extra-land statics it gained from
// its Officer Animate do not function; phasing back in restores them.
func TestPhasedOutGainedPlayGrantStops(t *testing.T) {
	t.Parallel()
	e, nog := nogOfficer(t)
	g0 := graveCard(e, card(t, landSrc("Forest")), 0, 0)

	// Precondition: the grants are live while Nog is phased in.
	if got := e.adjustLandPlays(0); got != 1 {
		t.Fatalf("precondition: seat 0 extra land drops = %d, want 1", got)
	}
	if !graveLandOffered(e, 0, g0) {
		t.Fatal("precondition: the graveyard land was not offered while Nog is phased in")
	}

	e.emit(events.Event{Kind: events.PhaseOut, Obj: nog, Amount: 1})
	if o := e.G.Obj(nog); o == nil || !o.PhasedOut {
		t.Fatalf("precondition: Nog not phased out: %+v", o)
	}
	if got := e.adjustLandPlays(0); got != 0 {
		t.Fatalf("CR 702.26b: seat 0 extra land drops while Nog is phased out = %d, want 0", got)
	}
	if graveLandOffered(e, 0, g0) {
		t.Fatal("CR 702.26b: the graveyard land was still offered while Nog is phased out")
	}

	// Phasing back in is a filter reset, not a re-grant: the same grants
	// function again (CR 702.26b's "treated as though it does not exist" ends
	// with the phase in).
	e.emit(events.Event{Kind: events.PhaseOut, Obj: nog, Amount: -1})
	if got := e.adjustLandPlays(0); got != 1 {
		t.Fatalf("seat 0 extra land drops after phasing back in = %d, want 1", got)
	}
	if !graveLandOffered(e, 0, g0) {
		t.Fatal("the graveyard land was not offered after phasing back in")
	}
}

// TestGainedPlayGrantFollowsAbilityLoss pins CR 613.1f for the same route: a
// newer "loses all abilities" removes the gained play grants for good.
func TestGainedPlayGrantFollowsAbilityLoss(t *testing.T) {
	t.Parallel()
	e, _ := nogOfficer(t)
	g0 := graveCard(e, card(t, landSrc("Forest")), 0, 0)
	if got := e.adjustLandPlays(0); got != 1 {
		t.Fatalf("precondition: seat 0 extra land drops = %d, want 1", got)
	}
	if !graveLandOffered(e, 0, g0) {
		t.Fatal("precondition: the graveyard land was not offered")
	}
	moveCorpusCard(t, e, "Humility", 1, state.ZBattlefield)
	if got := e.adjustLandPlays(0); got != 0 {
		t.Fatalf("CR 613.1f: seat 0 extra land drops under Humility = %d, want 0", got)
	}
	if graveLandOffered(e, 0, g0) {
		t.Fatal("CR 613.1f: the graveyard land was still offered under Humility")
	}
}
