package oracle

import "testing"

// Dominate ({X}{1}{U}{U}, instant, gain control of target creature with mana
// value X or less) beside Ancient Ziggurat (creature spells only): the kernel
// offers cast(Dominate) at decision 419 where gorge does not (tail-39 pair 54
// at land). The recording plays on the table built from its own decks (pool
// "custom"): a synthetic pool's membership moves with every lowering, and two
// earlier recordings dealt from tail-39 failed the replay on a deck card the
// pool had dropped or gained before they reached the decision.
func TestMBCDominateReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "dominate_x_spell_offered")
}

// CR 700.8: Cascade Seer (a Wizard) beside Game-Trail Changeling (every
// creature type) is a party of two, so its scry 2 orders two cards. The
// matchup's subtype table kept none of the four party roles (nothing named
// them in a filter), so the kernel saw the Seer fill no role and counted a
// party of one, offering one card to arrange where gorge offered two
// (decision 203, pair 350 of tail-39).
func TestMBCPartyRolesReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "party_roles_kept_in_matchup")
}
