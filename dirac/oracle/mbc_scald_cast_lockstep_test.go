package oracle

import "testing"

// CR 307.1, 602.5d, 704.5a: p1 sits at about two life with Scald in play and four
// Islands, three Mountains and Overflowing Basin untapped, and Laquatus's Creativity
// ({4}{U}, a sorcery) in hand. One Island tap (one Scald damage) beside the other lands
// pays for it, so gorge offers the cast; dirac's Taps-deferred search dropped it because
// the lines that tap every Island lose the game first.
func TestMBCLaquatusSCreativityReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "scald_islands_cast_at_low_life")
}
