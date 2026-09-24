package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

func TestAttackingYouOrYourPWLKICountAndBattleExclusion(t *testing.T) {
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
	src := mk("Name:Mangara Test\nManaCost:2 W\nTypes:Creature Human\nPT:2/4\nOracle:x\n", 0)
	atYou := mk("Name:Raider\nManaCost:R\nTypes:Creature\nPT:1/1\nOracle:x\n", 1)
	pw := mk("Name:Walker\nManaCost:3 W\nTypes:Planeswalker\nLoyalty:3\nOracle:x\n", 0)
	battle := mk("Name:Battle\nManaCost:3 R\nTypes:Battle\nDefense:5\nOracle:x\n", 0)
	g.Obj(atYou).IsAttacking, g.Obj(atYou).Attacking = true, 0
	if got, ok := EvalCountOK(&fakeHost{g: g}, &Ctx{Controller: 0, Source: src}, "Count$ValidAll Creature.attackingYouOrYourPWLKI"); !ok || got != 1 {
		t.Fatalf("one attacker count=(%d,%v), want 1", got, ok)
	}
	g.Obj(atYou).AttackingBattle = pw
	if got := predicates["attackingYouOrYourPWLKI"](g, g.Obj(atYou), 0, src); !got {
		t.Fatal("attacker at controlled planeswalker did not match")
	}
	g.Obj(atYou).AttackingBattle = battle
	if got := predicates["attackingYouOrYourPWLKI"](g, g.Obj(atYou), 0, src); got {
		t.Fatal("attacker at protected battle incorrectly matched")
	}
}
