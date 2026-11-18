package oracle

import "testing"

// Far // Away (a fuse split card) in hand: a half cast alone is only that
// half (CR 709.3a/709.3b), so Pillar of the Paruns' "spend this mana only to
// cast a multicolored spell" mana (CR 106.6) pays for neither half alone,
// though the card in hand is blue and black (CR 709.4).
//   - far_fuse_castface_offer: gorge offered castface(Away) where dirac
//     offered only pass.
//   - split_half_multicolor_mana: with Pillar and one Island untapped, gorge
//     offered the front half, Far ({1}{U}), where dirac did not (decision 680
//     of tail-72-c000 pair 12, seed 164453).
func TestMBCFarReprosLockstep(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"far_fuse_castface_offer", "split_half_multicolor_mana"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			stdReproNamed(t, name)
		})
	}
}
