package oracle

import "testing"

// A-Bruenor Battlehammer's {0} ability ("Attach target Equipment you control
// to target creature you control") is a Pump root whose SubAbility$ DB$ Attach
// carries Object$ ParentTarget: the ability fastens the ROOT TARGET (the
// Equipment) to a creature, so the CR 701.3a "another permanent" constraint
// binds the attached object, never the ability's source. Gorge's chain-target
// census once excluded the source (Bruenor itself, the only creature its
// controller then had) from the attach link's pool and withheld the payable
// activation the kernel offered (decision 119; tail-29 pair 3, the game the
// evidence loop filed under A-Bruenor Battlehammer). The census fix landed on
// main separately (a52b7ab37b); this recording keeps the offer pinned.
func TestMBCABruenorBattlehammerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "chain_attach_offer_excludes_source")
}
