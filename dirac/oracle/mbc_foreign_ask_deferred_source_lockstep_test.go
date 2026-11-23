package oracle

import "testing"

// TestMBCGogglesOfNightReprosLockstep replays Goggles of Night's Equip {2}
// beside an opponent's Relic Bind on the player's Talisman of Unity (tail-08,
// autopay, decision 716 of 716). The Talisman's tap fires the Aura's
// mode-asking watcher, so gorge's planner defers it and its prefix search ends
// at the opponent's ask: only a land is left to pay {2}, and gorge withholds
// the Equip. The kernel kept the Talisman's census unit (its two-ability
// production is no modelled gap) and offered it.
func TestMBCGogglesOfNightReprosLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	stdReproNamed(t, "foreign_ask_deferred_source_funds_no_activation")
}
