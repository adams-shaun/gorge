package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// DivideEvenly$ RoundedDown (Fireball): the X damage is divided evenly among
// the targets still legal as the spell resolves, rounded down, each target
// taking the quotient (Forge's DamageDealEffect divides by the resolving
// target list). Before the key was read each target took the whole X.
func TestDealDamageDivideEvenlyRoundedDown(t *testing.T) {
	h := newHost(t, 2)
	src := battlefield(t, h, damageCarrier)
	opp := h.g.AddObject(mkCard(t, "Name:Opp Bear\nTypes:Creature\nPT:9/9\nOracle:x\n"), 1)
	h.g.Obj(opp.ID).Zone = state.ZBattlefield
	gone := h.g.AddObject(mkCard(t, "Name:Left Bear\nTypes:Creature\nPT:9/9\nOracle:x\n"), 1)
	// precondition: the dead target is not on the battlefield, so it is
	// not one of the targets the damage is divided among
	if h.g.Obj(gone.ID).Zone == state.ZBattlefield {
		t.Fatal("precondition: the departed target is on the battlefield")
	}
	h.g.Players[1].Life = 20
	line := "SP$ DealDamage | ValidTgts$ Any | NumDmg$ 7 | DivideEvenly$ RoundedDown"
	c := &Ctx{Source: src, Controller: 0, Targets: []state.Target{
		{Player: 1, IsPlayer: true}, {Obj: opp.ID}, {Obj: gone.ID},
	}, TargetsOffered: true}
	Resolve(h, c, sa(t, line))
	// 7 over the two legal targets: 3 each, 1 lost to rounding
	if h.g.Players[1].Life != 17 {
		t.Fatalf("player life = %d, want 17 (3 of 7 divided over 2 legal targets)", h.g.Players[1].Life)
	}
	if got := h.g.Obj(opp.ID).Damage; got != 3 {
		t.Fatalf("creature damage = %d, want 3", got)
	}
	if got := h.g.Obj(gone.ID).Damage; got != 0 {
		t.Fatalf("departed target damage = %d, want 0", got)
	}

	// no legal target: nothing to divide by, nothing dealt
	h.g.Players[1].Life = 20
	c = &Ctx{Source: src, Controller: 0, TargetsOffered: true}
	Resolve(h, c, sa(t, line))
	if h.g.Players[1].Life != 20 {
		t.Fatalf("player life = %d, want 20 with no target", h.g.Players[1].Life)
	}
}
