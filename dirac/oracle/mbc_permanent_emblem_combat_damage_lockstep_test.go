package oracle

import "testing"

// CR 603.3b with CR 114.4: Kaito Shizuki's emblem ("Whenever a creature you
// control deals combat damage to a player, search ...") is permanent, so two
// of its owner's creatures dealing combat damage together queue two emblem
// triggers and their controller orders them. The kernel only fired the
// emblems that end at cleanup, queued none, and posed priority where gorge
// posed the trigger_order ask (decision 927; Chillerpillar and Gray Merchant
// of Asphodel were the attackers).
func TestMBCChillerpillarReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "permanent_emblem_combat_damage_trigger")
}
