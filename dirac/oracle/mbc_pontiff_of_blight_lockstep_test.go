package oracle

import "testing"

// CR 118.12 with CR 106.6: Spell Contortion's "counter unless its controller
// pays {2}" ask is a non-spell payment, which a Powerstone's {C} ("can't be
// spent to cast a nonartifact spell") may fund. With a Swamp and a Powerstone
// untapped gorge offered pay and decline while the kernel counted only the
// Swamp and offered decline alone (decision 388 of tail-41 pair 12 of a
// Spell Contortion and Stone Retrieval Unit focus). The first recording, tail-41
// pair 38 (Pontiff of Blight's extort), named cards the pool has since lost.
func TestMBCPontiffOfBlightReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_pay_offer_powerstone")
}
