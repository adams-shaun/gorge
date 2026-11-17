package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// digPickable: o, a card of a Dig's looked-at window, is a pick. A copy that
// has ceased to exist there (CR 707.10h; Spin into Myth's fateseal after the
// target, an Offspring token copy, went on top) matches no filter, the empty
// ChangeValid$ filter included, so it is not a pick (gorge's matchesObjectText
// IsCopy rejection); matchCardGenAt reads f == 0 as match-all before it asks
// copyCeased.
func (s *State) digPickable(k *prep, f kop.FilterID, o Obj, ctrl uint8) bool {
	if s.g.tli&tliOn == 0 && s.objs[o].of&ofCopy != 0 && s.copyCeased(k, o) {
		return false
	}
	return s.matchCardGen(k, f, s.objs[o].card, s.objs[o].owner(), ctrl, o) && s.pickFits(k, f, o, true)
}
