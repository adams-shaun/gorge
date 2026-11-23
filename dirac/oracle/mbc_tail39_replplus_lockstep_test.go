package oracle

import "testing"

// CR 616.1: the combat damage a red source one player controls would deal to
// the other player is matched by two non-commuting replacements. The
// recipient controls Tok-Tok, Volcano Born ("If a red source would deal
// damage to a player, it deals that much damage plus 1 to that player
// instead") and the source's controller controls Absorbing Man and Titania
// ("Double all damage that creature sources you control would deal"). The
// order decides the amount (2(n+1) against 2n+1), so gorge asks the affected
// player. dirac's walk collected candidates only from the source controller's
// battlefield and stalled on the recipient's Tok-Tok (decision 641 of pool
// tail-39 pair 4).
func TestMBCTail39ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "tail39_replplus")
}
