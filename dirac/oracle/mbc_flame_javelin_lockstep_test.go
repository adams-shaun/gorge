package oracle

import "testing"

// CR 608.2b, 701.8b: Relic Crush destroys two targets at once, and one of
// them is an Aura (Felidar Umbra) while the other is a creature (Crashing
// Drawbridge). The kernel's leave-together path leaves both in one event and
// panics in fireLeave when it looks up the Aura's attachment with the "no
// card" sentinel (255) as an index into a 192-entry table (decision 505,
// dr-mbc-bbaa pair 25).
func TestMBCFlameJavelinReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "flame_javelin")
}
