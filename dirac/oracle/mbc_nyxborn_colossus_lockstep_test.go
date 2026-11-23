package oracle

import "testing"

// CR 106.1, 601.2f: Incubation Druid (three +1/+1 counters, so three mana of one
// type) is the only source beside two Forests that could fund Nyxborn Colossus's
// {3}{G}{G}{G}. gorge's PotentialMana bound prices the Druid's reflected Amount$
// in the colourless slot alone, so its bound never lists the cast and its prefix
// search never runs for it; the kernel searched every plain cast beside a live
// reflected Amount$ source and offered it at p1's priority decision 416 (tail-55
// pair 372). The kernel now searches only the plays gorge's bound pays.
func TestMBCNyxbornColossusReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reflamt_search_only_plays_gorge_bound_pays")
}
