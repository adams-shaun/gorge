package heuristic

import (
	"context"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// attachRuleBoard is the pump-Aura ask: an own 2/2 (101) and the opponent's
// 4/4 (201). The plain ranking takes the opponent's creature.
func attachRuleBoard() botpolicy.Board {
	b := botpolicy.Board{
		Cards:     botpolicy.TableOf(map[state.ObjID]botpolicy.Card{}),
		Creatures: botpolicy.TableOf(map[state.ObjID]botpolicy.Creature{}),
		Life:      botpolicy.TableOf(map[state.PlayerID]int32{}),
	}
	b.Creatures.Set(101, botpolicy.Creature{Power: 2, Toughness: 2, Controller: 0})
	b.Creatures.Set(201, botpolicy.Creature{Power: 4, Toughness: 4, Controller: 1})
	b.Stack = []botpolicy.StackEntry{{ID: 900, Controller: 0, IsSpell: true}}
	return b
}

func attachRuleDecision() decision.Decision {
	return decision.Decision{
		Seq: 1, Player: 0, Kind: decision.KTarget, Min: 1, Max: 1, Source: 900,
		TargetEffect: &decision.TargetEffect{API: "Attach", AttachLogic: "Pump"},
		Options: []decision.Option{
			{Index: 0, Kind: "permanent", Obj: 201, Player: 1, Controller: 1},
			{Index: 1, Kind: "permanent", Obj: 101, Player: 0, Controller: 0},
		},
	}
}

// TestBotAppliesAttachRules pins that the A5 rules travel on the Board for
// every entry point: the armed bot aims the pump Aura at its own creature,
// a bot with every rule switched off at the opponent's.
func TestBotAppliesAttachRules(t *testing.T) {
	r := botpolicy.AttachRules{AuraPolarity: true, Carrier: true}
	if got := NewBot(1).EnableAttachRules(r).AttachRules(); got != r {
		t.Fatalf("AttachRules() = %+v, want %+v", got, r)
	}
	plain, err := NewBot(1).EnableAttachRules(botpolicy.AttachRules{}).DecideBoard(context.Background(), attachRuleBoard(), attachRuleDecision())
	if err != nil {
		t.Fatal(err)
	}
	if len(plain.Choices) != 1 || plain.Choices[0] != 0 {
		t.Fatalf("unarmed choices = %v, want [0] (the opponent's 201); the fixture no longer discriminates", plain.Choices)
	}
	armed, err := NewBot(1).EnableAttachRules(r).DecideBoard(context.Background(), attachRuleBoard(), attachRuleDecision())
	if err != nil {
		t.Fatal(err)
	}
	if len(armed.Choices) != 1 || armed.Choices[0] != 1 {
		t.Fatalf("armed choices = %v, want [1] (own 101)", armed.Choices)
	}
}
