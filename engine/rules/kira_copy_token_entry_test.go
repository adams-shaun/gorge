package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestKiraReplacesACopyTokensEntry: a token copy is created (CR 111.1, 707.2)
// by entering the battlefield, so Kira Nerys, Firebrand's replacement (CR
// 614.1c, 614.12) sees that entry like any other creature's: Replicator's copy
// of a creature token, made while its controller controls an attacking
// creature, enters tapped and attacking. It must not be lost: the mint is never
// left in its pre-entry slot to cease as a token off the battlefield.
func TestKiraReplacesACopyTokensEntry(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	bear := card(t, "Name:Test Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e, _ := corpusEngineCfg(t, reg, []*cards.Card{
		lookup(t, reg, "Kira Nerys, Firebrand"), lookup(t, reg, "Replicator"), bear,
	}, nil)
	moveByName(t, e, 0, "Kira Nerys, Firebrand", state.ZBattlefield)
	rep := moveByName(t, e, 0, "Replicator", state.ZBattlefield)
	b := moveByName(t, e, 0, "Test Bear", state.ZBattlefield)
	kiraDrain(t, e)
	e.G.Obj(b).SummonSick = false
	e.G.Obj(rep).SummonSick = false
	e.G.Active = 0
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{b}})
	e.G.Step = state.StepDeclareAttackers
	kiraDrain(t, e)
	if !e.G.Obj(b).IsAttacking {
		t.Fatal("precondition: the Bear is not attacking")
	}
	tok := e.G.NextID
	e.EmitTokenCreate(events.Event{Kind: events.TokenCreate, Player: 0, Text: "r_1_1_goblin"})
	kiraDrain(t, e)
	if o := e.G.Obj(tok); o == nil || !o.IsToken || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: the Goblin token is not on the battlefield: %+v", o)
	}
	e.G.Step = state.StepDeclareBlockers
	e.pending = nil
	e.G.Players[0].Pool[state.MC] = 2
	d := activateObjAbility(t, e, 0, rep)
	pick := -1
	for _, o := range d.Options {
		if o.Obj == tok {
			pick = o.Index
		}
	}
	if pick < 0 {
		t.Fatalf("precondition: Replicator cannot target the Goblin token: %+v", d.Options)
	}
	submitChoices(t, e, pick)
	cp := e.G.NextID
	passUntilStackEmpty(t, e, 20)
	kiraDrain(t, e)
	o := e.G.Obj(cp)
	if o == nil || !o.IsToken || o.Zone != state.ZBattlefield {
		t.Fatalf("Replicator's copy did not enter the battlefield: %+v", o)
	}
	if !o.Tapped || !o.IsAttacking || o.Attacking != 1 {
		t.Fatalf("copy tapped=%v attacking=%v defender=%d, want tapped and attacking seat 1", o.Tapped, o.IsAttacking, o.Attacking)
	}
}
