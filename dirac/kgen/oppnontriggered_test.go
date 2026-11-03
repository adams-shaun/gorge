package kgen

import (
	"strings"
	"testing"
)

// DamageAll ValidPlayers$ OppNonTriggeredTarget in a combat-damage-to-an-
// opponent trigger names every opponent but the damaged one (gorge
// validPlayers), and in two players none is left: the node lowers to
// nothing. In a trigger whose damaged object may be a permanent it stays
// refused by name.
func TestOppNonTriggeredTargetTwoPlayers(t *testing.T) {
	const name = "U.S.S. Prometheus, Prototype"
	kp := kfhBuild(t, []string{name}, nil)
	kp.accepted(t, name)

	kp = kfhBuild(t, []string{name}, func(p *pool) {
		c := kfhClone(p.byName[name])
		sv := c.Faces[0].SVars
		sv["DamageTrig"] = strings.Replace(sv["DamageTrig"], "ValidTarget$ Opponent", "ValidTarget$ Card", 1)
		p.byName[name] = c
	})
	kp.refusedWith(t, name, "OppNonTriggeredTarget")
}
