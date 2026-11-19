package oracle

import "testing"

// CR 605.1a, 601.2g: Whisperer of the Wilds ({T}: Add {G}, ferocious {T}: Add
// {G}{G}) under Cryptolith Rite (creatures tap for one mana of any colour)
// beside four Forests and two more creatures pays Goliath Spider's {6}{G}{G}:
// gorge's prefix search taps the Whisperer for its {G}{G} and the others for
// their granted mana (its "choose a mana ability" ask lists the pair's larger
// alternative second). dirac's grant search left that answer out and spent its
// 160 nodes on the Whisperer's {G} and its any colour, so the Spider was not
// offered (decision 293, tail-15-c002 pair 108).
func TestMBCGoliathSpiderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "gated_pair_beside_grant_prefix_ask")
}
