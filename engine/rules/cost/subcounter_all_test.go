package cost

import "testing"

// TestParseCostSubCounterAll pins Forge's whole-pile counter removal
// SubCounter<All/Kind> (Jar of Eyeballs' "{3}, {T}, Remove all eyeball
// counters"): removing counters is a non-mana cost (CR 118.3), so the token
// adds no generic mana (CR 601.2f) and is not an unknown symbol. The part
// carries Dyn "All": its count is the counters of Kind on the source at
// payment, zero included.
func TestParseCostSubCounterAll(t *testing.T) {
	c := ParseCost("3 T SubCounter<All/EYEBALL>")
	if len(c.Unknown) != 0 {
		t.Errorf("Unknown = %v, want none", c.Unknown)
	}
	if c.Generic != 3 {
		t.Errorf("Generic = %d, want 3 (no phantom pip)", c.Generic)
	}
	if !c.Tap {
		t.Errorf("Tap = false, want true")
	}
	if len(c.SubCounter) != 1 {
		t.Fatalf("SubCounter = %+v, want one part", c.SubCounter)
	}
	part := c.SubCounter[0]
	if part.Dyn != SubCounterAll || part.Spec != "EYEBALL" || part.Announced || part.N != 0 {
		t.Errorf("SubCounter part = %+v, want Dyn %q kind EYEBALL", part, SubCounterAll)
	}
	// The source-anchored third field (Catti-brie's NICKNAME) is the same part.
	n := ParseCost("SubCounter<All/P1P1/NICKNAME>")
	if len(n.Unknown) != 0 || n.Generic != 0 || len(n.SubCounter) != 1 || n.SubCounter[0].Dyn != SubCounterAll {
		t.Errorf("NICKNAME form = %+v", n)
	}
}
