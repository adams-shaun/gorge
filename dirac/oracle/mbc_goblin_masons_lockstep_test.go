package oracle

import "testing"

// A filter land ({1}, {T}: one mana of any colour) funded by one mana of a
// land's multi-mana tap (Remote Farm's {W}{W}) prices Goblin Masons ({1}{R}):
// gorge offers the cast and the kernel did not (decision 72 of tail-51-c000
// pair 2).
func TestMBCGoblinMasonsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_land_funded_by_batch_mana")
}
