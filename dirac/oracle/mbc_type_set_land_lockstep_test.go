package oracle

import "testing"

// A Mountain enchanted by the opponent's Contaminated Ground is a Swamp that
// lost its printed abilities, and the Aura's "whenever enchanted land becomes
// tapped" watcher makes gorge's payment planner defer it (a census gap). To
// cast Caldera Kavu with its {B}, gorge's prefix script taps it at priority
// and both players pass, so the life-loss trigger resolves before the cast.
// The kernel's Taps-deferred gap walk skipped a land-type-set land (its
// genLost and printed-ability-index gates), kept it as a plain unit and
// tapped it inside the cast, leaving the trigger above the spell (decision
// 173 of tail-55 pair 44: Life[0] 18 vs 20). The ticket named Skirsdag
// Supplicant; at this land the same pair and class implicate this game.
func TestMBCSkirsdagSupplicantReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "type_set_land_taps_deferred_gap")
}
