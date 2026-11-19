package oracle

import "testing"

// gorge's prefix search spends its 160-node budget on a funding source's ask
// even when the source's dynamic filter makes nothing: Three Tree City ("{2},
// {T}: add as much of one colour as you control creatures of the chosen type")
// with Mystic chosen and no Mystic out sits in the ask beside its free {C}
// while the pool holds its fee, and its colour ask branches five colours that
// each continue the search with the fee spent. The kernel's mirror skipped
// those nodes, found Omen of the Sun's {2}{W} sacrifice-scry through Mana
// Flare's doubled Swamps and Starfleet Academy's {5},{T} filter within the
// budget, and offered the activation gorge's search never reaches (dr-mbc-abbb
// pair 36, decision 709). The recording is that game as a custom table.
func TestMBCOmenOfTheSunReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "dynamic_filter_unchosen_type_count")
}
