package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A wide any-colour restricted fee gap (Orb of Dragonkind's "{1}, {T}: Add two
// mana in any combination of colors", Dragon spells and abilities only) in the
// restricted fee prefix (restrFeePrefix). Gorge's script search answers the
// source's colour ask with the first pair of colours after which the plain walk
// lists the play, the pairs in the order scriptOrderAnswers gives (the play's
// announced colours first), exactly as restrComboPrefix does for a Great Hall's
// combo pair: a prefix that floated the first colour twice paid a {1}{R} pump
// with no red (CR 106.6, 601.2b, 605.3b).

// restrFeeWideAb reports whether the fee gap ability ab makes two mana of any
// combination of colours.
func restrFeeWideAb(ab *kop.Ability) bool {
	return ab.AFlags&kop.AfManaChoice != 0 && ab.ManaN == 2 && ab.ManaColors&kop.ColAny == kop.ColAny
}

// restrFeeActivateWide activates the wide fee gap sl as gorge's search does:
// the fee paid from the pool, then each pair of colours floated in answer order
// until the plain walk lists play a, else the first answer.
func (s *State) restrFeeActivateWide(k *prep, p uint8, a Action, sl Slot, ab *kop.Ability, fee int) {
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	dc, dn := s.deferSnap(p)
	s.filtFeeSpend(k, p, sl, fee)
	if k.artMana {
		s.artSync(p)
	}
	s.deferSync(p, dc, dn)
	need := s.playNeed(k, a)
	pl := &s.pl[SeatIx(p)]
	rb, pool := s.rb[SeatIx(p)], pl.pool
	var d Decision
	for c := 0; c <= wideAnswers; c++ {
		// the last pass re-floats the first answer when none lists the play
		x, y := wideAnswer(wideEngineAnswer(need, c%wideAnswers))
		s.rb[SeatIx(p)], pl.pool = rb, pool
		if !s.floatRestrAb(k, p, sl, ab, x, 1) || !s.floatRestrAb(k, p, sl, ab, y, 1) {
			s.capFull(StallRestrSrc) // never a silent spill
			return
		}
		if c == wideAnswers {
			return
		}
		s.plainOptions(k, &d, availNoActScript|availNoFeeUnit)
		if hasOption(&d, a) {
			return
		}
	}
}
