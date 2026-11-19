package oracle

import "testing"

// Confront the Past ({X}{B} sorcery, charm: return a planeswalker card with
// mana value X or less from your graveyard, or remove twice X loyalty from an
// opponent's planeswalker): the recorded game (tail-32-c006 pair 33) has the
// caster's one planeswalker, Tevesh Szat (mana value 5), in the graveyard and
// one black mana to spend. Two parts of the cast diverged. gorge offered the
// cast at decision 738 although no payable X (CR 601.2b, 601.2h) names that
// planeswalker (the Charm's X-bounded mode target was not priced against the
// payable X). At decision 942, with X affordable, dirac skipped the mode ask
// (its mode census judged the cmcLEX target at X = 0) where CR 601.2b asks the
// modes before the X announcement.
func TestMBCConfrontThePastReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "charm_announce_x_no_planeswalker_target")
}
