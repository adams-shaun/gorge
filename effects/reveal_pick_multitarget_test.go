package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// handCard adds a card named name to owner's hand and returns its id.
func handCard(t *testing.T, h *fakeHost, name string, owner state.PlayerID) state.ObjID {
	t.Helper()
	o := h.g.AddObject(mkCard(t, "Name:"+name+"\nTypes:Creature\nPT:1/1\nOracle:x\n"), owner)
	o.Zone = state.ZHand
	return o.ID
}

// revealPickPublicNotes returns every non-Secret Note carrying ids, in order.
func revealPickPublicNotes(log []events.Event) []events.Event {
	var out []events.Event
	for _, e := range log {
		if e.Kind == events.Note && !e.Secret && len(e.IDs) > 0 {
			out = append(out, e)
		}
	}
	return out
}

// TestHandRevealPickPosesOncePerDefinedTarget pins the per-target cursor on
// the hand-reveal pick (the r2 review's MAJOR fix): a pickable reveal
// resolving over MORE than one Defined$ player must pose one pick per target
// and apply each answer to its OWN hand. Pre-fix the answer was consumed once
// into a local slice and that same slice was applied while iterating every
// later player's distinct hand — none of the first player's object ids can
// occur in another hand, so the pool emptied, n became 0, and every target
// after the first was silently skipped with no ask and no reveal.
//
// The board is the reproducer's: three seats, `Defined$ Player.Opponent` (the
// controller's two opponents, seats 1 and 2), each with a two-card hand, so
// each hand is pickable (2 eligible > the mandatory 1) and the two hands are
// disjoint.
func TestHandRevealPickPosesOncePerDefinedTarget(t *testing.T) {
	h := newHost(t, 3)

	// Precondition: the two opponent hands are non-empty and DISJOINT — the
	// assertion below (each target reveals its own hand) depends on it. A
	// shared card would let the pre-fix bug pass by accident.
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
	m := sa(t, "SP$ Reveal | Defined$ Player.Opponent | RememberRevealed$ True")

	// Pass 1: the pick for target 0 (seat 1), bound to ResumeTarget 0, over
	// seat 1's own hand. No reveal yet.
	Resolve(sh, ctx, m)
	if sh.asked == nil || sh.asked.ResumeKind != "reveal_pick" {
		t.Fatalf("pass 1 posed %+v, want a reveal_pick", sh.asked)
	}
	if sh.asked.Player != 1 || sh.asked.ResumeTarget != 0 {
		t.Fatalf("pass 1 ask = Player %d target %d, want seat 1 / target 0", sh.asked.Player, sh.asked.ResumeTarget)
	}
	if len(sh.asked.Options) != 2 || sh.asked.Options[0].Obj != s1a || sh.asked.Options[1].Obj != s1b {
		t.Fatalf("pass 1 options = %+v, want seat 1's hand %d,%d", sh.asked.Options, s1a, s1b)
	}
	if got := revealPickPublicNotes(sh.log); len(got) != 0 {
		t.Fatalf("a reveal note landed before any answer: %+v", got)
	}

	// Pass 2: answer seat 1's pick. Target 0's note lands; target 1 (seat 2)
	// poses its OWN pick, bound to ResumeTarget 1, over SEAT 2's hand.
	sh.suspended, sh.asked = false, nil
	ctx.RevealPick = []state.ObjID{s1a}
	ctx.RevealPickTarget = 0
	Resolve(sh, ctx, m)
	notes := revealPickPublicNotes(sh.log)
	if len(notes) != 1 || notes[0].Player != 1 || len(notes[0].IDs) != 1 || notes[0].IDs[0] != s1a {
		t.Fatalf("after seat 1's answer notes = %+v, want exactly seat 1's chosen card", notes)
	}
	if sh.asked == nil || sh.asked.ResumeKind != "reveal_pick" {
		t.Fatalf("pass 2 posed %+v, want a second reveal_pick for seat 2", sh.asked)
	}
	if sh.asked.Player != 2 || sh.asked.ResumeTarget != 1 {
		t.Fatalf("pass 2 ask = Player %d target %d, want seat 2 / target 1", sh.asked.Player, sh.asked.ResumeTarget)
	}
	if len(sh.asked.Options) != 2 || sh.asked.Options[0].Obj != s2a || sh.asked.Options[1].Obj != s2b {
		t.Fatalf("pass 2 options = %+v, want seat 2's hand %d,%d", sh.asked.Options, s2a, s2b)
	}

	// Pass 3: answer seat 2's pick. Its note lands, target 0 is skipped (not
	// re-emitted), and the walk TERMINATES with no third ask.
	sh.suspended, sh.asked = false, nil
	ctx.RevealPick = []state.ObjID{s2b}
	ctx.RevealPickTarget = 1
	Resolve(sh, ctx, m)
	if sh.asked != nil {
		t.Fatalf("the walk re-posed an ask after every target was answered: %+v", sh.asked)
	}
	notes = revealPickPublicNotes(sh.log)
	if len(notes) != 2 {
		t.Fatalf("got %d reveal notes (%+v), want exactly one per target", len(notes), notes)
	}
	if notes[0].Player != 1 || notes[0].IDs[0] != s1a {
		t.Fatalf("note 0 = %+v, want seat 1 revealing %d", notes[0], s1a)
	}
	if notes[1].Player != 2 || notes[1].IDs[0] != s2b {
		t.Fatalf("note 1 = %+v, want seat 2 revealing %d", notes[1], s2b)
	}
}
