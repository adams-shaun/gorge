package oracle

import "testing"

// Sword of Fire and Ice's Equip {2} was offered as an activation by gorge but
// not by the kernel at p1's priority decision 723 (tail-55-c000 pair 4,
// seed 830205, recorded at 0463f27a15): the potential fold of a
// reflected-amount source beside it (Incubation Druid) read canTapForManaPC,
// which rejects every census-gap source (its manaAb is < 0), so the fold was
// dropped and the activation priced unaffordable. The fold reads gapLive now
// (mana_reflamt.go), and the game replays clean.
func TestMBCSwordOfFireAndIceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reflamt_gap_liveness_offer")
}
