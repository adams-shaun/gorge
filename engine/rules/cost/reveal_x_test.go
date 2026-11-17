package cost

import "testing"

// TestParseCostRevealX: Martyr of Sands' `1 Reveal<X/Card.White> Sac<1/CARDNAME>`
// reads as one dynamic Reveal part (Dyn X, Spec Card.White), the printed
// {1} stays generic, and nothing is reported unknown. The literal count form
// keeps its shape.
func TestParseCostRevealX(t *testing.T) {
	t.Parallel()
	c := ParseCost("1 Reveal<X/Card.White> Sac<1/CARDNAME>")
	if len(c.Reveal) != 1 || c.Reveal[0].Dyn != "X" || c.Reveal[0].Spec != "Card.White" || c.Reveal[0].N != 0 {
		t.Fatalf("Reveal = %+v, want one Dyn X Card.White part", c.Reveal)
	}
	if len(c.Unknown) != 0 || c.Generic != 1 || c.X != 0 || len(c.Sac) != 1 {
		t.Fatalf("Unknown=%v Generic=%d X=%d Sac=%v, want none/1/0/one sac", c.Unknown, c.Generic, c.X, c.Sac)
	}
	if got := ParseCost("Reveal<X/Card.Red;Card.Blue>"); len(got.Reveal) != 1 || got.Reveal[0].Spec != "Card.Red,Card.Blue" {
		t.Fatalf("OR alternation not folded: %+v", got.Reveal)
	}
	lit := ParseCost("Reveal<2/Card.White>")
	if len(lit.Reveal) != 1 || lit.Reveal[0].Dyn != "" || lit.Reveal[0].N != 2 {
		t.Fatalf("Reveal<2/Card.White> = %+v, want one counted part", lit.Reveal)
	}
	if rt := ParseCost(FormatCost(c)); len(rt.Reveal) != 1 || rt.Reveal[0].Dyn != "X" {
		t.Fatalf("round trip lost the dynamic reveal: %q -> %+v", FormatCost(c), rt.Reveal)
	}
}
