package oracle

import (
	"testing"
)

// TestMBCCastWindowSplitColourAskReprosLockstep replays the recorded
// dr-mbc-abbb game (pool deal focus59_00233_WBR vs focus59_00079_WBR, seed
// 3625844914334216298, pair 59 of seed 298377): p1 casts Walking Ballista
// ({X}{X}) with a lucky Gemstone Caverns (a luck counter, its arm one mana of
// any colour) and a Plains untapped. The lucky activation is the forge
// script's DB$ Mana chain, whose colour ask (askManaColor, the "mana_color"
// resume) is a canonical decision the oracle's manaChoiceResolving gate
// declines to answer -- so gorge's manual CR 601.2g window taps the Caverns
// and pauses on the ask. The kernel's cast window walked over the source and
// floated the arm's mana by rule (the lowest colour), finished the payment
// and reached priority while gorge waited on the ask: Walking Ballista's
// announced {X} was one above gorge's (Projection Stack[0].X, the opponent's
// Plains tapped).
func TestMBCCastWindowSplitColourAskReprosLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro("testdata/repro/ballista_x_split_colour_ask.repro.json")
	if err != nil {
		t.Fatal(err)
	}
	if r.Pool != "custom" {
		t.Fatalf("the recording plays pool %s, want custom", r.Pool)
	}
	stdReproNamed(t, "ballista_x_split_colour_ask")
}
