package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// TestOptionalPickableRevealAsksEachTarget first accepts and picks from one
// opponent's hand, then verifies that the next opponent still gets its own
// may-reveal ask before its own pick. An answered RevealPick belongs only to
// its ResumeTarget; it must not suppress Optional$ for a later target.
func TestOptionalPickableRevealAsksEachTarget(t *testing.T) {
	h := newHost(t, 3)
	s1a := handCard(t, h, "Seat1Alpha", 1)
	s1b := handCard(t, h, "Seat1Beta", 1)
	s2a := handCard(t, h, "Seat2Alpha", 2)
	s2b := handCard(t, h, "Seat2Beta", 2)
	h.g.SetZone(state.ZHand, 1, []state.ObjID{s1a, s1b})
	h.g.SetZone(state.ZHand, 2, []state.ObjID{s2a, s2b})
	if len(h.g.Zone(state.ZHand, 1)) != 2 || len(h.g.Zone(state.ZHand, 2)) != 2 || s1a == s2a || s1b == s2b {
		t.Fatalf("precondition: need two disjoint, pickable hands; got %v / %v", h.g.Zone(state.ZHand, 1), h.g.Zone(state.ZHand, 2))
	}

	sh := &suspendHost{fakeHost: *h}
	ctx := &Ctx{Controller: 0}
	m := sa(t, "SP$ Reveal | Defined$ Player.Opponent | Optional$ True")

	Resolve(sh, ctx, m)
	if d := sh.asked; d == nil || d.ResumeKind != "reveal_optional" || d.Player != 1 || d.ResumeTarget != 0 {
		t.Fatalf("pass 1 ask = %+v, want seat 1's optional gate", d)
	}

	// Seat 1 accepts, so it gets its own one-of-two reveal pick.
	sh.suspended, sh.asked = false, nil
	ctx.RevealOpt, ctx.RevealOptTarget = "yes", 0
	Resolve(sh, ctx, m)
	if d := sh.asked; d == nil || d.ResumeKind != "reveal_pick" || d.Player != 1 || d.ResumeTarget != 0 {
		t.Fatalf("pass 2 ask = %+v, want seat 1's reveal pick", d)
	}

	// Once seat 1 answers that pick, seat 2 must get its OPTIONAL gate, not a
	// mandatory reveal_pick. This is the regression: global picks != nil used
	// to skip the gate for every later target.
	sh.suspended, sh.asked = false, nil
	ctx.RevealPick, ctx.RevealPickTarget = []state.ObjID{s1b}, 0
	Resolve(sh, ctx, m)
	if notes := revealPickPublicNotes(sh.log); len(notes) != 1 || notes[0].Player != 1 || len(notes[0].IDs) != 1 || notes[0].IDs[0] != s1b {
		t.Fatalf("after seat 1's pick notes = %+v, want its chosen card", notes)
	}
	if d := sh.asked; d == nil || d.ResumeKind != "reveal_optional" || d.Player != 2 || d.ResumeTarget != 1 {
		t.Fatalf("pass 3 ask = %+v, want seat 2's own optional gate", d)
	}

	// Seat 2 accepts and independently chooses its card; the walk then ends.
	sh.suspended, sh.asked = false, nil
	ctx.RevealOpt, ctx.RevealOptTarget = "yes", 1
	Resolve(sh, ctx, m)
	if d := sh.asked; d == nil || d.ResumeKind != "reveal_pick" || d.Player != 2 || d.ResumeTarget != 1 {
		t.Fatalf("pass 4 ask = %+v, want seat 2's reveal pick", d)
	}
	sh.suspended, sh.asked = false, nil
	ctx.RevealPick, ctx.RevealPickTarget = []state.ObjID{s2a}, 1
	Resolve(sh, ctx, m)
	if sh.asked != nil {
		t.Fatalf("walk re-posed an ask after both picks: %+v", sh.asked)
	}
	notes := revealPickPublicNotes(sh.log)
	if len(notes) != 2 || notes[1].Player != 2 || len(notes[1].IDs) != 1 || notes[1].IDs[0] != s2a {
		t.Fatalf("final notes = %+v, want seat 2's chosen card", notes)
	}
}
