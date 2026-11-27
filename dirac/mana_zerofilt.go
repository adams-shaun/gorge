package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A dynamic filter whose amount resolves to zero (Three Tree City's {2},{T}
// with no creature of the chosen type) is no census gap and no filter gap:
// gorge's census lists nothing of it (manaAbilityYieldsNothing), so the prefix
// search never funds its fee as a gap. It is still one of the source's mana
// abilities, though, and gorge's funding tap of the source (the free {T}: {C})
// poses the "choose a mana ability" ask whenever the floating pool pays the
// fee: the ask is a node of the search's potentialScriptNodes budget (scriptSearch.visit
// counts a node before its dedup), so a board holding the source spends one
// node the plain funding lines do not (CR 605.3a, 107.4a).

// zeroFiltAsk reports whether the funding tap of sl poses the ask between its
// free ability and a zero-amount dynamic filter the pool now pays for. The
// ask is all the search spends on it: the zero filter's own answer is no
// branch, because it yields nothing -- paying its fee only taps the source for
// no mana -- and gorge's search prunes exactly that answer from the ask's
// list (scriptDropEmptyManaAnswers; an announced source-counter tap stays one,
// the idle storage land's branch, and that battery is no gfFilt filter here).
// Without the ask the search and gorge's part at the budget's edge: gorge
// spends the node and offers the play, the kernel's plain funding lines end a
// node early and it does not (Shiv's Embrace's {R} pump at decision 745).
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
