package oracle

import "testing"

// CR 115.5 with the kernel's target kinds: A-Hullbreaker Horror's "Whenever
// you cast a spell, choose up to one" trigger's first mode returns a spell you
// don't control to its owner's hand, and its script names the choice with
// TgtZone$ Stack beside ValidTgts$ Card.YouDontCtrl and no TargetType$ (Forge's
// rebalanced spelling). Gorge reads TgtZone$ Stack as the stack-spell target
// (its SpecTargetsStack rule) and offers the mode while the opponent's countered
// spell is still on the stack; kgen fell through changeZoneMove's stack arm to
// the generic targeted move, which lowered the slot as a CARD target whose
// candidate scan never walks the stack, so the kernel dropped the mode from the
// charm's "up to one" offer (decision 525 of the recorded game: gorge
// [done mode0 mode1], kernel [done mode0]; the game was dealing with Muddle the
// Mixture countering Cracked Earth Technique when the trigger fired).
func TestMBCMuddleTheMixtureReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "stack_spell_target_lowered_as_card")
}
