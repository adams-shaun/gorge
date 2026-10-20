package heuristic

import (
	"context"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/state"
)

// postureAggroDeck is the R1 aggro manifest (24 lands; 18 one-drops and 18
// two-drops: average 1.5) used by the A4 travel tests.
func postureAggroDeck() *deck.Manifest {
	return &deck.Manifest{
		Main: []deck.ManifestRow{
			{Name: "Land", Count: 24, Land: true},
			{Name: "One", Count: 18},
			{Name: "Two", Count: 18},
		},
		Curve: []deck.CurveRow{{CMC: 0, Count: 24}, {CMC: 1, Count: 18}, {CMC: 2, Count: 18}},
	}
}

func postureBoard(creatures map[state.ObjID]botpolicy.Creature, life map[state.PlayerID]int32) botpolicy.Board {
	return botpolicy.Board{
		OwnDeck:   postureAggroDeck(),
		Creatures: botpolicy.TableOf(creatures),
		Life:      botpolicy.TableOf(life),
	}
}

func postureAttackDecision(ids ...state.ObjID) decision.Decision {
	d := decision.Decision{Seq: 1, Player: 0, Kind: decision.KAttackers, Min: 0, Max: len(ids)}
	for _, id := range ids {
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "attacker", Obj: id, Player: 1})
	}
	return d
}

func TestPostureRulesTravelOnBoard(t *testing.T) {
	b := postureBoard(
		map[state.ObjID]botpolicy.Creature{
			101: {Power: 3, Toughness: 3, Controller: 0},
			201: {Power: 2, Toughness: 2, Controller: 1},
		},
		map[state.PlayerID]int32{0: 20, 1: 20},
	)
	d := postureAttackDecision(101)
	in, err := NewBot(1).DecideBoard(context.Background(), b, d)
	if err != nil {
		t.Fatalf("DecideBoard: %v", err)
	}
	if !slices.Equal(in.Choices, []int{}) {
		t.Fatalf("unarmed attackers = %v, want none", in.Choices)
	}
	in, err = NewBot(1).EnablePostureRules(botpolicy.PostureRules{Attack: true, Reserve: true}).DecideBoard(context.Background(), b, d)
	if err != nil {
		t.Fatalf("DecideBoard: %v", err)
	}
	if !slices.Equal(in.Choices, []int{0}) {
		t.Fatalf("armed attackers = %v, want [0]", in.Choices)
	}
}

func TestPostureRulesReachAttackSim(t *testing.T) {
	b := postureBoard(
		map[state.ObjID]botpolicy.Creature{
			101: {Power: 1, Toughness: 1, Controller: 0},
			102: {Power: 2, Toughness: 2, Controller: 0},
			201: {Power: 2, Toughness: 1, Controller: 1},
			202: {Power: 2, Toughness: 1, Controller: 1},
		},
		map[state.PlayerID]int32{0: 11, 1: 10},
	)
	d := postureAttackDecision(101, 102)
	in, err := NewAttackSimBot(1, botpolicy.DefaultAttackSimParams()).DecideBoard(context.Background(), b, d)
	if err != nil {
		t.Fatalf("DecideBoard: %v", err)
	}
	if !slices.Equal(in.Choices, []int{}) {
		t.Fatalf("unarmed attackers = %v, want none", in.Choices)
	}
	in, err = NewAttackSimBot(1, botpolicy.DefaultAttackSimParams()).EnablePostureRules(botpolicy.PostureRules{Attack: true}).DecideBoard(context.Background(), b, d)
	if err != nil {
		t.Fatalf("DecideBoard: %v", err)
	}
	if !slices.Equal(in.Choices, []int{0}) {
		t.Fatalf("armed attackers = %v, want [0]", in.Choices)
	}
}
