package cards

import "testing"

// TestManaCostChroma pins the ChromaSource count head's per-symbol read
// Forge's CardLists.getTotalChroma counts each
// space-separated mana-cost symbol ONCE per query whose colour bits
// intersect the symbol's (ManaCostShard.isColor) -- so a hybrid or
// Phyrexian symbol is one symbol for a query naming either of its colours
// and one for the all-colours query, while colourless/snow/generic/X
// symbols count for none. The carrier shapes are the census's: "W/U",
// "2/W", "2R", "BG", "U/P", "GWP", "CW".
func TestManaCostChroma(t *testing.T) {
	all := ColourWhite | ColourBlue | ColourBlack | ColourRed | ColourGreen
	for _, tc := range []struct {
		cost string
		col  uint8
		want int32
	}{
		// White = W, W, W/U; Blue = W/U, U/P; every colour = the 4 symbols
		// once each -- deliberately NOT 5, which a per-letter count gives.
		{"2 W W W/U U/P", ColourWhite, 3},
		{"2 W W W/U U/P", ColourBlue, 2},
		{"2 W W W/U U/P", ColourRed, 0},
		{"2 W W W/U U/P", all, 4},
		// Forge's slashed, non-slashed and mono-hybrid spellings parse
		// identically (parseNonGeneric ORs char atoms; digits contribute
		// nothing), and each is one symbol.
		{"2R", ColourRed, 1},
		{"2/R", ColourRed, 1},
		{"2/W", all, 1},
		{"BG", ColourBlack, 1},
		{"BG", all, 1},
		{"GW", ColourGreen, 1},
		{"GW", ColourWhite, 1},
		{"GW", all, 1},
		// Phyrexian and the compleated tricolour are one symbol each.
		{"U/P", ColourBlue, 1},
		{"GWP", ColourWhite, 1},
		{"GWP", ColourGreen, 1},
		{"GWP", all, 1},
		// Colourless hybrid counts for its colour query, never for a
		// colourless one.
		{"CW", ColourWhite, 1},
		{"CW", all, 1},
		// Non-colour symbols count for nothing.
		{"2 C S X", all, 0},
		{"", all, 0},
		{"10", all, 0},
	} {
		if got := ManaCostChroma(tc.cost, tc.col); got != tc.want {
			t.Errorf("ManaCostChroma(%q, %08b) = %d, want %d", tc.cost, tc.col, got, tc.want)
		}
	}
}
