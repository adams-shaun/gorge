package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The round-2 review finding (fx45): a DECLINED Optional$ hand reveal
// resumed into a MANDATORY reveal_pick. On the reveal_optional resume with
// answer "no", deferToOptionalAsk was false (it only covers the first pass)
// and the pick block ran before the decline `continue` at the bottom of the
// walk, so a player who declined "You may reveal a card from your hand"
// (Dragons Disciple, Vault 21: House Gambit, Temple of the Dragon Queen)
// was then forced to pick a card and reveal it anyway. This file pins the
// decline arm of the pickable shape; the accept arm's pick is pinned in
// effects/infernal_tutor_test.go.

// TestOptionalHandRevealDeclinePosesNoPick drives the declined resume of a
// PICKABLE optional hand reveal (a two-card hand, NumCards default 1 -- the
// reviewer's exact repro shape, also Dragons Disciple's real parameters): no
// reveal_pick may be posed, nothing is revealed, and the walk finishes.
func TestOptionalHandRevealDeclinePosesNoPick(t *testing.T) {
	h := newHost(t, 2)
	var hand []state.ObjID
	for _, n := range []string{"Alpha", "Beta"} {
		o := h.g.AddObject(mkCard(t, "Name:"+n+"\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
		o.Zone = state.ZHand
		hand = append(hand, o.ID)
	}
	h.g.SetZone(state.ZHand, 0, hand)
	sh := &suspendHost{fakeHost: *h}
	ctx := &Ctx{Controller: 0, Source: hand[0]}
	m := sa(t, "SP$ Reveal | Defined$ You | Optional$ True")

	// Pass 1: the may-reveal yes/no is posed (precondition for the decline
	// arm to even be reachable).
	Resolve(sh, ctx, m)
	if sh.asked == nil || sh.asked.ResumeKind != "reveal_optional" {
		t.Fatalf("first pass = %+v, want the reveal_optional ask", sh.asked)
	}

	// Pass 2 ("no"): the walk must finish with no further ask and no reveal.
	sh.suspended, sh.asked = false, nil
	ctx.RevealOpt = "no"
	Resolve(sh, ctx, m)
	if sh.asked != nil {
		t.Fatalf("a declined optional hand reveal posed another ask (fx45 regression): %+v", sh.asked)
	}
	for _, e := range sh.log {
		if e.Kind == events.Note {
			t.Fatalf("a declined optional hand reveal emitted a reveal Note: %+v", e)
		}
	}
}

// TestOptionalHandRevealDeclineWithRevealValidPosesNoPick runs the same gate
// through Vault 21: House Gambit's REAL parameters (NumCards$ 5 over a
// nonland-narrowed hand): a declined "reveal up to five" over a SIX-card
// hand must not fall into a mandatory pick either (the hand is strictly
// larger than the count, so the pre-fix build posed a 5-of-6 pick here).
func TestOptionalHandRevealDeclineWithRevealValidPosesNoPick(t *testing.T) {
	h := newHost(t, 2)
	var hand []state.ObjID
	for _, n := range []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon", "Zeta"} {
		o := h.g.AddObject(mkCard(t, "Name:"+n+"\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
		o.Zone = state.ZHand
		hand = append(hand, o.ID)
	}
	h.g.SetZone(state.ZHand, 0, hand)
	sh := &suspendHost{fakeHost: *h}
	ctx := &Ctx{Controller: 0, Source: hand[0]}
	m := sa(t, "DB$ Reveal | NumCards$ 5 | Optional$ True | RevealValid$ Card.nonLand+YouOwn | RememberRevealed$ True")

	Resolve(sh, ctx, m)
	if sh.asked == nil || sh.asked.ResumeKind != "reveal_optional" {
		t.Fatalf("first pass = %+v, want the reveal_optional ask", sh.asked)
	}
	sh.suspended, sh.asked = false, nil
	ctx.RevealOpt = "no"
	Resolve(sh, ctx, m)
	if sh.asked != nil {
		t.Fatalf("a declined NumCards$ 5 optional reveal posed a pick (fx45 regression): %+v", sh.asked)
	}
	for _, e := range sh.log {
		if e.Kind == events.Note {
			t.Fatalf("a declined NumCards$ 5 optional reveal emitted a Note: %+v", e)
		}
	}
	if len(ctx.Remembered) != 0 {
		t.Fatalf("a declined reveal must leave Remembered empty, got %+v", ctx.Remembered)
	}
}
