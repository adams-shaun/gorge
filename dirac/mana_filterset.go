package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A paid filter set (Koskun Keep: {T}: Add {C}, {1},{T}: Add {R},
// {2},{T}: Add {B} or {G}; Castle Sengir, An-Havva Township, Aysen Abbey,
// Wizards' School) is one "choose a mana ability" ask in gorge's prefix
// search: the free {C} answer, then one answer per colour of each paid
// ability whose fee the floating pool pays (CR 601.2b, 605.3b), in printed
// order ("Pay 1: Add R", "Pay 2: Add B", "Pay 2: Add G"). A choice
// ability's colours are options of that same ask, not a second one, and
// each names its colour as the option's mana symbol; the {C} answer and a
// fixed-colour ability's answer name none. The search's scriptOrderAnswers
// hoists the answers whose symbol is a colour the play needs ahead of the
// rest, engine order kept inside each half, so only a choice ability's
// needed colour moves (Black Knight's {B}{B} takes "Pay 2: Add B" before
// {C}; Shock's {R} keeps {C} before "Pay 1: Add R"). The empty {C} symbol
// keeps the ask out of the subsume run's and the one-shot prunes' scope
// (scriptManaUnits, engine/rules/potential_script.go).

// filtSetAnswer is one answer of a paid filter set's ask: the free {C}
// (free), or colour col of the card's filter ability at ordinal ab.
// sym: the answer names col as its mana symbol (a choice ability's).
type filtSetAnswer struct {
	ab   int8
	col  uint8
	free bool
	sym  bool
}

// filtSetGap walks the ask of the paid filter set at sl in gorge's answer
// order. The ask's own node is the caller's (visit's two-ability tick).
func (fs *filtSearch) filtSetGap(k *prep, t *State, sl Slot, funded int) bool {
	var list [1 + maxCardFilters*5]filtSetAnswer
	n := 0
	intrinsic := t.filtIntrinsicFree(k, sl) // a granted type's {c} lists after the printed abilities
	if !intrinsic {
		list[n] = filtSetAnswer{free: true}
		n++
	}
	for i, nc := 0, t.filtCount(k, sl); i < nc; i++ {
		ord := int8(i)
		if !t.filtPaidAt(k, fs.p, sl, ord) {
			continue // the pool does not pay this ability's fee: the ask does not list it
		}
		a := t.filtAbAt(k, sl, ord)
		choice := a.AFlags&kop.AfManaChoice != 0
		var order [5]uint8
		for _, c := range order[:manaColourOrder(a, &order)] {
			list[n] = filtSetAnswer{ab: ord, col: c, sym: choice}
			n++
		}
	}
	if intrinsic {
		list[n] = filtSetAnswer{free: true}
		n++
	}
	for pass := 0; pass < 2; pass++ {
		for _, a := range list[:n] {
			if needed := a.sym && fs.need[a.col]; needed != (pass == 0) {
				continue
			}
			if a.free {
				if fs.gapFreeTry(k, t, sl, funded, true) {
					return true
				}
				continue
			}
			if fs.try(k, t, filtStep{sl: sl, ans: 1 + a.col, gap: true, ab: a.ab}, funded, true) {
				return true
			}
		}
	}
	return false
}
