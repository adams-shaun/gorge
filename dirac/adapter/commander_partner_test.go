package adapter

import "testing"

// TestCommanderPairGate: a seat's two commanders must be a legal partner pair
// (CR 903.3, 702.124) of two distinct deck cards; the bench decks' first two
// cards are no pair, and a repeated index seats one commander, not two.
func TestCommanderPairGate(t *testing.T) {
	e, c := benchCommander(t)
	for _, pair := range [][]int{{0, 1}, {0, 0}} {
		cp := c
		cp.Commanders = [][]int{pair, c.Commanders[1]}
		if _, err := e.New(cp); err == nil {
			t.Errorf("commanders %v were accepted as a partner pair", pair)
		}
	}
}

// TestCommanderConfigWidens: both indices of a partner pair reach dirac's
// Config, and a one-commander seat leaves the second slot empty.
func TestCommanderConfigWidens(t *testing.T) {
	e, c := benchCommander(t)
	c.Commanders = [][]int{{3, 5}, {7}}
	cfg := e.config(c)
	if cfg.Commanders[0] != [2]int16{3, 5} || cfg.Commanders[1][0] != 7 || cfg.Commanders[1][1] != -1 {
		t.Fatalf("Config.Commanders %v, want {3 5} {7 -1}", cfg.Commanders[:2])
	}
}
