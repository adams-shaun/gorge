package oracle

import (
	"testing"
)

// CR 601.2g with CR 733.1: a convoke announcement whose committed creatures
// strand the cast's remaining mana payment is an illegal action reversed
// before any mana is activated, so the window's census must reach that
// conclusion without tapping anything. The board is kfuzz pair 252 of pool
// dr-mbc-baab (surface autopay, seed 648290): casting {2}{U} Referee Squad
// with convoke commits one creature for a generic, leaving {1}{U}, and the
// untapped sources are Cabal Pit#42 ({T}: Add {B}, lose 1 life), Forest#55
// ({G}) and Gilanra, Caller of Wirewood#56 ({T}: Add {G}, whose mana carries
// a bounded spend rider). No source can make {U}, so gorge's window census
// prices all three (the rider does not change what the activation floats),
// finds the payment unreachable and reverses the cast with nothing tapped.
// The kernel's dry walk once stopped at the rider source as an unpriced gap,
// opened the CR 601.2g window instead and activated all three sources --
// floating {B}{G}{G} and losing the 1 life Cabal Pit's activation costs --
// before reversing the cast, which CR 733.1's optional mana-ability reversal
// then kept (decision 357).
func TestMBCConvokeRiderCensusGapReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "convoke_rider_census_gap")
}
