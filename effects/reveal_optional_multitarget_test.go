package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// TestOptionalRevealPosesOncePerDefinedTarget pins the per-target cursor on
// the Optional$ reveal yes/no (the sol2 review's MAJOR fix): an optional reveal
// resolving over MORE than one Defined$ player must pose one yes/no per target
// and apply each answer to its OWN hand.
//
// Pre-fix the reveal_optional decision carried no ResumeTarget and the resume
// arm set no cursor, so the single consumed Ctx.RevealOpt answer was read by
// every loop iteration. The consequences the review measured: a "yes" for
// target 0 silently accepted every later target with no ask; a "no" for target
// 0 silently declined every later target too.
//
// The board is the reproducer's: three seats, `Defined$ Player.Opponent` (the
// controller's two opponents, seats 1 and 2), each with two cards so the hand
// is non-empty (n > 0, so the optional block is reached) and the two hands are
// disjoint (a shared card would let a mis-applied answer pass by accident).
func TestOptionalRevealPosesOncePerDefinedTarget(t *testing.T) {
	h := newHost(t, 3)

	s1a := handCard(t, h, "Seat1Alpha", 1)
	s1b := handCard(t, h, "Seat1Beta", 1)
	h.g.SetZone(state.ZHand, 1, []state.ObjID{s1a, s1b})
	s2a := handCard(t, h, "Seat2Alpha", 2)
	s2b := handCard(t, h, "Seat2Beta", 2)
	h.g.SetZone(state.ZHand, 2, []state.ObjID{s2a, s2b})
	for _, a := range []state.ObjID{s1a, s1b} {
		for _, b := range []state.ObjID{s2a, s2b} {
			if a == b {
				t.Fatalf("precondition: hands share object %d, want disjoint", a)
			}
		}
	}
	if len(h.g.Zone(state.ZHand, 1)) != 2 || len(h.g.Zone(state.ZHand, 2)) != 2 {
		t.Fatalf("precondition: hands = %v / %v, want two cards each",
			h.g.Zone(state.ZHand, 1), h.g.Zone(state.ZHand, 2))
	}

	sh := &suspendHost{fakeHost: *h}
	ctx := &Ctx{Controller: 0}
	// A non-pickable optional reveal (whole hand, no NumCards$): the ONLY ask
	// is the yes/no, so the assertion isolates the optional cursor.
	m := sa(t, "SP$ RevealHand | Defined$ Player.Opponent | Optional$ True")

	// Pass 1: the yes/no for target 0 (seat 1), bound to ResumeTarget 0. No
	// reveal yet.
	Resolve(sh, ctx, m)
	if sh.asked == nil || sh.asked.ResumeKind != "reveal_optional" {
		t.Fatalf("pass 1 posed %+v, want a reveal_optional ask", sh.asked)
	}
	if sh.asked.Player != 1 || sh.asked.ResumeTarget != 0 {
		t.Fatalf("pass 1 ask = Player %d target %d, want seat 1 / target 0", sh.asked.Player, sh.asked.ResumeTarget)
	}
	if got := revealPickPublicNotes(sh.log); len(got) != 0 {
		t.Fatalf("a reveal note landed before any answer: %+v", got)
	}

	// Pass 2: DECLINE seat 1. Seat 1 must stay unrevealed, and seat 2 must
	// pose its OWN yes/no, bound to ResumeTarget 1 -- the pre-fix bug
	// silently applied the "no" to seat 2 and finished the walk.
	sh.suspended, sh.asked = false, nil
	ctx.RevealOpt, ctx.RevealOptTarget = "no", 0
	Resolve(sh, ctx, m)
	if got := revealPickPublicNotes(sh.log); len(got) != 0 {
		t.Fatalf("a declined reveal emitted a note: %+v", got)
	}
	if sh.asked == nil || sh.asked.ResumeKind != "reveal_optional" {
		t.Fatalf("pass 2 posed %+v, want seat 2's own reveal_optional ask", sh.asked)
	}
	if sh.asked.Player != 2 || sh.asked.ResumeTarget != 1 {
		t.Fatalf("pass 2 ask = Player %d target %d, want seat 2 / target 1", sh.asked.Player, sh.asked.ResumeTarget)
	}

	// Pass 3: ACCEPT seat 2. Seat 1 is skipped (never re-asked, never
	// revealed), and seat 2's whole hand is revealed. The walk terminates
	// with no third ask.
	sh.suspended, sh.asked = false, nil
	ctx.RevealOpt, ctx.RevealOptTarget = "yes", 1
	Resolve(sh, ctx, m)
	if sh.asked != nil {
		t.Fatalf("the walk re-posed an ask after every target was answered: %+v", sh.asked)
	}
	notes := revealPickPublicNotes(sh.log)
	if len(notes) != 1 {
		t.Fatalf("got %d reveal notes (%+v), want exactly seat 2's", len(notes), notes)
	}
	n := notes[0]
	if n.Player != 2 || len(n.IDs) != 2 || n.IDs[0] != s2a || n.IDs[1] != s2b {
		t.Fatalf("reveal note = %+v, want seat 2's own hand %d,%d", n, s2a, s2b)
	}
}

// TestOptionalRevealAcceptDoesNotAcceptLaterTargets pins the mirror case: a
// YES for target 0 must not silently accept every later target. Seat 0
// accepts seat 1's hand, and seat 2 is then asked its own question rather
// than revealed unasked.
func TestOptionalRevealAcceptDoesNotAcceptLaterTargets(t *testing.T) {
	h := newHost(t, 3)
	s1 := handCard(t, h, "Seat1Card", 1)
	h.g.SetZone(state.ZHand, 1, []state.ObjID{s1})
	s2 := handCard(t, h, "Seat2Card", 2)
	h.g.SetZone(state.ZHand, 2, []state.ObjID{s2})

	sh := &suspendHost{fakeHost: *h}
	ctx := &Ctx{Controller: 0}
	m := sa(t, "SP$ RevealHand | Defined$ Player.Opponent | Optional$ True")

	Resolve(sh, ctx, m) // target 0's ask
	if sh.asked == nil || sh.asked.ResumeKind != "reveal_optional" || sh.asked.ResumeTarget != 0 {
		t.Fatalf("pass 1 posed %+v, want the target-0 reveal_optional ask", sh.asked)
	}
	sh.suspended, sh.asked = false, nil
	ctx.RevealOpt, ctx.RevealOptTarget = "yes", 0
	Resolve(sh, ctx, m)
	notes := revealPickPublicNotes(sh.log)
	if len(notes) != 1 || notes[0].Player != 1 || len(notes[0].IDs) != 1 || notes[0].IDs[0] != s1 {
		t.Fatalf("after seat 1 accepted notes = %+v, want exactly seat 1's hand", notes)
	}
	// Precondition for the assertion below: seat 2's hand is non-empty, so
	// the optional block is genuinely reachable for target 1.
	if len(h.g.Zone(state.ZHand, 2)) == 0 {
		t.Fatalf("precondition: seat 2's hand is empty, the later-target ask is unreachable")
	}
	if sh.asked == nil || sh.asked.ResumeKind != "reveal_optional" || sh.asked.Player != 2 || sh.asked.ResumeTarget != 1 {
		t.Fatalf("pass 2 posed %+v, want seat 2's OWN reveal_optional ask (accept did not leak)", sh.asked)
	}
	// Seat 2's hand must not have been revealed by seat 1's yes.
	for _, e := range notes {
		if e.Player == 2 {
			t.Fatalf("seat 2's hand was revealed without its own answer: %+v", e)
		}
	}
}
