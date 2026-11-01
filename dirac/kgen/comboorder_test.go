package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestManaColourPrintedOrder: a Produced$ Combo printed outside WUBRG order
// (the TDM Devotees) lowers with the same ManaColors mask and a ManaColOrd
// that records gorge's askManaColor order (effects.ComboColours); a Combo in
// WUBRG order keeps ManaColOrd zero.
func TestManaColourPrintedOrder(t *testing.T) {
	want := map[string][]int{
		"Abzan Devotee":  {kop.W, kop.B, kop.G},
		"Jeskai Devotee": {kop.U, kop.R, kop.W},
		"Mardu Devotee":  {kop.R, kop.W, kop.B},
		"Sultai Devotee": {kop.B, kop.G, kop.U},
		"Temur Devotee":  {kop.G, kop.U, kop.R},
	}
	names := []string{"Abzan Devotee", "Jeskai Devotee", "Mardu Devotee", "Sultai Devotee", "Temur Devotee"}
	all, bs, _, err := buildPool(lowerNamedPool(t, names))
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for i, c := range all {
		seq, ok := want[c.Name]
		if !ok {
			continue
		}
		seen++
		if len(c.Refusals) != 0 {
			t.Errorf("%s refused: %v", c.Name, c.Refusals)
		}
		a := bs[i].def.Abilities[0]
		var mask kop.ColorMask
		for _, x := range seq {
			mask |= 1 << x
		}
		if a.ManaColors != mask {
			t.Errorf("%s: ManaColors %v, want %v", c.Name, a.ManaColors, mask)
		}
		inWUBRG := c.Name == "Abzan Devotee"
		if inWUBRG != (a.ManaColOrd == 0) {
			t.Errorf("%s: ManaColOrd %#o, zero only for WUBRG order", c.Name, a.ManaColOrd)
		}
		var ord uint16
		for p, x := range seq {
			ord |= uint16(x) << (3 * uint(p))
		}
		if !inWUBRG && a.ManaColOrd != ord {
			t.Errorf("%s: ManaColOrd %#o, want %#o", c.Name, a.ManaColOrd, ord)
		}
	}
	if seen != len(want) {
		t.Errorf("built %d of %d cards", seen, len(want))
	}
}
