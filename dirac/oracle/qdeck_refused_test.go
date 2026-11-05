package oracle

import "testing"

// TestQRefusedCopyCastTaps pins the paid copy cast Q's exile grant cannot pay:
// gorge begins the cast, its mana window taps the offered source, the pool
// stays short and the cast is reversed with the tap standing (CR 733.1). The
// lockstep reaches such activations on the autopay surface (even seeds); with
// the kernel latching StallCopyCast before the tap, every seed that activates
// one stalls or diverges on the tapped land's projection.
func TestQRefusedCopyCastTaps(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, acts := runQDeck(t, 32, false)
	if acts[qdeckUnpayable] == 0 {
		t.Fatalf("no unpayable granted activation in %d games: %v", games, acts)
	}
	if n := acts["Hill Giant"] + acts["Shock"]; n == 0 {
		t.Fatalf("no granted exile activation in %d games", games)
	}
	t.Logf("%d games, %d unpayable activations, %d divergences, %d stalls", games, acts[qdeckUnpayable], divs, stalls)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
}
