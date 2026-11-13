package effects

// equalpt_attackingsame_test.go pins two Forge CardProperty words the MBC
// filters-9 cards write: equalPT (Gilt-Leaf Winnower's "target non-Elf
// creature whose power and toughness aren't equal", Creature.nonElf+!equalPT)
// and attackingSame (Kitesail Skirmisher's "another target creature attacking
// the same player or planeswalker", Creature.Other+attackingSame). Before
// either was read the word was unknown, matched nobody, and the trigger never
// found a target.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

func equalPTBoard(t *testing.T, pts ...string) (*state.Game, []state.ObjID) {
	t.Helper()
	g := state.NewGame([]string{"a", "b", "c"})
	var ids []state.ObjID
	for i, pt := range pts {
		c, d := cards.ParseBytes("t.txt", []byte("Name:C"+string(rune('A'+i))+"\nManaCost:1 G\nTypes:Creature Bear\nPT:"+pt+"\nOracle:x\n"))
		if len(d) != 0 {
			t.Fatalf("diags: %v", d)
		}
		c.Link()
		for _, f := range c.Faces {
			f.ApplyIntrinsics()
		}
		o := g.AddObject(c, 0)
		o.Zone = state.ZBattlefield
		g.SetZone(state.ZBattlefield, 0, append(g.Zone(state.ZBattlefield, 0), o.ID))
		ids = append(ids, o.ID)
	}
	return g, ids
}

func TestEqualPTWord(t *testing.T) {
	g, ids := equalPTBoard(t, "2/2", "3/1", "0/0")
	sc := NewSpecContext(0, 0)
	if got := UnknownPredicates("Creature.nonElf+!equalPT"); len(got) != 0 {
		t.Fatalf("equalPT still unknown: %v", got)
	}
	if !SpecReadsPT("Creature.!equalPT") {
		t.Fatalf("SpecReadsPT must see equalPT: a layer-derived P/T decides it")
	}
	for i, want := range []struct{ eq, ne bool }{{true, false}, {false, true}, {true, false}} {
		if g.Obj(ids[i]).Face().PT == "" {
			t.Fatalf("fixture %d has no printed P/T", i)
		}
		if got := MatchesSpecCtx(g, "Creature.equalPT", ids[i], sc); got != want.eq {
			t.Errorf("object %d Creature.equalPT = %v, want %v", i, got, want.eq)
		}
		if got := MatchesSpecCtx(g, "Creature.nonElf+!equalPT", ids[i], sc); got != want.ne {
			t.Errorf("object %d Creature.nonElf+!equalPT = %v, want %v", i, got, want.ne)
		}
	}
	// a layer-derived P/T (a pump the rules tier binds) is what Forge's net
	// values read: 2/2 pumped to 3/2 is no longer equal
	pumped := NewSpecContext(0, 0)
	pumped.HasDerivedPT, pumped.DerivedPower, pumped.DerivedToughness = true, 3, 2
	if MatchesSpecCtx(g, "Creature.equalPT", ids[0], pumped) {
		t.Errorf("a 3/2 derived P/T must not be equalPT")
	}
}

func TestAttackingSameWord(t *testing.T) {
	g, ids := equalPTBoard(t, "2/2", "2/2", "2/2", "2/2", "2/2")
	src, same, otherSeat, walkerSame, idle := ids[0], ids[1], ids[2], ids[3], ids[4]
	// src and same attack seat 1; otherSeat attacks seat 2; walkerSame attacks
	// seat 1's planeswalker (a different defender entity than the player)
	g.Obj(src).IsAttacking, g.Obj(src).Attacking = true, 1
	g.Obj(same).IsAttacking, g.Obj(same).Attacking = true, 1
	g.Obj(otherSeat).IsAttacking, g.Obj(otherSeat).Attacking = true, 2
	g.Obj(walkerSame).IsAttacking, g.Obj(walkerSame).Attacking, g.Obj(walkerSame).AttackingBattle = true, 1, ids[4]
	sc := NewSpecContext(0, src)
	if got := UnknownPredicates("Creature.Other+attackingSame"); len(got) != 0 {
		t.Fatalf("attackingSame still unknown: %v", got)
	}
	for _, c := range []struct {
		name string
		id   state.ObjID
		want bool
	}{
		{"same defender", same, true},
		{"another seat", otherSeat, false},
		{"same seat, a planeswalker instead of the player", walkerSame, false},
		{"not attacking", idle, false},
	} {
		if got := MatchesSpecCtx(g, "Creature.Other+attackingSame", c.id, sc); got != c.want {
			t.Errorf("%s: Creature.Other+attackingSame = %v, want %v", c.name, got, c.want)
		}
	}
	// the source itself attacks the same defender as itself; Other drops it
	if !MatchesSpecCtx(g, "Creature.attackingSame", src, sc) || MatchesSpecCtx(g, "Creature.Other+attackingSame", src, sc) {
		t.Errorf("the source is attackingSame with itself and not Other")
	}
	// a source that is not attacking has no defender: nothing matches
	g.Obj(src).IsAttacking = false
	if MatchesSpecCtx(g, "Creature.Other+attackingSame", same, sc) {
		t.Errorf("a non-attacking source must match nothing")
	}
}
