package oracle

import "testing"

// TestMBCUrzaOwnManaPaysOwnActivationReprosLockstep replays the synthetic
// tail-64 game the lockstep run recorded: at turn 30 upkeep p1 holds priority
// beside four pushed abilities (three Mishra's Factory animates and Urza, Lord
// High Artificer's) and an untapped Jeweled Lotus, Rustvale Bridge, Construct
// token and Glint Raker. Urza's {5}: Shuffle your library, then exile the top
// card… activation (no {T} in its 2026 printing) is payable only by activating
// its own "Tap an untapped artifact you control: Add {U}" repeatedly, five uses
// over the five untapped artifacts, so gorge's payment planner — which prices
// Urza's mana ability at most once per plan (one {U} beside Rustvale Bridge's
// printed {1}) — declines it, and the kernel's open any-mana walk must not
// offer it either (tail-64 pair 5, decision 851; the play stays reachable
// across priorities by floating the mana manually). The kernel's plain
// activations are priced against the payment's potential bound
// (actBoundDrops, CR 106.6), which counts each source's printed mana once and
// so cannot afford it either.
func TestMBCUrzaOwnManaPaysOwnActivationReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "urza_own_mana_pays_own_activation")
}
