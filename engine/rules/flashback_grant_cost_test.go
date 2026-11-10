package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestGrantedFlashbackOwnCost: a continuous effect granting flashback with a
// cost of its own (Iroh, Grand Lotus's "each Lesson card in your graveyard has
// flashback {1}") is that card's flashback cost, the only instance offered,
// when the card prints no flashback (CR 702.34a): the plan and the cast charge
// {1}, not the card's {2}{R}, and the resolved card is exiled. A grant with no
// cost of its own still costs the mana cost.
func TestGrantedFlashbackOwnCost(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _, lesson := newFixtureDeck(t, 9702, "Name:Study Hall Probe\nManaCost:2 R\nTypes:Instant Lesson\nA:SP$ Draw | NumCards$ 1\nOracle:x\n")
	onBoardCard(t, e, 0, tlaCorpusCard(t, reg, "Iroh, Grand Lotus"))
	for i := 0; i < 3; i++ {
		onBoard(t, e, 0, paymentPlanIsland)
	}
	toGraveyard(t, e, lesson)
	if e.G.Active != 0 {
		t.Fatalf("precondition: active player %d, want 0 (the grant is your-turn only)", e.G.Active)
	}
	if !e.hasKeywordH(lesson, kwhFlashback) {
		t.Fatalf("precondition: the Lesson has no derived flashback: %v", e.Derived(lesson).Keywords)
	}
	if c := e.flashbackCost(lesson); c.Generic != 1 || c.Colored != (state.Mana{}) {
		t.Fatalf("flashbackCost = generic %d coloured %v, want {1}", c.Generic, c.Colored)
	}
	if x := e.extraFlashbackCosts(lesson); len(x) != 0 {
		t.Fatalf("extraFlashbackCosts = %v, want none: {1} is the one instance", x)
	}

	d := paymentPlanReask(t, e)
	a := paymentPlanActionFor(t, d, lesson)
	if a.Cast.Origin != decision.OriginFlashback {
		t.Fatalf("action origin = %q, want %q", a.Cast.Origin, decision.OriginFlashback)
	}
	if got := a.Plans[0].Cost; got.Generic != 1 || got.Mana != (decision.ManaAmount{}) {
		t.Fatalf("plan cost = %+v, want {1}", got)
	}
	submitPaymentPlan(t, e, d, a)
	if z := e.G.Obj(lesson).Zone; z != state.ZStack {
		t.Fatalf("Lesson zone = %s, want stack", z)
	}
	tapped := 0
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if e.G.Obj(id).Tapped {
			tapped++
		}
	}
	if tapped != 1 {
		t.Errorf("%d permanents tapped, want the one Island paying {1}", tapped)
	}
	resolveStack(t, e)
	if z := e.G.Obj(lesson).Zone; z != state.ZExile {
		t.Errorf("resolved Lesson zone = %s, want exile (flashback)", z)
	}
}
