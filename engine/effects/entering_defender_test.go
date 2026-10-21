package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// enteringDefenderWalker puts a face-up planeswalker onto seat 1's
// battlefield and returns its id.
func enteringDefenderWalker(t *testing.T, h *fakeHost) state.ObjID {
	t.Helper()
	pw := h.g.AddObject(mkCard(t, "Name:Walker\nTypes:Legendary Planeswalker Jace\nLoyalty:3\nOracle:x\n"), 1)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: pw.ID, From: pw.Zone, To: state.ZBattlefield})
	if o := h.g.Obj(pw.ID); o == nil || o.Zone != state.ZBattlefield || o.Controller != 1 {
		t.Fatalf("precondition: walker = %+v, want battlefield under seat 1", o)
	}
	return pw.ID
}

// TestEnteringDefenderAsksWithAWalkerAndFallsBackToTheTriggerDefender: an
// unnamed defender with two candidates poses one entering_defender ask,
// fallback first; the fake host serves no answer, so the R-9 stand-in is
// the trigger's defending player.
func TestEnteringDefenderAsksWithAWalkerAndFallsBackToTheTriggerDefender(t *testing.T) {
	h, c, id := attackingFixture(t)
	pw := enteringDefenderWalker(t, h)
	c.DefendingPlayer = state.Target{IsPlayer: true, Player: 1}
	rider := classifyAttackingEntry(h, c, &cards.SA{Params: map[string]string{"Attacking": "True"}}, state.ZBattlefield, c.Controller)
	rider.apply(h, c, id, 0, state.ZBattlefield)
	if h.askCount != 1 || h.lastAsk == nil || h.lastAsk.ResumeKind != "entering_defender" || h.lastAsk.Player != 0 {
		t.Fatalf("asks=%d last=%+v, want one entering_defender ask of seat 0", h.askCount, h.lastAsk)
	}
	if opts := h.lastAsk.Options; len(opts) != 2 || opts[0].Kind != "player" || opts[0].Player != 1 || opts[1].Obj != pw {
		t.Fatalf("options = %+v, want [player 1, walker %d]", opts, pw)
	}
	if o := h.g.Obj(id); !o.IsAttacking || o.Attacking != 1 || o.AttackingBattle != 0 {
		t.Fatalf("object attacking=%v defender=%d battle=%d, want the stand-in player 1", o.IsAttacking, o.Attacking, o.AttackingBattle)
	}
}

// TestEnteringDefenderFixedDefenderAsksNothing: K:Ninjutsu's defender is
// named by the effect (CR 702.49c), so no CR 508.4 ask is posed even with a
// second candidate on the board.
func TestEnteringDefenderFixedDefenderAsksNothing(t *testing.T) {
	h, c, id := attackingFixture(t)
	pw := enteringDefenderWalker(t, h)
	c.DefendingPlayer = state.Target{IsPlayer: true, Player: 1}
	c.DefendingBattle = pw
	c.DefenderFixed = true
	rider := classifyAttackingEntry(h, c, &cards.SA{Params: map[string]string{"Attacking": "True"}}, state.ZBattlefield, c.Controller)
	rider.apply(h, c, id, 0, state.ZBattlefield)
	if h.askCount != 0 {
		t.Fatalf("asks=%d (%+v), want none for a fixed defender", h.askCount, h.lastAsk)
	}
	if o := h.g.Obj(id); !o.IsAttacking || o.Attacking != 1 || o.AttackingBattle != pw {
		t.Fatalf("object attacking=%v defender=%d battle=%d, want the fixed walker %d", o.IsAttacking, o.Attacking, o.AttackingBattle, pw)
	}
}

// TestEnteringDefenderNoContextInCombatAttacksTheOpponent: an activated
// "return it tapped and attacking" (Nemesis Phoenix) has no trigger
// defender; during its controller's combat the CR 508.4 candidates still
// exist, so the permanent attacks the sole opponent with no degrade Note.
func TestEnteringDefenderNoContextInCombatAttacksTheOpponent(t *testing.T) {
	h, c, id := attackingFixture(t)
	h.g.Active = 0
	h.g.Step = state.StepDeclareAttackers
	rider := classifyAttackingEntry(h, c, &cards.SA{Params: map[string]string{"Attacking": "True"}}, state.ZBattlefield, c.Controller)
	rider.apply(h, c, id, 0, state.ZBattlefield)
	if o := h.g.Obj(id); !o.IsAttacking || o.Attacking != 1 || o.Tapped {
		t.Fatalf("object attacking=%v defender=%d tapped=%v, want attacking seat 1, untapped", o.IsAttacking, o.Attacking, o.Tapped)
	}
	if n := countAttackingNotes(h); n != 0 {
		t.Fatalf("Attacking$ notes = %d, want 0 (a defender exists)", n)
	}
}
