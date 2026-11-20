package oracle

import "testing"

// The first divergence of the tail-75 evidence pool, pair 32 (seed 938630,
// full surface, gorge leading): p0 casts Enrage ({X}{R}) with the pool holding
// a floated {R} and {G}, the Mountain and Forest tapped for them, and one
// untapped Desolate Mire ("{1}, {T}: Add {W}{B}") that p1's Wolfwillow Haven
// enchants. CR 601.2b announces X before CR 601.2g's window opens, and the
// window can make four mana: the Mire's fee is funded from the floated {G},
// its tap adds {W}{B}, and its Static$ True TapsForMana trigger (the aura's
// "its controller adds an additional {G}") adds another {G} in the same
// resolution (CR 605.1b, 605.4a). {3}{R} is payable, so X = 3 is announceable;
// the kernel offered choose:x 0..3, gorge offered 0..2 -- its cast-window fold
// carried a tap trigger's extra mana onto single-symbol alternatives only, so
// the Mire's {W}{B} alternative lost the {G}.
func TestMBCEnrageReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "enrage_wolfwillow_x")
}
