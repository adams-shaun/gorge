package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// tapExtraLive reports whether a tap for mana can bring extra mana on this
// board: some all-at-once source's own rest is in the table (manaEach) or a
// StatTapExtra static is in force. A table that merely prints a carrier
// (genState.anyTapExtra) but has none in force prices every tap at its own
// mana, so the walk's units and the offer's census agree. The carrier may be
// a battlefield permanent or a command-zone Vanguard (Sisay), so the walk is
// eachTapExtra's, not plain eachStatic's.
func (s *State) tapExtraLive(k *prep) bool {
	if k.gen == nil || !k.gen.anyTapExtra {
		return false
	}
	if k.manaEach {
		return true
	}
	live := false
	s.eachTapExtra(k, func(st statSrc) bool {
		if st.a.Kind == kop.AbStatic && st.a.Static.Kind == kop.StatTapExtra {
			live = true
		}
		return !live
	})
	return live
}
