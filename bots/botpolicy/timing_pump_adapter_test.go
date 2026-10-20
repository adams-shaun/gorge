package botpolicy_test

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// TestPumpAdaptersAgree pins the A6a card fact both halves must fill: the
// game-shaped BoardFromGame and the view-shaped BoardFromView read the same
// literal pump bonus off the same printed card. The view half is built
// through view.Project -- the REAL projection a client receives -- so a
// reverted projection line fails here, not only in the whole-game parity
// tests. A missing fill on either half makes the timing rule see a different
// board depending on the host.
func TestPumpAdaptersAgree(t *testing.T) {
	for _, tc := range []struct {
		name  string
		src   string
		power int32
		tough int32
		own   bool
	}{
		{
			name:  "literal pump",
			src:   "Name:Giant Growth\nManaCost:G\nTypes:Instant\nOracle:Target creature gets +3/+3 until end of turn.\nA:SP$ Pump | ValidTgts$ Creature | NumAtt$ +3 | NumDef$ +3 | SpellDescription$ Target creature gets +3/+3 until end of turn.\n",
			power: 3,
			tough: 3,
			own:   true,
		},
		{
			name:  "power only",
			src:   "Name:Rush of Adrenaline\nManaCost:R\nTypes:Instant\nOracle:Target creature gets +2/+0 and gains haste until end of turn.\nA:SP$ Pump | ValidTgts$ Creature | NumAtt$ +2 | SpellDescription$ Target creature gets +2/+0 and gains haste until end of turn.\n",
			power: 2,
			own:   true,
		},
		{
			name: "negative bonus",
			src:  "Name:Shrink\nManaCost:U\nTypes:Instant\nOracle:Target creature gets -2/-2 until end of turn.\nA:SP$ Pump | ValidTgts$ Creature | NumAtt$ -2 | NumDef$ -2 | SpellDescription$ Target creature gets -2/-2 until end of turn.\n",
		},
		{
			name: "dynamic amount",
			src:  "Name:Untamed Might\nManaCost:X G\nTypes:Instant\nOracle:Target creature gets +X/+X until end of turn.\nA:SP$ Pump | ValidTgts$ Creature | NumAtt$ +X | NumDef$ +X | SpellDescription$ Target creature gets +X/+X until end of turn.\n",
		},
		{
			name: "not a creature",
			src:  "Name:Twiddle\nManaCost:U\nTypes:Instant\nOracle:Tap or untap target artifact.\nA:SP$ Pump | ValidTgts$ Artifact | NumAtt$ +1 | SpellDescription$ Tap or untap target artifact.\n",
		},
		{
			name: "opponent-only target",
			src:  "Name:Wrong Target\nManaCost:U\nTypes:Instant\nOracle:Target creature an opponent controls gets +3/+3 until end of turn.\nA:SP$ Pump | ValidTgts$ Creature.OppCtrl | NumAtt$ +3 | NumDef$ +3 | SpellDescription$ Target creature an opponent controls gets +3/+3 until end of turn.\n",
		},
		{
			name: "you-dont-control target",
			src:  "Name:Not Yours\nManaCost:U\nTypes:Instant\nOracle:Target creature you don't control gets +2/+2 until end of turn.\nA:SP$ Pump | ValidTgts$ Creature.YouDontCtrl | NumAtt$ +2 | NumDef$ +2 | SpellDescription$ Target creature you don't control gets +2/+2 until end of turn.\n",
		},
		{
			name:  "attacking-only target",
			src:   "Name:Run Amok\nManaCost:1 R\nTypes:Instant\nOracle:Target attacking creature gets +3/+3 until end of turn.\nA:SP$ Pump | ValidTgts$ Creature.attacking | NumAtt$ +3 | NumDef$ +3 | SpellDescription$ Target attacking creature gets +3/+3 until end of turn.\n",
			power: 3,
			tough: 3,
		},
		{
			name:  "blocking-only target",
			src:   "Name:Barge In\nManaCost:R\nTypes:Instant\nOracle:Target blocking creature gets +2/+2 until end of turn.\nA:SP$ Pump | ValidTgts$ Creature.blocking+YouCtrl | NumAtt$ +2 | NumDef$ +2 | SpellDescription$ Target blocking creature gets +2/+2 until end of turn.\n",
			power: 2,
			tough: 2,
		},
		{
			name:  "own-creature target",
			src:   "Name:Might of the Masses\nManaCost:G\nTypes:Instant\nOracle:Target creature you control gets +1/+1 until end of turn.\nA:SP$ Pump | ValidTgts$ Creature.YouCtrl | NumAtt$ +1 | NumDef$ +1 | SpellDescription$ Target creature you control gets +1/+1 until end of turn.\n",
			power: 1,
			tough: 1,
			own:   true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			card, diags := cards.ParseBytes("timing_pump_adapter_test.go", []byte(tc.src))
			if len(diags) != 0 {
				t.Fatalf("parse: %v", diags)
			}
			card.Link()
			g := state.NewGame([]string{"a", "b"})
			o := g.AddObject(card, 0)
			g.SetZone(state.ZHand, 0, []state.ObjID{o.ID})

			v := view.Project(g, nil, 0, nil)
			var cv *view.CardView
			for i := range v.Players[0].Hand {
				if v.Players[0].Hand[i].ID == o.ID {
					cv = &v.Players[0].Hand[i]
				}
			}
			if cv == nil {
				t.Fatalf("projection: hand card %d missing", o.ID)
			}
			if cv.PumpPower != tc.power || cv.PumpToughness != tc.tough || cv.PumpOwn != tc.own {
				t.Fatalf("projected pump = %d/%d own %v, want %d/%d own %v", cv.PumpPower, cv.PumpToughness, cv.PumpOwn, tc.power, tc.tough, tc.own)
			}

			gameCard := botpolicy.BoardFromGame(g, printedChars{}, 0).Cards.Get(o.ID)
			viewCard := heuristic.BoardFromView(v).Cards.Get(o.ID)
			if gameCard.PumpPower != tc.power || gameCard.PumpToughness != tc.tough || gameCard.PumpOwn != tc.own {
				t.Fatalf("game facts pump = %d/%d own %v, want %d/%d own %v", gameCard.PumpPower, gameCard.PumpToughness, gameCard.PumpOwn, tc.power, tc.tough, tc.own)
			}
			if viewCard.PumpPower != tc.power || viewCard.PumpToughness != tc.tough || viewCard.PumpOwn != tc.own {
				t.Fatalf("view facts pump = %d/%d own %v, want %d/%d own %v", viewCard.PumpPower, viewCard.PumpToughness, viewCard.PumpOwn, tc.power, tc.tough, tc.own)
			}
		})
	}
}

// TestPumpCreatureFactsAdaptersAgree pins the A6a Creature facts both halves
// must fill from the same combat state: Attacking, Defender and Blocks (the
// lowest blocker id) agree between BoardFromGame and BoardFromView.
func TestPumpCreatureFactsAdaptersAgree(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	mk := func(name string, owner state.PlayerID) *state.Object {
		src := fmt.Sprintf("Name:%s\nTypes:Creature\nPT:2/2\nOracle:x\n", name)
		card, diags := cards.ParseBytes("timing_pump_adapter_test.go", []byte(src))
		if len(diags) != 0 {
			t.Fatalf("parse: %v", diags)
		}
		card.Link()
		o := g.AddObject(card, owner)
		g.SetZone(state.ZBattlefield, owner, []state.ObjID{o.ID})
		return o
	}
	attackerID := mk("Attacker", 0).ID
	blockerID := mk("Blocker", 1).ID
	attacker := g.Obj(attackerID)
	attacker.IsAttacking, attacker.Attacking = true, 1
	attacker.BlockedBy = []state.ObjID{blockerID}
	g.Obj(blockerID).IsBlocking = true
	g.NoteBlockers()

	gameBoard := botpolicy.BoardFromGame(g, printedChars{}, 0)
	viewBoard := heuristic.BoardFromView(view.Project(g, nil, 0, nil))
	for _, id := range []state.ObjID{attackerID, blockerID} {
		gc, vc := gameBoard.Creatures.Get(id), viewBoard.Creatures.Get(id)
		if gc.Attacking != vc.Attacking || gc.Defender != vc.Defender || gc.Blocks != vc.Blocks {
			t.Fatalf("creature %d: game %+v vs view %+v disagree on Attacking/Defender/Blocks", id, gc, vc)
		}
	}
	if a := gameBoard.Creatures.Get(attackerID); !a.Attacking || a.Defender != 1 || a.Blocks != 0 {
		t.Fatalf("attacker facts = %+v, want Attacking, Defender 1, Blocks 0", a)
	}
	if b := gameBoard.Creatures.Get(blockerID); b.Attacking || b.Blocks != attacker.ID {
		t.Fatalf("blocker facts = %+v, want Blocks %d", b, attackerID)
	}
}

// TestPumpFaceDownStackFactsAdaptersAgree is the CR 708.4 half of the
// StackEntry pump parity: the controller sees their own face-down spell's
// printed bonus on both halves, while an opponent sees 0/0 on both.
func TestPumpFaceDownStackFactsAdaptersAgree(t *testing.T) {
	src := "Name:Giant Growth\nManaCost:G\nTypes:Instant\nOracle:x\nA:SP$ Pump | ValidTgts$ Creature | NumAtt$ +3 | NumDef$ +3 | SpellDescription$ x\n"
	card, diags := cards.ParseBytes("timing_pump_adapter_test.go", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("parse: %v", diags)
	}
	card.Link()
	g := state.NewGame([]string{"a", "b"})
	o := g.AddObject(card, 0)
	g.SetZone(state.ZHand, 0, []state.ObjID{o.ID})
	events.Apply(g, events.Event{Kind: events.PutOnStack, Obj: o.ID, Player: 0,
		From: state.ZHand, To: state.ZStack, Counter: events.FaceDownEntryCounter})
	if !g.Obj(o.ID).FaceDown {
		t.Fatal("precondition: the spell is not face down")
	}
	for _, tc := range []struct {
		viewer   state.PlayerID
		power    int32
		tough    int32
		scenario string
	}{
		{viewer: 0, power: 3, tough: 3, scenario: "controller"},
		{viewer: 1, power: 0, tough: 0, scenario: "opponent"},
	} {
		gameBoard := botpolicy.BoardFromGame(g, printedChars{}, tc.viewer)
		viewBoard := heuristic.BoardFromView(view.Project(g, nil, tc.viewer, nil))
		var gs, vs botpolicy.StackEntry
		for _, s := range gameBoard.Stack {
			if s.ID == o.ID {
				gs = s
			}
		}
		for _, s := range viewBoard.Stack {
			if s.ID == o.ID {
				vs = s
			}
		}
		if gs.PumpPower != tc.power || gs.PumpToughness != tc.tough {
			t.Fatalf("%s game stack pump = %d/%d, want %d/%d", tc.scenario, gs.PumpPower, gs.PumpToughness, tc.power, tc.tough)
		}
		if vs.PumpPower != tc.power || vs.PumpToughness != tc.tough {
			t.Fatalf("%s view stack pump = %d/%d, want %d/%d", tc.scenario, vs.PumpPower, vs.PumpToughness, tc.power, tc.tough)
		}
	}
}
