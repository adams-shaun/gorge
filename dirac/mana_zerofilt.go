package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A dynamic filter whose amount resolves to zero (Three Tree City's {2},{T}
// with no creature of the chosen type) is no census gap and no filter gap:
// gorge's census lists nothing of it (manaAbilityYieldsNothing), so the prefix
// search never funds its fee as a gap. It is still one of the source's mana
// abilities, though, and gorge's funding tap of the source (the free {T}: {C})
// poses the "choose a mana ability" ask whenever the floating pool pays the
// fee: the {C} answer, then the filter's, which pays the fee, taps the source,
// asks the colour and makes no mana (CR 605.3a, 107.4a). Each of those is a
// node of the search's 160-node budget (scriptSearch.visit counts a node
// before its dedup), so a board holding the source spends the budget the
// plain funding lines need: Vampire Hexmage's {B}{B} beside Starfleet
// Academy's {5},{T} filter was priced by the kernel in 95 nodes and by gorge
// in none within 160.

// zeroFiltAsk reports whether the funding tap of sl poses the ask between its
// free ability and a zero-amount dynamic filter the pool now pays for.
func (fs *filtSearch) zeroFiltAsk(k *prep, t *State, sl Slot) bool {
	if !k.gen.anyDynFilt || !k.gen.has(t.cardAt(sl), gfFilt) || fs.gaps.has(sl) {
		return false
	}
	a := t.filtAb(k, sl)
	if a.AFlags&kop.AfManaDynamic == 0 || t.filtUnits(k, sl) != 0 || a.ManaEach || comboFiltAb(a) {
		return false
	}
	if _, ok := t.filtPip(k, sl); ok {
		return false
	}
	if _, ok := t.filtRepeatPip(k, sl); ok {
		return false
	}
	if _, _, ok := t.filtHyb(k, sl); ok {
		return false
	}
	return t.filtPaid(k, fs.p, sl)
}

// zeroFiltFund branches the zero filter's answer to the ask zeroFiltAsk
// posed: its colour ask and one funded step per colour, the mana it makes
// none. The source is a covered one, so the step funds (funded+1) and leaves
// gapUsed as the path had it.
func (fs *filtSearch) zeroFiltFund(k *prep, t *State, sl Slot, funded int, gapUsed bool) bool {
	if !t.filtFixed(k, sl) && !fs.tick() {
		return false
	}
	if t.filtFixed(k, sl) {
		c := uint8(bits.TrailingZeros8(uint8(t.filtAb(k, sl).ManaColors)))
		return fs.try(k, t, filtStep{sl: sl, ans: 1 + c, gap: true}, funded+1, gapUsed)
	}
	var order [5]uint8
	n := manaColourOrder(t.filtAb(k, sl), &order)
	needFirst(order[:n], fs.need)
	for i := 0; i < n; i++ {
		if fs.try(k, t, filtStep{sl: sl, ans: 1 + order[i], gap: true}, funded+1, gapUsed) {
			return true
		}
	}
	return false
}
