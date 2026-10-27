package cost

import "testing"

// TestParseCostDiscardXIsAnnounced: Gix's `4 B B B Discard<X/Card/card>` reads
// as one Announced Discard part (Spec Card) and nothing is reported unknown;
// the literal Discard<2/Card> keeps its counted shape.
func TestParseCostDiscardXIsAnnounced(t *testing.T) {
	t.Parallel()
	c := ParseCost("4 B B B Discard<X/Card/card>")
	if len(c.Discard) != 1 || !c.Discard[0].Announced || c.Discard[0].Spec != "Card" || c.Discard[0].N != 0 {
		t.Fatalf("Discard = %+v, want one Announced Card part", c.Discard)
	}
	if len(c.Unknown) != 0 || c.Generic != 4 || c.X != 0 {
		t.Fatalf("Unknown=%v Generic=%d X=%d, want none/4/0", c.Unknown, c.Generic, c.X)
	}
	if c := ParseCost("Discard<X/Creature;Land/card>"); len(c.Discard) != 1 || c.Discard[0].Spec != "Creature,Land" || !c.Discard[0].Announced {
		t.Fatalf("OR alternation not folded: %+v", c.Discard)
	}
	c = ParseCost("1 Discard<2/Card>")
	if len(c.Discard) != 1 || c.Discard[0].Announced || c.Discard[0].N != 2 || len(c.Unknown) != 0 {
		t.Fatalf("Discard<2/Card> = %+v unknown=%v, want one counted part", c.Discard, c.Unknown)
	}
}
