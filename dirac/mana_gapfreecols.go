package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A free gap activation whose source makes more than one colour (a filter
// land that also taps for an intrinsic granted basic type's mana: Shimmering
// Grotto under an Island grant makes {C} or {U}) is an ask in gorge's
// scriptSearch.visit: one node for the ask, and a branch per answer in the
// ask's order. The step used to take the source's lowest colour alone, with
// no ask node and no second branch, so the plain run's budget (160 nodes)
// ran out in gorge on a subtree the mirror never walked.

// gapFreeOrder lists the colours of mask that gap source sl's free ask offers,
// in gorge's order: the printed free ability's colours (its {C} included, which
// manaColourOrder does not name) before the granted ones, the play's needed
// colours moved first (needFirst).
func (fs *filtSearch) gapFreeOrder(k *prep, t *State, sl Slot, pc *pcard, mask uint8, out *[kop.NumManaKinds]uint8) int {
	n := 0
	var seen uint8
	if cd := &k.t.Cards[t.cardAt(sl)]; pc.manaAb >= 0 && int(pc.manaAb) < len(cd.Abilities) {
		a := &cd.Abilities[pc.manaAb]
		var ord [5]uint8
		for _, c := range ord[:manaColourOrder(a, &ord)] {
			if mask&(1<<c) != 0 && seen&(1<<c) == 0 {
				out[n] = c
				seen |= 1 << c
				n++
			}
		}
		if uint8(a.ManaColors)&(1<<kop.C) != 0 && mask&(1<<kop.C) != 0 && seen&(1<<kop.C) == 0 {
			out[n] = kop.C
			seen |= 1 << kop.C
			n++
		}
	}
	for m := mask &^ seen; m != 0; m &= m - 1 {
		out[n] = uint8(bits.TrailingZeros8(m))
		n++
	}
	needFirst(out[:n], fs.need)
	return n
}

// gapFreeTry activates gap source sl's free ability: one step when it makes
// one colour, else the colour ask (a node, unless asked: the filter's own ask
// already listed the free abilities) and a step per answer.
func (fs *filtSearch) gapFreeTry(k *prep, t *State, sl Slot, funded int, asked bool) bool {
	mask := t.srcMaskAt(t.manaPCGen(k, sl), sl)
	if bits.OnesCount8(mask) <= 1 {
		return fs.try(k, t, filtStep{sl: sl, gap: true}, funded, true)
	}
	if !asked && !fs.tick() {
		return false
	}
	var order [kop.NumManaKinds]uint8
	for _, c := range order[:fs.gapFreeOrder(k, t, sl, t.manaPCGen(k, sl), mask, &order)] {
		if !fs.subsumeKeeps(t, c) {
			continue // the subsume run's ask keeps only the colours the pool lacks
		}
		if fs.try(k, t, filtStep{sl: sl, gap: true, col: c + 1}, funded, true) {
			return true
		}
	}
	return false
}
