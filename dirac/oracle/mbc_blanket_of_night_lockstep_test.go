package oracle

import "testing"

const blanketOfNightRepro = "blanket_of_night_grant_over_a_counter_cost_land_unmodelled"

// CR 305.6: Blanket of Night makes every land a Swamp, and a land with a basic
// land type has the intrinsic "{T}: Add {B}" beside its own abilities. Remote
// Farm's own ability ({T}, remove a depletion counter: Add {W}{W}) is no plain
// one-mana tap, so gorge offers both productions of the one land. The landgrant
// pair unit now models the intrinsic {B} beside the land's own counter-cost
// ability as two alternatives (dirac/static_landgrantalt.go), and the recorded
// game (tail-06 pairs 19 and 59) replays clean on a raw table of its cards; the
// card is therefore no longer refused (dirac/support/unrefused.tsv), and this
// test pins the unrefusal's proof.
func TestMBCTail06ReprosLockstep(t *testing.T) {
	if landGrantReplay(t) {
		return
	}
	res := replayRawRepro(t, blanketOfNightRepro)
	if res.Div != nil {
		t.Fatalf("%s's recorded game diverges at %d: %s", blanketOfNightRepro, res.Div.At, res.Div)
	}
	if res.StallUnmodelled {
		t.Fatalf("%s's recorded game ends at an unmodelled stall: %q", blanketOfNightRepro, res.Stall)
	}
}
