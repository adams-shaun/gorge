package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestCastPumpOwnCombatantsReadsTheTargetScope pins the A6a target-scope fact:
// a pump whose ValidTgts$ cannot name both the caster's own attackers and
// blockers answers false, so the timing rule holds it instead of aiming at a
// creature the card cannot target. The bonus stays literal for a narrowed
// scope; only an opponent-only scope reads 0/0, the shape that leaves the
// card ungoverned by the pump rule.
func TestCastPumpOwnCombatantsReadsTheTargetScope(t *testing.T) {
	cases := []struct {
		name      string
		validTgts string
		att, def  string
		power     int32
		tough     int32
		own       bool
	}{
		{name: "unrestricted creature", validTgts: "Creature", att: "+3", def: "+3", power: 3, tough: 3, own: true},
		{name: "own creatures", validTgts: "Creature.YouCtrl", att: "+3", def: "+3", power: 3, tough: 3, own: true},
		{name: "own team", validTgts: "Creature.YourTeamCtrl", att: "+3", def: "+3", power: 3, tough: 3, own: true},
		{name: "creature alternative is enough", validTgts: "Artifact,Creature.YouCtrl", att: "+1", def: "0", power: 1, own: true},
		{name: "attacking only", validTgts: "Creature.attacking", att: "+3", def: "+3", power: 3, tough: 3},
		{name: "blocking only", validTgts: "Creature.blocking+YouCtrl", att: "+2", def: "+2", power: 2, tough: 2},
		{name: "non-attacker only", validTgts: "Creature.!attacking", att: "+2", def: "+2", power: 2, tough: 2},
		{name: "defending player's creatures", validTgts: "Creature.DefendingPlayerCtrl", att: "+2", def: "+2", power: 2, tough: 2},
		{name: "blocked creatures", validTgts: "Creature.blocked+YouCtrl", att: "+2", def: "+2", power: 2, tough: 2},
		{name: "colour restriction", validTgts: "Creature.Green", att: "+2", def: "+2", power: 2, tough: 2},
		{name: "opponent only", validTgts: "Creature.OppCtrl", att: "+3", def: "+3"},
		{name: "you dont control", validTgts: "Creature.YouDontCtrl", att: "+2", def: "+2"},
		{name: "non-creature target", validTgts: "Artifact", att: "+1", def: "0"},
	}
	for _, c := range cases {
		sa := &cards.SA{API: "Pump", Params: map[string]string{
			"ValidTgts": c.validTgts, "NumAtt": c.att, "NumDef": c.def,
		}}
		power, tough := CastPumpBonus(sa)
		if power != c.power || tough != c.tough {
			t.Errorf("%s: CastPumpBonus = %d/%d, want %d/%d", c.name, power, tough, c.power, c.tough)
		}
		if got := CastPumpOwnCombatants(sa); got != c.own {
			t.Errorf("%s: CastPumpOwnCombatants = %v, want %v", c.name, got, c.own)
		}
	}
}
