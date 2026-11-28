package oracle

import "testing"

const sacManaActivationRepro = "sac_mana_funds_sorcery_sac_land_activation"

// CR 602.2b with CR 605.3b: Surtland Frostpyre's {2}{U}{U}{R}, {T}, Sacrifice
// activation is payable when Diamond Kaleidoscope's Prism token ("Sacrifice a
// Prism token: Add one mana of any colour") is the only red source, so the
// kernel offers it on Plains, Plains, Island, Island and a summoning-sick Prism
// (tail-12 pair 27, seed 7910311743838239922, decision 876: only the kernel's
// menu carries activate(#76)). With Vigil for the Lost out, the sacrificed
// Prism queues its pay-X trigger, and gorge poses that ask with a third, manual
// route (trigger_cost_pay_manual) that opens a window over every untapped mana
// source: gorge's prefix search spends its potentialScriptNodes budget there
// before it reaches the red answer, and offers no activation. That was the
// divergence the card was refused for. At the recorded decision both engines
// now offer the activation of Surtland Frostpyre, and the recorded game replays
// clean on a table of its own cards, so the card is unrefused with this game as
// its proof (dirac/support/unrefused.tsv).
func TestMBCSurtlandFrostpyreReprosLockstep(t *testing.T) {
	t.Parallel()
	oneOffReproClean(t, sacManaActivationRepro)
}
