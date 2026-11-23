package oracle

import "testing"

// CR 601.2b, 601.2g, 605.3b: Enrage's {X}{R} is announced against a window
// whose floating {R} funds Desolate Mire's {1},{T}: {W}{B}, and the {W} funds
// Mystic Skull's {1},{T}: any colour -- {R} for the pip, the {B} pays X = 1.
// The X bound activated the filters in slot order only (the Skull first, its
// {1} paid by the {R}, then the Mire's fee by the Skull's mana, leaving no
// {R} for the pip), so it offered X = 0 alone where gorge also offers X = 1.
func TestMBCEnrageReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_order_x_bound")
}
