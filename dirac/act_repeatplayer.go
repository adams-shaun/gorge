package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ctrlEachPl is kop.CtrlEachPl's controller test:
// gorge's RememberedPlayerCtrl (effects/filter.go) inside a RepeatEach
// player body, where the Remembered player is the repeated one
// (effRepeatEach; PlayerCountRemembered$Valid, effects/count_cov3.go
// cov3PlayerGroupPlayerCountRemembered, counts through the same match).
// pc is the permanent's controller. s.g.eachPl is read only once this
// resolution started an OpRepeatEachPlayer (repEnd != 0); outside one the
// filter matches nothing.
func (s *State) ctrlEachPl(pc, ctrl uint8) bool {
	if DebugEachPlCtrlYou {
		return pc == ctrl
	}
	return s.g.repEnd != 0 && pc == s.g.eachPl
}

// DebugEachPlCtrlYou makes CtrlEachPl match the resolving controller instead
// of the repeated player. Test-only: the repeatpl lockstep's mutation proof.
var DebugEachPlCtrlYou bool

// sacForcedBatch sacrifices every match of one player's Sacrifice effect
// together (gorge effSacrifice moves a player's whole batch at once: a
// "whenever a player sacrifices another creature" listener sacrificed in the
// same batch still sees the rest, CR 603.10a), in battlefield order. A
// Pox Plague iteration sacrifices Zodiark with the permanents beside it.
func (s *State) sacForcedBatch(k *prep, f kop.FilterID, p uint8, src Slot) {
	var ord [MaxPerm]Slot
	n := s.battlefieldOrder(p, s.ctrlM[p], ord[:], 0)
	for _, sl := range ord[:n] {
		if s.matchPerm(k, f, sl, p, src) {
			s.addSac(sl)
		}
	}
	s.flushSac(k)
}

// sacPickBatch records one pick of a multi-pick Sacrifice ask (raSacrifice
// with two or more still to choose); the picks leave together after the
// last one, as sacForcedBatch. It reports whether the ask is complete.
func (s *State) sacPickBatch(k *prep, o Obj) bool {
	s.addSac(s.objs[o].slot)
	s.res.left--
	if s.res.left > 0 {
		return false
	}
	s.flushSac(k)
	return true
}
