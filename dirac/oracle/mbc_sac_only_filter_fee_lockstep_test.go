package oracle

import "testing"

// CR 605.3b with CR 602.2: a mana ability with a mana cost ({1}, {T}: Artifact
// Unknown Shores' one mana of any color) is paid from other mana, and an
// Eldrazi Scion's "Sacrifice this creature: Add {C}" (no {T}) is one of
// gorge's last-resort census alternatives, not a census gap beside the
// filter. Archangel Elspeth ({2}{W}{W}) beside a Plains, two Swamps, the
// Shores and a Scion is payable: the Scion's {C} funds the Shores' fee for the
// second {W}. The kernel read the Scion as a gap that is no filter, so the
// board was not filter-only, ran no filter search and left the cast out.
// The recording is the first divergence of the evidence loop's tail-10 pair
// 199 (options options at decision 224, gorge leading), dealt before the
// evidence files moved the pool's decks.
func TestMBCArchangelElspethReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sac_only_mana_beside_filter_fee")
}
