package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestDamageAskLeavesDamageProofLast: two Phyrexian Vindicators aimed at each
// other redirect the prevented damage back and forth until the object cap (a
// Commander pod did). A damage ask's DamageEffect.Prevented permanent is
// passed over while another target meets Min, and still answers a Min it
// alone can meet.
func TestDamageAskLeavesDamageProofLast(t *testing.T) {
	t.Parallel()
	b := Board{Creatures: makeTable[state.ObjID, Creature](4)}
	b.Creatures.Set(400, Creature{Power: 5, Toughness: 5, Controller: 1, Keywords: []string{"Flying"}})
	b.Creatures.Set(401, Creature{Power: 1, Toughness: 1, Controller: 1})
	mk := func() *decision.Decision {
		return &decision.Decision{Seq: 1, Player: 0, Kind: decision.KTarget, Min: 1, Max: 1,
			TargetEffect: &decision.TargetEffect{API: "DealDamage", Damage: &decision.DamageEffect{Prevented: []state.ObjID{400}}},
			Options: []decision.Option{
				{Index: 0, Kind: "player", Player: 1},
				{Index: 1, Kind: "permanent", Obj: 400, Player: 1, Controller: 1},
				{Index: 2, Kind: "permanent", Obj: 401, Player: 1, Controller: 1},
			}}
	}
	if got := Decide(b, mk(), rng(1)).Choices; len(got) != 1 || got[0] == 1 {
		t.Fatalf("damage choices = %v: aimed at the permanent that prevents it", got)
	}
	d := mk()
	d.Options = d.Options[1:2]
	d.Options[0].Index = 0
	if got := Decide(b, d, rng(1)).Choices; len(got) != 1 || got[0] != 0 {
		t.Fatalf("a forced pick of the damage-proof permanent = %v, want [0]", got)
	}
}
