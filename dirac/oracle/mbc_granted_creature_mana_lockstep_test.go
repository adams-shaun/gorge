package oracle

import "testing"

// CR 601.2g-h, 605.1a: Song of Freyalise's chapter I grants every creature
// "{T}: Add one mana of any color"; with three lands and four untapped
// creatures the five mana of Arrow Storm are payable, and both engines must
// offer the cast (decision 593 of 593; dr-mbc-bbab pair 63).
func TestMBCArrowStormReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "granted_creature_mana_cast_window_reach")
}
