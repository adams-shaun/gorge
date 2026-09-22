package effects

// count_attackingyou_test.go pins the Count$Valid Creature.attackingYou head
// (Arachnogenesis' SVar:X:Count$Valid Creature.attackingYou — "create X 1/2
// green Spider creature tokens with reach, where X is the number of creatures
// attacking you"; 28 corpus files carry the spec). The predicate binds
// through the count's SpecContext.Source — the resolving spell or ability's
// own object, whose controller IS the counting player — so an attacker at the
// counting player's seat matches and an attacker at any other defender does
// not. Before attackingYou existed as a filter predicate at all the unknown
// word matched nobody and X degraded to 0 (the "card makes zero tokens"
// defect class), so the test also asserts the sibling `attacking` head
// differs: three attackers declared, only two of them at the counting player.

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// attackingYouBoard builds a three-seat game and returns it together with the
// ids of: a battlefield source controlled by seat 0 (the counting player), a
// second source controlled by seat 1, two creatures attacking seat 0, one
// creature attacking seat 1, and one creature not attacking at all.
func attackingYouBoard(t *testing.T) (g *state.Game, src0, src1, atYou1, atYou2, atOther, idle state.ObjID) {
	t.Helper()
	g = state.NewGame([]string{"counting", "other", "third"})
	mk := func(owner state.PlayerID, src string) state.ObjID {
		c, d := cards.ParseBytes("t.txt", []byte(src))
		if len(d) != 0 {
			t.Fatalf("diags: %v", d)
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
	src0 = mk(0, "Name:CountingSource\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	src1 = mk(1, "Name:OtherSource\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	atYou1 = mk(1, "Name:Raider1\nManaCost:2 R\nTypes:Creature Goblin\nPT:2/2\nOracle:x\n")
	atYou2 = mk(2, "Name:Raider2\nManaCost:2 R\nTypes:Creature Goblin\nPT:2/2\nOracle:x\n")
	atOther = mk(2, "Name:Raider3\nManaCost:2 R\nTypes:Creature Goblin\nPT:2/2\nOracle:x\n")
	idle = mk(1, "Name:Sitter\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	// Bind the attack declarations AFTER every object exists: AddObject
	// appends to g.Objs, so an *Object captured mid-helper can point at a
	// stale backing array once the slice grows (Game.Obj always re-derives).
	for _, b := range []struct {
		id       state.ObjID
		defender state.PlayerID
	}{{atYou1, 0}, {atYou2, 0}, {atOther, 1}} {
		o := g.Obj(b.id)
		o.IsAttacking, o.Attacking = true, b.defender
	}
	return g, src0, src1, atYou1, atYou2, atOther, idle
}

// TestCountValidAttackingYouCountsAttackersAtTheCountingPlayer is the count
// leaf: with two attackers declared at seat 0 and one at seat 1, the head
// evaluated from a seat-0 source counts exactly the two, the same head from a
// seat-1 source counts the one, and the undifferentiated `attacking` sibling
// counts all three — so the predicate is the defender binding, not the
// IsAttacking flag. The preconditions the differential rows rest on (each
// attacker on the battlefield, attacking, and its defender binding pointing
// where the fixture says) are asserted first, so a vacuous setup fails loudly.
func TestCountValidAttackingYouCountsAttackersAtTheCountingPlayer(t *testing.T) {
	g, src0, src1, atYou1, atYou2, atOther, idle := attackingYouBoard(t)

	// Preconditions: the two attackers at seat 0 and the one at seat 1 are on
	// the battlefield with exactly the bindings the differential rows need,
	// and the idle creature is not attacking.
	for id, want := range map[state.ObjID]state.PlayerID{atYou1: 0, atYou2: 0, atOther: 1} {
		o := g.Obj(id)
		if o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("attacker %d missing from the battlefield: %+v", id, o)
		}
		if !o.IsAttacking || o.Attacking != want {
			t.Fatalf("attacker %d fixture broken: IsAttacking %v Attacking %d, want attacking seat %d",
				id, o.IsAttacking, o.Attacking, want)
		}
	}
	if idleO := g.Obj(idle); idleO == nil || idleO.Zone != state.ZBattlefield || idleO.IsAttacking {
		t.Fatalf("idle fixture broken: %+v", idleO)
	}
	if g.Obj(src0) == nil || g.Obj(src0).Controller != 0 || g.Obj(src1) == nil || g.Obj(src1).Controller != 1 {
		t.Fatalf("source fixture broken: src0 %+v src1 %+v", g.Obj(src0), g.Obj(src1))
	}

	h := &fakeHost{g: g}

	// The counting player's own source: two attackers at them.
	c := &Ctx{Controller: 0, Source: src0}
	if got, ok := EvalCountOK(h, c, "Count$Valid Creature.attackingYou"); !ok || got != 2 {
		t.Fatalf("Count$Valid Creature.attackingYou from seat 0 = (%d, %v), want (2, true)", got, ok)
	}
	// The other defender's source: the one attacker at them. A seat-blind
	// predicate (plain IsAttacking) would answer 3 here — that is the
	// "every attacker matches" widening the source-relative read must not
	// have.
	c1 := &Ctx{Controller: 1, Source: src1}
	if got, ok := EvalCountOK(h, c1, "Count$Valid Creature.attackingYou"); !ok || got != 1 {
		t.Fatalf("Count$Valid Creature.attackingYou from seat 1 = (%d, %v), want (1, true)", got, ok)
	}
	// The undifferentiated sibling counts all three declared attackers: the
	// two heads genuinely differ on this board.
	if got, ok := EvalCountOK(h, c, "Count$Valid Creature.attacking"); !ok || got != 3 {
		t.Fatalf("Count$Valid Creature.attacking = (%d, %v), want (3, true)", got, ok)
	}
	// No source at all fails closed to nobody, never to every attacker.
	cNoSrc := &Ctx{Controller: 0}
	if got, ok := EvalCountOK(h, cNoSrc, "Count$Valid Creature.attackingYou"); !ok || got != 0 {
		t.Fatalf("attackingYou with no source = (%d, %v), want (0, true)", got, ok)
	}
	// Blessed Reversal's spelling: the head with a /Times.<n> op suffix --
	// "You gain 3 life for each creature attacking you" -- still binds the
	// defender and still applies the op (2 x 3).
	if got, ok := EvalCountOK(h, c, "Count$Valid Creature.attackingYou/Times.3"); !ok || got != 6 {
		t.Fatalf("Count$Valid Creature.attackingYou/Times.3 = (%d, %v), want (6, true)", got, ok)
	}
}
