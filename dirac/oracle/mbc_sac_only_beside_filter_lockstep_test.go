package oracle

import "testing"

// Cao Ren, Wei Commander ({2}{B}{B}) is castable only through Artifact Unknown
// Shores' paid "{1},{T}: Add one mana of any color" — its fee from a tapped
// Island, one generic from an Eldrazi Scion token's sacrifice-only "Sacrifice
// this creature: Add {C}" (CR 605.1a: a mana ability; gorge's census prices
// it as a last-resort unit, so the filter beside it is still a filter-only
// board and its relaxed alternative funds the cast). The kernel registered the
// sacrifice-only ability as a census gap (gfGapOther), which took the filter
// off the filtScript path: the cast was never offered. The recording is the
// evidence loop's tail-10 game (seed 3248101647261936343, decisions to 468).
func TestMBCCaoRenWeiCommanderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sac_only_beside_filter")
}
