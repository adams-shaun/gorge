package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// manaColourOrder writes a's ManaColors in the printed order ManaColOrd
// records (gorge's askManaColor; effects.ComboColours), as WUBRG indexes.
// 0 = WUBRG order. Mask bits the field does not name follow in WUBRG order
// (defensive; the lowering always names the whole mask).
func manaColourOrder(a *kop.Ability, out *[5]uint8) int {
	n := 0
	var seen uint8
	mask := uint8(a.ManaColors) & 0x1f
	for o := a.ManaColOrd; o != 0 && n < 5; o >>= 3 {
		c := uint8(o & 7)
		if c > 4 || mask&(1<<c) == 0 || seen&(1<<c) != 0 {
			continue
		}
		seen |= 1 << c
		out[n] = c
		n++
	}
	for c := uint8(0); c < 5; c++ {
		if mask&(1<<c) != 0 && seen&(1<<c) == 0 {
			out[n] = c
			n++
		}
	}
	return n
}
