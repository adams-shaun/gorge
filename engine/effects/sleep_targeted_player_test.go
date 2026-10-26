package effects_test

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestSleepScopesToTargetedPlayer is the class leaf for a `DB$ *All` sub whose
// scope is inherited from its parent's player target: Sleep's parent TapAll
// carries ValidTgts$ Player, and its DBPumpAll names `Defined$ Targeted` with
// no ValidTgts$ of its own. The pump half must cover only the target player's
// creatures (engine/effects/scoped_sweep.go's targetPlayerKindScope), not the
// caster's -- the reported bug stamped CantUntapNextStep on p0's own board.
//
// The test drives the whole flow through the rules engine so the untap step
// half is real: p1's creature must stay tapped through p1's next untap step,
// while p0's creature is never tapped and never carries the window.
func TestSleepScopesToTargetedPlayer(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	sleep, ok := reg.Lookup("Sleep")
	if !ok {
		t.Fatal("corpus fixture: Sleep missing")
	}
	myCreature, ok := reg.Lookup("Centaur Courser")
	if !ok {
		t.Fatal("corpus fixture: Centaur Courser missing")
	}
	theirCreature, ok := reg.Lookup("Centaur Courser")
	if !ok {
		t.Fatal("corpus fixture: Centaur Courser missing")
	}
	mountain, ok := reg.Lookup("Mountain")
	if !ok {
		t.Fatal("corpus fixture: Mountain missing")
	}
	library := func(n int) []*cards.Card {
		out := make([]*cards.Card, n)
		for i := range out {
			out[i] = mountain
		}
		return out
	}
	e, ids, err := rules.NewStaged(rules.Config{Seed: 7, Names: []string{"p0", "p1"}, Tokens: map[string]*cards.Card{}}, rules.Stage{
		Turn: 1, Active: 0, Starting: 0, Step: state.StepMain1, Enter: rules.StagePriorityFresh,
		Players: []rules.StagedPlayer{
			{Life: 20, ManaPool: "UUCC", Library: library(20), Hand: []*cards.Card{sleep}},
			{Life: 20, Library: library(20)},
		},
		Permanents: []rules.StagedPermanent{
			{Card: myCreature, Owner: 0, Controller: 0, Sick: true},
			{Card: theirCreature, Owner: 1, Controller: 1, Sick: true},
		},
	})
	if err != nil {
		t.Fatalf("stage: %v", err)
	}
	if len(ids.Hand[0]) != 1 || ids.Hand[0][0] == 0 {
		t.Fatalf("precondition: Sleep not in p0's staged hand: %v", ids.Hand[0])
	}
	mine, theirs := ids.Permanents[0], ids.Permanents[1]
	for _, id := range []state.ObjID{mine, theirs} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: staged permanent %d zone = %v, want battlefield", id, o)
		}
	}
	if e.G.Obj(mine).Controller == e.G.Obj(theirs).Controller {
		t.Fatal("precondition: the two staged creatures share a controller")
	}
	if e.G.Obj(theirs).Tapped || e.G.Obj(mine).Tapped {
		t.Fatal("precondition: a staged creature started tapped")
	}
	sleepID := ids.Hand[0][0]

	e.Advance()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("precondition: no p0 priority decision: %+v", d)
	}
	castIdx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == sleepID {
			castIdx = o.Index
		}
	}
	if castIdx < 0 {
		t.Fatalf("precondition: Sleep is not offered as a cast: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{castIdx}}); err != nil {
		t.Fatalf("cast Sleep: %v", err)
	}

	// Answer the player target, then drain the spell to resolution.
	answeredTarget := false
	resolved := false
	for i := 0; i < 200 && !resolved; i++ {
		if e.G.Over {
			t.Fatal("game ended while resolving Sleep")
		}
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		switch d.Kind {
		case decision.KTarget:
			if answeredTarget {
				t.Fatalf("Sleep posed a second target ask: %+v", d)
			}
			idx := -1
			for _, o := range d.Options {
				if o.Player == 1 {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("Sleep's target ask did not offer p1: %+v", d.Options)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
				t.Fatalf("target p1: %v", err)
			}
			answeredTarget = true
		case decision.KPriority:
			if answeredTarget && len(e.G.Stack) == 0 {
				resolved = true
				break
			}
			sleepPassPriority(t, e, d)
		case decision.KAttackers:
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{}}); err != nil {
				t.Fatalf("decline attackers: %v", err)
			}
		default:
			t.Fatalf("unexpected decision %v while resolving Sleep: %+v", d.Kind, d)
		}
	}
	if !resolved {
		t.Fatal("Sleep did not resolve within the pass budget")
	}

	// The bug: the pump half swept every seat, so p0's creature carried the
	// window too. Both halves are asserted before the untap step.
	if !e.G.Obj(theirs).Tapped {
		t.Fatal("target player's creature was not tapped")
	}
	if !e.G.Obj(theirs).CantUntapNextStep {
		t.Fatal("target player's creature did not receive the no-untap window")
	}
	if e.G.Obj(mine).Tapped {
		t.Fatal("non-target player's creature was tapped by Sleep")
	}
	if e.G.Obj(mine).CantUntapNextStep {
		t.Fatal("non-target player's creature received the no-untap window")
	}

	// Drive to p1's next untap step (turn 2, upkeep is after it): the target
	// creature must still be tapped there.
	for i := 0; i < 4000; i++ {
		if e.G.Turn == 2 && e.G.Active == 1 && e.G.Step == state.StepUpkeep {
			break
		}
		if e.G.Over {
			t.Fatalf("game ended before p1's next untap step (turn %d seat %d step %s)",
				e.G.Turn, e.G.Active, e.G.Step)
		}
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision while driving to p1's next untap step")
		}
		switch d.Kind {
		case decision.KPriority:
			sleepPassPriority(t, e, d)
		case decision.KAttackers, decision.KBlockers:
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{}}); err != nil {
				t.Fatalf("decline combat: %v", err)
			}
		default:
			t.Fatalf("unexpected decision %v while driving to p1's untap step: %+v", d.Kind, d)
		}
	}
	if !e.G.Obj(theirs).Tapped {
		t.Fatal("target player's creature untapped during its next untap step")
	}
	if e.G.Obj(mine).Tapped {
		t.Fatal("non-target player's creature ended up tapped")
	}
}

func sleepPassPriority(t *testing.T, e *rules.Engine, d *decision.Decision) {
	t.Helper()
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "pass" {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("priority decision with no pass option: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
		t.Fatalf("pass priority: %v", err)
	}
}
