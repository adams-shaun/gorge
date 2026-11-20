package oracle

import "testing"

// CR 601.2g-h with CR 605.3a: Darksteel Forge's {9} is paid by p0's six lands
// plus three of the creatures Elven Chorus gives "{T}: Add one mana of any
// color}". Prismite, one of those creatures, also has its own repeatable
// "{2}: Add one mana of any color"; a generic cost gets no closer to paid by
// spending {2} for one mana, so the fee-only filter's colour ask offers no
// useful branch and gorge's potential script skips it
// (scriptSearch.feeOnlyAskPrune). The kernel once walked all five of its
// colours at every grant line and spent the 160-node budget before the grant
// plan, so its priority list omitted the cast (decision 545 of 545).
func TestMBCDarksteelForgeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "darksteel_forge")
}
