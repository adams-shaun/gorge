package oracle

import "testing"

// CR 601.2b-c: Shattering Blow's targets are announced before its mana
// abilities are activated (CR 601.2g), so its {1}{R/W} is payable by tapping
// a Zoetic Cavern for {C} and sacrificing the Treasure token that is also
// the spell's only artifact target (the spell then resolves with an illegal
// target, CR 608.2b). gorge's potential walk prices the cast at priority,
// where that sacrifice leaves no target, so the potential payment's witness
// does not reach the play and no keeping plan exists: gorge offers pass
// only ("unsupported witness_unreachable"). The kernel withholds the plain
// cast the same way through plainHandPlayLowers' plainSacKeepsTarget
// (mana_sacplaintarget.go): the pin fails if that gate stops stranding the
// target (decision 190 of 190; pool tail-07-c000, seed 256884).
func TestMBCShatteringBlowReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "plain_cast_pays_with_its_only_target")
}
