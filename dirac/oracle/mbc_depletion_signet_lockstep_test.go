package oracle

import "testing"

// CR 605.3a with CR 602.2b: Peat Bog's "{T}, remove a depletion counter: add
// {B}{B}" makes two mana, one of which pays Gruul Signet's {1} fee for {R}{G},
// so Peat Bog and the Signet together make three mana and Dragon Throne of
// Tarkir's Equip {3} is activatable. gorge offered the re-equip; the kernel
// did not (tail-58 pair 0, decision 442).
func TestMBCDragonThroneOfTarkirReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "depletion_gap_funds_signet_fee_equip")
}
