package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A funding source with a restricted any-colour ability and a reflected one
// (Plaza of Heroes: {T}: Add {C}; {T}: Add one mana of any colour, spent only
// on a legendary spell; {T}: Add one mana of any colour among legendary
// permanents you control). gorge's prefix search (rules scriptSearch.visit)
// activates the source and branches the "choose a mana ability" ask in printed
// order: the {C} (one node), the restricted any colour (its colour ask next,
// then each colour) and the reflected ability (its colour ask only when it
// makes more than one colour, then each colour). The ability ask is the only
// ask between the free {C} and the reflected colours: the kernel's census view
// of the source is the union {C} | the reflected colours (mana_reflected.go),
// whose own colour ask would charge a node gorge's search has no ask for.

// fundReflected is fundTap's arm for a source of that shape: the {C}, the
// restricted ability's answers (rm; their colour ask first), then the reflected
// colours, the play's announced colours first as the restricted answers are
// (gorge's scriptOrderAnswers on the same colour ask, CR 601.2b).
func (fs *filtSearch) fundReflected(k *prep, t *State, sl Slot, rm uint8, funded int, gapUsed bool) (res, stop bool) {
	if fs.try(k, t, filtStep{sl: sl, ans: kop.C}, funded+1, gapUsed) {
		return true, true
	}
	if bits.OnesCount8(rm) > 1 && !fs.tick() {
		return false, true // the restricted ability's colour ask
	}
	if fs.restrictedAnswers(k, t, sl, rm, funded, gapUsed) {
		return true, true
	}
	rc := uint8(t.reflColours(k, sl, t.cardAt(sl))) &^ (1 << kop.C)
	if bits.OnesCount8(rc) > 1 && !fs.tick() {
		return false, true // the reflected colour ask
	}
	var order [kop.NumManaKinds]uint8
	n := 0
	for m := rc; m != 0; m &= m - 1 {
		order[n] = uint8(bits.TrailingZeros8(m))
		n++
	}
	needFirst(order[:n], fs.need)
	for _, c := range order[:n] {
		if fs.try(k, t, filtStep{sl: sl, ans: c}, funded+1, gapUsed) {
			return true, true
		}
	}
	return false, false
}
