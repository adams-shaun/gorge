package oracle

import "testing"

const bogardanPhoenixRepro = "bogardan_phoenix_granted_tap_gold_budget"

// CR 602.2b, 605.3a: Bogardan Phoenix ({2}{R}{R}{R}) is payable from a
// Mountain, two Forests, Painted Bluffs ({T}: {C}; {1}, {T}: one mana of any
// color) and a Gold token (sacrifice: one mana of any color) once Jaheira,
// Friend of the Forest gives the token "{T}: Add {G}". That granted tap makes
// the Gold token a census gap beside the Bluffs, and gorge's plain prefix
// search needs 372 nodes against its potentialScriptNodes budget, so the play
// is left "unsupported census_incomplete" and the full surface withholds it
// while the kernel prices and offers it (tail-76 pair 183, decision 291,
// "options options"). That was the divergence the card was refused for. At the
// recorded decision both engines now offer the cast of Bogardan Phoenix, and
// the recorded game replays clean on a table of its own cards, so the card is
// unrefused with this game as its proof (dirac/support/unrefused.tsv).
func TestMBCBogardanPhoenixReprosLockstep(t *testing.T) {
	t.Parallel()
	oneOffReproClean(t, bogardanPhoenixRepro)
}
