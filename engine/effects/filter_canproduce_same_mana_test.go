package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestCanProduceSameManaTypeWith pins Mana Web's body predicate: the
// candidate is a Land that can produce at least one type of mana the
// triggering land can produce (card text: "all lands that player controls
// that could produce any type of mana that land could produce"). The trigger
// referent is SpecContext.TriggerCard, the card the triggering event named;
// with no bound trigger card the predicate fails closed. {C} counts as a
// producible type, so a Wastes shares a type with another Wastes.
func TestCanProduceSameManaTypeWith(t *testing.T) {
	g := state.NewGame(names(2))
	add := func(owner state.PlayerID, src string) state.ObjID {
		o := g.AddObject(mkCard(t, src), owner)
		g.SetZone(state.ZBattlefield, owner, append(g.Zone(state.ZBattlefield, owner), o.ID))
		return o.ID
	}
	forest := func(name string) state.ObjID {
		return add(1, "Name:"+name+"\nTypes:Basic Land Forest\nA:AB$ Mana | Cost$ T | Produced$ G | SpellDescription$ Add {G}.\nOracle:x\n")
	}
	mountain := add(1, "Name:Mountain\nTypes:Basic Land Mountain\nA:AB$ Mana | Cost$ T | Produced$ R | SpellDescription$ Add {R}.\nOracle:x\n")
	dual := add(1, "Name:Dual\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ Combo G R | SpellDescription$ Add {G} or {R}.\nOracle:x\n")
	wastes := func(name string) state.ObjID {
		return add(1, "Name:"+name+"\nTypes:Basic Land Wastes\nA:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Add {C}.\nOracle:x\n")
	}
	forest1, forest2 := forest("Forest One"), forest("Forest Two")
	wastes1, wastes2 := wastes("Wastes One"), wastes("Wastes Two")

	const spec = "TriggeredCard.canProduceSameManaTypeWith+Land"
	if un := UnknownPredicates(spec); len(un) != 0 {
		t.Fatalf("UnknownPredicates(%q) = %v, want empty (the predicate is modelled)", spec, un)
	}

	// The triggering land is a Forest: every Forest and the dual land share
	// green with it; the Mountain and the Wastes do not.
	sc := SpecContext{You: 0, TriggerContext: TriggerContext{TriggerCard: forest1}}
	for _, tc := range []struct {
		name string
		id   state.ObjID
		want bool
	}{
		{"another Forest", forest2, true},
		{"the triggering Forest", forest1, true},
		{"a dual land producing green", dual, true},
		{"a Mountain", mountain, false},
		{"a Wastes", wastes1, false},
	} {
		if got := MatchesObjectCtx(g, spec, g.Obj(tc.id), sc); got != tc.want {
			t.Errorf("%s: MatchesObjectCtx(%q) = %v, want %v", tc.name, spec, got, tc.want)
		}
	}

	// The triggering land is a Wastes: only a land producing {C} shares a
	// type with it.
	sc = SpecContext{You: 0, TriggerContext: TriggerContext{TriggerCard: wastes1}}
	if !MatchesObjectCtx(g, spec, g.Obj(wastes2), sc) {
		t.Error("a Wastes must share {C} with the triggering Wastes")
	}
	if MatchesObjectCtx(g, spec, g.Obj(forest2), sc) {
		t.Error("a Forest must not share a mana type with the triggering Wastes")
	}

	// No bound trigger card: the predicate fails closed, never widened.
	if MatchesObjectCtx(g, spec, g.Obj(forest2), SpecContext{You: 0}) {
		t.Error("the predicate matched without a bound trigger card")
	}
}
