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

// riderAnswerOrder writes a rider ability's colour answers in the order
// gorge's mana ask lists them: a printed Combo's printed order
// (kop.Ability.ManaColOrd through manaColourOrder), then any other bit in
// W U B R G C order. A rider activation's answer index is a position in this
// list (deferGapAnswers counts it; manaRiderFloat decodes it).
func riderAnswerOrder(a *kop.Ability, out *[kop.NumManaKinds]uint8) int {
	n := 0
	var seen uint8
	mask := uint8(a.ManaColors)
	for o := a.ManaColOrd; o != 0 && n < kop.NumManaKinds; o >>= 3 {
		c := uint8(o & 7)
		if c >= uint8(kop.NumManaKinds) || seen&(1<<c) != 0 || mask&(1<<c) == 0 {
			continue
		}
		seen |= 1 << c
		out[n] = c
		n++
	}
	for c := uint8(0); c < uint8(kop.NumManaKinds); c++ {
		if mask&(1<<c) != 0 && seen&(1<<c) == 0 {
			out[n] = c
			n++
		}
	}
	return n
}

// riderAnswerColour maps a rider activation's answer index to the colour the
// ability's ask lists there (riderAnswerOrder); W when the index names no
// colour, as the mask's loop left it.
func riderAnswerColour(a *kop.Ability, answer int) int {
	var order [kop.NumManaKinds]uint8
	if n := riderAnswerOrder(a, &order); answer >= 0 && answer < n {
		return int(order[answer])
	}
	return 0
}

// riderAnswerIndex is colour col's position in the rider ask order
// (riderAnswerOrder): the engine-order answer manaRiderFloat decodes.
func riderAnswerIndex(a *kop.Ability, col int) int {
	var order [kop.NumManaKinds]uint8
	n := riderAnswerOrder(a, &order)
	for i := 0; i < n; i++ {
		if int(order[i]) == col {
			return i
		}
	}
	return 0
}
