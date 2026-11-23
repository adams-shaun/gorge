package oracle

import "testing"

// CR 120.2 with CR 603.2c: a damage map that hits one player twice in one
// batch (Orcish Cannonade aimed at its own caster: 2 and 3 damage to the same
// player) is one simultaneous event, so a "whenever you're dealt damage"
// per-referent line (Darien, King of Kjeldor, DamageDoneOnce) triggers once
// with the total. The kernel fired it once per map entry and offered a second
// Darien order option (decision 503 of 503; tail-07-c000 pair 172).
func TestMBCDarienKingOfKjeldorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "damage_map_same_player_one_per_referent_trigger")
}
