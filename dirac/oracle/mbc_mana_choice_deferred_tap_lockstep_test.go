package oracle

import "testing"

// CR 605.3a with CR 603.3: a mana ability activated to pay a cost resolves
// immediately — the source is tapped, the mana floats, and a trigger on the
// tap ("whenever a player taps a land for mana", Manabarbs) fires there, so
// it is placed on the stack below the ability being paid for, together with
// the triggers of the window's choiceless taps. Dirac once deferred the
// resolution of the window's colour-choice tap (Unknown Shores, "add one mana
// of any colour") until after it pushed the ability, so that trigger landed
// above the ability while gorge kept it below (decision 305 of the recording,
// a Charix, the Raging Isle activation under an opponent's Manabarbs).
func TestMBCCharixManaChoiceDeferredTapReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mana_choice_deferred_tap")
}
