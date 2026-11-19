package oracle

import "testing"

// CR 106.12a, 605.1b: an opponent's Overgrowth on the player's Reflecting Pool
// adds {G}{G} whenever the Pool taps, so Mountain, Swamp and the enchanted Pool
// pay Tiamat's Fanatics' {4}{R}. Gorge's potential bound read the reflected
// ability's blank Produced$ and dropped that extra, withholding the cast the
// kernel offers (dr-mbc-aaaa pair 181, decision 163).
func TestMBCTiamatSFanaticsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "overgrowth_on_reflecting_pool_funds_the_cast")
}
