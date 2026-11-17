package oracle

import "testing"

// TestCabalpitWithheldActivateReprosLockstep is the Cabal Pit recording of
// seed 134 of the deferred-land loop game's own decks: beside the controller's
// live Price of Glory every land is Taps-deferred, and the kernel withheld the
// Pit's {B},{T}, sacrifice activation at a priority decision where gorge
// offered it. The root is the script search's gap chain: it held the play's
// own source (the Pit's tap is a Defer-class census gap beside the watcher),
// so the chain's first tap spent the very {T} the play's cost spends and the
// play was unlisted at every depth -- dead end, no offer. The chain never
// holds the play's own tap-cost source (mana_tapsdefer.go deferGapsForPlay,
// hideDeferred), so the script taps a deferred Swamp instead and the Pit is
// offered, as gorge's planner plans it (CR 602.2b).
//
// The recording's own game goes on to a second withholding of the same class:
// a Taps-deferred gap whose tap's pain alone reaches the payer's life (the
// Pit at 1 life) is no branch of any completing script (painUnit's guard).
// The Pit replaying its {B} damage at 1 life ends every branch of the search
// before Quicksilver Amulet's {4},{T} activation lists, so the second
// recording pins that half of the fix.
func TestCabalpitWithheldActivateReprosLockstep(t *testing.T) {
	stdReproNamed(t, "activation_script_skips_the_plays_own_tap_cost_source")
	stdReproNamed(t, "activation_script_skips_a_pain_lethal_deferred_gap")
}
