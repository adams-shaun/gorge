package oracle

import "testing"

// Confront the Past ({X}{B} sorcery, charm: return a planeswalker card with
// mana value X or less from your graveyard, or remove twice X loyalty from an
// opponent's planeswalker): the recorded game (tail-32-c006 pair 33) has the
// caster's one planeswalker, Tevesh Szat (mana value 5), in the graveyard and
// one black mana to spend. The recording pins decision 942: with X affordable,
// dirac skipped the mode ask (its mode census judged the cmcLEX target at
// X = 0) where CR 601.2b asks the modes before the X announcement. The
// gorge-side offer at decision 738 (no payable X names that planeswalker) is
// pinned by TestCharmXBoundModeTargetNeedsPayableX in engine/rules.
func TestMBCConfrontThePastNoPlaneswalkerTargetReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "charm_announce_x_no_planeswalker_target")
}
