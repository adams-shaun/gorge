package oracle

import "testing"

// CR 701.39a (bolster) with CR 701.66a (earthbend): an earthbent land is a
// 3/3 creature, so it is among the creatures tied for the least toughness
// that Dromoka's Gift may put its counters on. gorge read the printed type
// line and left the Forest out of the election dirac offered (decision 571
// of tail-08 pair 4, before the tail pools were re-bucketed; the recording
// replays on the table built from its own decks).
func TestMBCTail08ReprosLockstep(t *testing.T) {
	if landGrantReplay(t) {
		return
	}
	stdReproNamed(t, "bolster_elects_an_earthbent_land")
	dryadIlysianGroveRefused(t)
}
