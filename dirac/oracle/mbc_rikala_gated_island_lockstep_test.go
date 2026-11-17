package oracle

import "testing"

// Rikala, Homarid King with three tide counters defers every Island the
// planner would tap (its gated "taps an Island for mana" line), a land under
// Spreading Seas included, so gorge's planner cannot pay Incremental Growth's
// {3}{G}{G} from four Forests and a spent Chalice and its autopay surface
// leaves the plain cast out (the prefix search that would pay it is no autopay
// route). The kernel kept the Spreading Seas land (an Island whose own
// abilities a land-type setter took) as an ordinary census unit, counted a
// fifth mana and offered the cast (dr-mbc-abab pair 107, decision 497).
func TestMBCIncrementalGrowthReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "rikala_gated_island_not_a_script_gap")
}
