package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestCanProduceManaColorAndAttackingYouOrYourPW pins the two Forge
// CardProperty words: canProduceManaColor <Colour> (a mana ability of the
// candidate can produce it; an "Any" production never counts as colourless,
// Forge's AbilityManaPart.canProduce) and attackingYouOrYourPW, the prefix
// spelling of the attackingYouOrYourPWLKI branch.
func TestCanProduceManaColorAndAttackingYouOrYourPW(t *testing.T) {
	g := state.NewGame([]string{"you", "them"})
	mk := func(src string, owner state.PlayerID) state.ObjID {
		c, d := cards.ParseBytes("t.txt", []byte(src))
		if len(d) != 0 {
			t.Fatalf("parse: %v", d)
		}
		c.Link()
		for _, f := range c.Faces {
			f.ApplyIntrinsics()
		}
		o := g.AddObject(c, owner)
		o.Zone = state.ZBattlefield
		g.SetZone(state.ZBattlefield, owner, append(g.Zone(state.ZBattlefield, owner), o.ID))
		return o.ID
	}
	src := mk("Name:Charmaw Test\nManaCost:3 R R\nTypes:Creature Dragon\nPT:4/4\nOracle:x\n", 0)
	wastes := mk("Name:Waste Test\nManaCost:no cost\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ x\nOracle:x\n", 1)
	forest := mk("Name:Forest Test\nManaCost:no cost\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ G | SpellDescription$ x\nOracle:x\n", 1)
	anyLand := mk("Name:Any Test\nManaCost:no cost\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ Any | SpellDescription$ x\nOracle:x\n", 1)
	painLand := mk("Name:Pain Test\nManaCost:no cost\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ x\nA:AB$ Mana | Cost$ T | Produced$ R | SpellDescription$ x\nOracle:x\n", 1)
	for _, tc := range []struct {
		spec string
		id   state.ObjID
		want bool
	}{
		{"Land.canProduceManaColor Colorless", wastes, true},
		{"Land.canProduceManaColor Colorless", forest, false},
		{"Land.canProduceManaColor Colorless", anyLand, false},
		{"Land.canProduceManaColor Colorless", painLand, true},
		{"Land.canProduceManaColor Green", forest, true},
		{"Land.canProduceManaColor Green", anyLand, true},
		{"Land.canProduceManaColor Green", wastes, false},
	} {
		if got := MatchesSpecCtx(g, tc.spec, tc.id, NewSpecContext(0, src)); got != tc.want {
			t.Errorf("%s on %s = %v, want %v", tc.spec, g.Obj(tc.id).Face().Name, got, tc.want)
		}
	}
	if got, ok := EvalCountOK(&fakeHost{g: g}, &Ctx{Controller: 0, Source: src}, "Count$Valid Land.OppCtrl+canProduceManaColor Colorless"); !ok || got != 2 {
		t.Fatalf("Obsidian Charmaw count=(%d,%v), want 2", got, ok)
	}
	if w := UnknownPredicates("Creature.attackingYouOrYourPW"); len(w) != 0 {
		t.Fatalf("attackingYouOrYourPW unknown: %v", w)
	}
	raider := mk("Name:Raider\nManaCost:R\nTypes:Creature\nPT:1/1\nOracle:x\n", 1)
	if MatchesSpecCtx(g, "Creature.attackingYouOrYourPW", raider, NewSpecContext(0, src)) {
		t.Fatal("non-attacker matched attackingYouOrYourPW")
	}
	g.Obj(raider).IsAttacking, g.Obj(raider).Attacking = true, 0
	if !MatchesSpecCtx(g, "Creature.attackingYouOrYourPW", raider, NewSpecContext(0, src)) {
		t.Fatal("attacker at you did not match attackingYouOrYourPW")
	}
	if MatchesSpecCtx(g, "Creature.attackingYouOrYourPW", raider, NewSpecContext(1, src)) {
		t.Fatal("attacker at you matched for the other seat")
	}
}
