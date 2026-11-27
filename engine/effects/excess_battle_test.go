package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

const (
	excessBattleHit = "Name:Excess Battle Hit\nTypes:Instant\n" +
		"SVar:Hit:DB$ DealDamage | ValidTgts$ Battle | NumDmg$ 5 | ExcessSVar$ Excess\nOracle:x\n"
	excessBattleSiege = "Name:Excess Siege\nTypes:Battle Siege\nDefense:5\nOracle:x\n"
)

// excessBattleOf resolves a 5-damage hit with ExcessSVar$ against a Battle
// carrying `defense` defense counters and returns the published binding.
func excessBattleOf(t *testing.T, defense int32) (string, bool) {
	t.Helper()
	h := newHost(t, 2)
	card := mkCard(t, excessBattleHit)
	face := card.Faces[0]
	source := h.g.AddObject(card, 0)
	source.Zone = state.ZBattlefield
	battle := h.g.AddObject(mkCard(t, excessBattleSiege), 1)
	battle.Zone = state.ZBattlefield
	battle.AddCounter("DEFENSE", defense)
	if f := battle.Face(); f == nil || !f.IsBattle() || h.IsCreature(battle.ID) {
		t.Fatalf("precondition: want a non-creature Battle, got face %v", f)
	}
	if got := battle.Counter("DEFENSE"); got != defense {
		t.Fatalf("precondition: defense counters %d, want %d", got, defense)
	}
	c := &Ctx{Source: source.ID, Controller: 0, SVars: face.SVars,
		PickedTargets: []state.Target{{Obj: battle.ID}}}
	Resolve(h, c, svarSA(t, face, "Hit"))
	v, ok := c.SVars["Excess"]
	return v, ok
}

// TestExcessSVarBattleDefense: CR 120.4a/120.10 -- the excess of damage to a
// battle is the damage over its defense. 5 damage to a 5-defense battle is 0
// excess (Bottle-Cap Blast makes no Treasure), to a 3-defense battle it is 2.
func TestExcessSVarBattleDefense(t *testing.T) {
	for _, tc := range []struct {
		defense int32
		want    string
	}{{5, "0"}, {3, "2"}, {7, "0"}} {
		got, ok := excessBattleOf(t, tc.defense)
		if !ok || got != tc.want {
			t.Fatalf("defense %d: Excess = %q (bound %v), want %q", tc.defense, got, ok, tc.want)
		}
	}
}
