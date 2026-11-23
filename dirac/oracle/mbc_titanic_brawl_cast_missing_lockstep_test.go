package oracle

import "testing"

// Titanic Brawl (instant, {1}{G}) sat in p1's hand with a Treasure, an untapped
// Plains and Mountain, and a creature on each side, yet only one engine offered
// the cast (decision 360 of 360, tail-32-c001 pair 8).
func TestMBCTitanicBrawlReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "titanic_brawl_cast_missing")
}
