package oracle

import "testing"

// CR 701.39a: bolster's election reads the CURRENT (layer-derived)
// creature-ness and toughness, so a land animated by Verdant Touch ("Target
// land becomes a 2/2 creature ... this effect lasts indefinitely") with the
// least toughness on the battlefield is a legal candidate, ahead of the
// printed creatures. gorge's putCounterBolster once filtered its candidates
// with the printed type line (hasType), so the animated 2/2 lands were
// skipped and the election ran over the printed 5/5 creatures instead -- gorge
// offered counter_pick over Oran-Rief Ooze and Sandsteppe Mastodon where the
// kernel offered the two animated Forests (decision 584 of 586; the game the
// evidence loop filed under Oran-Rief Ooze, tail-74-c000 pair 32).
func TestMBCOranRiefOozeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "bolster_candidates_read_printed_types")
}
