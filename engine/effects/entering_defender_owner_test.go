package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestEnteringDefenderUsesTheDestinationOwner: CR 508.4's chooser is the
// entering permanent's controller, not the resolving source's controller.
// Noctis, Heir Apparent exiles itself and returns it under its OWNER's
// control tapped and attacking, so the owner's combat (and the owner's
// choice) is the one that matters.
func TestEnteringDefenderUsesTheDestinationOwner(t *testing.T) {
	h := newHost(t, 3)
	walker := h.g.AddObject(mkCard(t, "Name:Walker\nTypes:Legendary Planeswalker Jace\nLoyalty:3\nOracle:x\n"), 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: walker.ID, From: walker.Zone, To: state.ZBattlefield})
	returning := h.g.AddObject(mkCard(t, "Name:Returning\nTypes:Creature\nPT:2/2\nOracle:x\n"), 1)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: returning.ID, From: returning.Zone, To: state.ZExile})
	h.g.Active = 1
	h.g.Step = state.StepDeclareAttackers
	c := &Ctx{Source: returning.ID, Controller: 0}
	effChangeZone(h, c, &cards.SA{API: "ChangeZone", Params: map[string]string{
		"Defined": "Self", "Origin": "Exile", "Destination": "Battlefield", "Attacking": "True",
	}})
	if h.askCount != 1 || h.lastAsk == nil || h.lastAsk.ResumeKind != "entering_defender" || h.lastAsk.Player != 1 {
		t.Fatalf("asks=%d last=%+v, want one entering_defender ask of the OWNER seat 1", h.askCount, h.lastAsk)
	}
	if opts := h.lastAsk.Options; len(opts) != 3 || opts[0].Player != 0 || opts[1].Player != 2 || opts[2].Obj != walker.ID {
		t.Fatalf("options = %+v, want [player 0, player 2, walker %d]", opts, walker.ID)
	}
	o := h.g.Obj(returning.ID)
	if o == nil || !o.IsAttacking || o.Attacking != 0 || o.Controller != 1 {
		t.Fatalf("returned object = %+v, want seat 1's permanent attacking the stand-in seat 0", o)
	}
}

// TestTokenAttackingTrueAsksTheMintOwner: TokenOwner$ can name a seat other
// than the resolving controller (Najeela's TriggeredAttackerController). The
// CR 508.4 ask belongs to the mint's owner, not to c.Controller.
func TestTokenAttackingTrueAsksTheMintOwner(t *testing.T) {
	h := newHost(t, 3)
	h.g.Tokens = tokenFixtures(t)
	h.g.Active = 1
	h.g.Step = state.StepDeclareAttackers
	c := &Ctx{Source: 1, Controller: 0, TriggerContext: TriggerContext{AttackingPlayer: state.Target{IsPlayer: true, Player: 1}}}
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Token", Params: map[string]string{
		"TokenScript": "r_1_1_goblin", "TokenOwner": "TriggeredAttackerController",
		"TokenAttacking": "True",
	}})
	if h.askCount != 1 || h.lastAsk == nil || h.lastAsk.ResumeKind != "entering_defender" || h.lastAsk.Player != 1 {
		t.Fatalf("asks=%d last=%+v, want one entering_defender ask of the mint owner seat 1", h.askCount, h.lastAsk)
	}
	var attacks []events.Event
	for _, ev := range h.log {
		if ev.Kind == events.TokenAttacks {
			attacks = append(attacks, ev)
		}
	}
	if len(attacks) != 1 || attacks[0].Player != 1 || len(attacks[0].IDs) != 1 || attacks[0].IDs[0] != 0 {
		t.Fatalf("TokenAttacks = %+v, want one mint owned by seat 1 attacking stand-in seat 0", attacks)
	}
	if n := countAttackingNotes(h); n != 0 {
		t.Fatalf("TokenAttacking$ notes = %d, want 0 (the mint owner's combat has candidates)", n)
	}
}

// TestTokenAttackForControlChangeUsesTheFallback: a CreateToken replacement
// (Crafty Cutpurse) moves the mint to another controller. That controller did
// not answer the owner's ask, so the frozen fallback is used, and a fallback
// naming the new controller itself is dropped (a token cannot attack itself).
func TestTokenAttackForControlChangeUsesTheFallback(t *testing.T) {
	job := &TokenJob{AttackOwners: []tokenAttackAssignment{{
		Owner: 0, Defender: 2, Fallback: 2, HasFallback: true,
	}}}
	if defender, battle, ok := job.attackFor(0, 1); !ok || defender != 2 || battle != 0 {
		t.Fatalf("control-changed mint = (%d, %d, %v), want the owner's fallback seat 2", defender, battle, ok)
	}
	job.AttackOwners[0].Fallback = 1
	if _, _, ok := job.attackFor(0, 1); ok {
		t.Fatal("control-changed mint attacked its own new controller; want no attack")
	}
	if defender, _, ok := job.attackFor(0, 0); !ok || defender != 2 {
		t.Fatalf("unmoved mint = (%d, %v), want the served answer seat 2", defender, ok)
	}
}

// TestEnteringDefenderCopyFallbackDropsTheBattle: a CopyPermanent mint has no
// CopyToken battle slot, so a trigger fallback that is a battle must degrade
// to the battle's protector player rather than offering a battle the fold
// would read as a player.
func TestEnteringDefenderCopyFallbackDropsTheBattle(t *testing.T) {
	h := newHost(t, 2)
	h.g.Active = 0
	h.g.Step = state.StepDeclareAttackers
	battle := state.ObjID(4242)
	c := &Ctx{Controller: 0,
		TriggerContext: TriggerContext{
			DefendingPlayer: state.Target{IsPlayer: true, Player: 1},
			DefendingBattle: battle,
		}}
	d, ok := enteringDefenderFallback(h, c, 0, false)
	if !ok || d.Player != 1 || d.Obj != 0 {
		t.Fatalf("copy fallback = (%+v, %v), want player 1 with no battle", d, ok)
	}
	d, ok = enteringDefenderFallback(h, c, 0, true)
	if !ok || d.Player != 1 || d.Obj != battle {
		t.Fatalf("token/permanent fallback = (%+v, %v), want player 1 with battle %d", d, ok, battle)
	}
}
