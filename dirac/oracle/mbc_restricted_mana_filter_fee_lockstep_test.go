package oracle

import "testing"

// CR 106.6: Somberwald Sage's "spend this mana only to cast creature spells"
// mana can't pay the {1} of Mossfire Valley's activated mana ability, so with
// Forest, Island, Mossfire Valley and the Sage untapped the most mana p0 can
// make is six and the seven-mana Axebane Stag is not castable (decision 306
// of tail-23 pair 88; dirac offered the cast).
func TestMBCAxebaneStagReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "restricted_mana_filter_fee")
}
