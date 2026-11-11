package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// handRevealGrantLive reports whether a live MayLookAt$ Player grant over
// AffectedZone$ Hand is in force (the precondition of every test below).
func handRevealGrantLive(e *Engine) bool {
	for _, ce := range e.active() {
		if ce.MayLookAt && ce.MayLookAtAll && ce.AffectedZone == "Hand" {
			return true
		}
	}
	return false
}

func handIDs(cvs []view.CardView) []state.ObjID {
	var ids []state.ObjID
	for _, cv := range cvs {
		ids = append(ids, cv.ID)
	}
	return ids
}

// TestCR400_TelepathyRevealsOpponentHand: "Your opponents play with their
// hands revealed" shows seat 1's whole hand to seat 0 (CR 400.2 plus the
// static); seat 0's own hand stays hidden from seat 1 (Card.OppOwn).
func TestCR400_TelepathyRevealsOpponentHand(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Telepathy")}, nil)
	moveByName(t, e, 0, "Telepathy", state.ZBattlefield)
	hand := e.G.Zone(state.ZHand, 1)
	if !handRevealGrantLive(e) || len(hand) == 0 {
		t.Fatalf("precondition: grant live=%v, seat 1 hand=%d", handRevealGrantLive(e), len(hand))
	}
	for _, id := range hand {
		if !e.HandCardRevealed(id) {
			t.Fatalf("seq %d: HandCardRevealed(%d) = false for a card in seat 1's hand", len(e.L.Events), id)
		}
	}
	v := view.Project(e.G, e, 0, e.Pending())
	if got := handIDs(v.Players[1].Hand); !slices.Equal(got, hand) {
		t.Fatalf("seq %d: seat 0's view of seat 1's hand = %v, want %v", len(e.L.Events), got, hand)
	}
	if h := view.Project(e.G, e, 1, e.Pending()).Players[0].Hand; h != nil {
		t.Fatalf("seq %d: Telepathy leaked seat 0's hand to seat 1: %v", len(e.L.Events), handIDs(h))
	}
}

// TestCR400_EnduringRenewalRevealsOwnHandOnly: "Play with your hand revealed"
// shows seat 0's hand to seat 1, and shows seat 0 nothing of seat 1's.
func TestCR400_EnduringRenewalRevealsOwnHandOnly(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Enduring Renewal")}, nil)
	moveByName(t, e, 0, "Enduring Renewal", state.ZBattlefield)
	hand := e.G.Zone(state.ZHand, 0)
	if !handRevealGrantLive(e) || len(hand) == 0 {
		t.Fatalf("precondition: grant live=%v, seat 0 hand=%d", handRevealGrantLive(e), len(hand))
	}
	v := view.Project(e.G, e, 1, e.Pending())
	if got := handIDs(v.Players[0].Hand); !slices.Equal(got, hand) {
		t.Fatalf("seq %d: seat 1's view of seat 0's hand = %v, want %v", len(e.L.Events), got, hand)
	}
	if h := view.Project(e.G, e, 0, e.Pending()).Players[1].Hand; h != nil {
		t.Fatalf("seq %d: seat 1's hand shown to seat 0 without a grant: %v", len(e.L.Events), handIDs(h))
	}
}

// TestCR400_NoRevealGrantKeepsHandHidden is the guard: with no reveal grant
// in play another seat's Hand stays nil (marshals null).
func TestCR400_NoRevealGrantKeepsHandHidden(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Telepathy")}, nil)
	if handRevealGrantLive(e) || len(e.G.Zone(state.ZHand, 1)) == 0 {
		t.Fatalf("precondition: grant live=%v, seat 1 hand=%d", handRevealGrantLive(e), len(e.G.Zone(state.ZHand, 1)))
	}
	for _, viewer := range []state.PlayerID{0, 1} {
		v := view.Project(e.G, e, viewer, e.Pending())
		if h := v.Players[1-viewer].Hand; h != nil {
			t.Fatalf("seq %d: seat %d sees the other hand with no grant: %v", len(e.L.Events), viewer, handIDs(h))
		}
	}
}
