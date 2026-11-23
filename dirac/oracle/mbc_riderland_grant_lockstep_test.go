package oracle

import "testing"

// CR 605.3b: Peat Bog's own "{T}, Remove a depletion counter: Add {B}{B}" and
// the "{T}: Add one mana of any color" Abundant Growth grants it are one tap's
// alternatives. With only Peat Bog and a Plains untapped the board makes
// {B}{B}{W} or {G}{W}, so Primal Growth's kicked {2}{G} is not a play. The
// kernel's per-tap census widened the two into one {B,any}x2 unit that paid
// {G}{G} and offered the kicked cast gorge left out (decision 168 of
// dr-mbc-bbbb pair 22, autopay).
func TestMBCPrimalGrowthReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "rider_land_grant_is_one_tap")
}
