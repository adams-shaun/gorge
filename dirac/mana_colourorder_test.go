package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestManaColourPrintedOrder: manaColourOrder reads ManaColOrd for the
// membership ManaColors names; 0 is WUBRG order.
func TestManaColourPrintedOrder(t *testing.T) {
	pack := func(cs ...int) uint16 {
		var o uint16
		for i, c := range cs {
			o |= uint16(c) << (3 * uint(i))
		}
		return o
	}
	for _, tc := range []struct {
		name string
		mask kop.ColorMask
		ord  uint16
		want []uint8
	}{
		{"Abzan W B G", kop.ColW | kop.ColB | kop.ColG, 0, []uint8{kop.W, kop.B, kop.G}},
		{"Jeskai U R W", kop.ColW | kop.ColU | kop.ColR, pack(kop.U, kop.R, kop.W), []uint8{kop.U, kop.R, kop.W}},
		{"Mardu R W B", kop.ColW | kop.ColB | kop.ColR, pack(kop.R, kop.W, kop.B), []uint8{kop.R, kop.W, kop.B}},
		{"Sultai B G U", kop.ColU | kop.ColB | kop.ColG, pack(kop.B, kop.G, kop.U), []uint8{kop.B, kop.G, kop.U}},
		{"Temur G U R", kop.ColU | kop.ColR | kop.ColG, pack(kop.G, kop.U, kop.R), []uint8{kop.G, kop.U, kop.R}},
		{"single", kop.ColR, 0, []uint8{kop.R}},
		{"any", kop.ColAny, 0, []uint8{kop.W, kop.U, kop.B, kop.R, kop.G}},
		{"unnamed bits follow in WUBRG order", kop.ColW | kop.ColU | kop.ColR, pack(kop.R), []uint8{kop.R, kop.W, kop.U}},
	} {
		var out [5]uint8
		n := manaColourOrder(&kop.Ability{ManaColors: tc.mask, ManaColOrd: tc.ord}, &out)
		if n != len(tc.want) {
			t.Errorf("%s: %d colours %v, want %v", tc.name, n, out[:n], tc.want)
			continue
		}
		for i, c := range tc.want {
			if out[i] != c {
				t.Errorf("%s: order %v, want %v", tc.name, out[:n], tc.want)
				break
			}
		}
	}
}
