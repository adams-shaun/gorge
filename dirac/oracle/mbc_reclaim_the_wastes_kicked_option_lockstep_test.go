package oracle

import "testing"

// At p1's priority (tail-45 focus pair 7, decision 366 of 366) the kernel
// offered cast(#69{Reclaim the Wastes},-,1) -- the kicked {3}{G} cast -- while
// gorge offered only pass and the land play. The board's mana is five Plains
// and a repeatable fee-only filter (Signpost Scarecrow's "{2}: Add one mana of
// any color"), so the cast is legal and payable: two Plains pay the filter's
// fee, the filter makes {G}, and three Plains pay the {3}. gorge's plain
// prefix search walked past that funding line and exhausted its 160-node
// budget; the phased fallback (funding the fee before the gap) found it, but
// its scope was a cost that sacrifices or returns a permanent, so the
// non-consuming kicked cast stayed unpriced (census_incomplete) and only the
// kernel offered it. The fallback now takes a non-consuming play when the
// plain run budget-limited. The recording replays on its own pool's table.
func TestMBCReclaimTheWastesReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reclaim_the_wastes_kicked_option")
}
