package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Ext Clash (kop.ExtClash; CR 701.31, gorge effClash in effects/clash.go):
// the resolving controller clashes with their opponent. Both reveal their top
// card; the controller's and then the opponent's own card is put on the top or
// the bottom of its owner's library (one ChGenClash ask each, in that order:
// APNAP, CR 101.4; a player with an empty library has no card and is not
// asked); and the player whose card has the strictly higher mana value wins (a
// tie, two empty libraries, or the controller's empty library, leaves no
// winner, as gorge reads an empty library as -1).
//
// The verdict is struck when the cards are revealed, before either card moves
// (gorge reads both mana values first), and is left in the resolution's result
// slot (g.roll, ExRollResult, the slot FlipCoin's branches read): 1 the
// controller won, 2 anything else. WinSubAbility$ / OtherwiseSubAbility$ lower
// as blocks guarded by it (kgen/lower_clash.go). The ask needs no scratch: the
// revealed card is still the top of the asked player's own library when it is
// answered, and the controller's ask is the only one whose answer is followed
// by another.
//
// The options are gorge's, bottom (mode 0) then top (mode 1), over the
// revealed card.

// clashOpp is the seat the controller p clashes with, noSeat when none is
// left: gorge's clashParticipants fallback, the first living player but p
// scanning from seat 0 (AliveFrom(0)), not from p.
func (s *State) clashOpp(p uint8) uint8 {
	if DebugClashNextSeat {
		if o := s.oppsFrom(p); o.n > 0 {
			return o.s[0]
		}
		return noSeat
	}
	q := s.aliveFrom(0)
	for i := uint8(0); i < q.n; i++ {
		if q.s[i] != p {
			return q.s[i]
		}
	}
	return noSeat
}

// DebugClashNextSeat is the mutation switch: the controller clashes with the
// next living seat after it, not gorge's first from seat 0. Tests only.
var DebugClashNextSeat bool

// clashMV is the mana value of the top card of seat p's library, -1 for an
// empty library.
func (s *State) clashMV(k *prep, p uint8) int {
	pl := &s.pl[p]
	if pl.libLen() == 0 {
		return -1
	}
	o := pl.lib[pl.libLo]
	return s.objMV(k, s.objs[o].card, o)
}

// clashExt is OpExt ExtClash; false: a placement ask is posed.
//
//kmanifest:ext ExtClash
func (s *State) clashExt(k *prep, in *kop.Instr) (ok, done bool) {
	p := s.res.e.ctrl
	mine, theirs := s.clashMV(k, p), -1
	if o := s.clashOpp(p); o != noSeat {
		theirs = s.clashMV(k, o)
	}
	s.g.roll = 2
	if mine >= 0 && mine > theirs {
		s.g.roll = 1
	}
	return s.clashAsk(p), true
}

// clashAsk poses the placement ask of seat p, or of the opponent when p's
// library is empty and p is the controller; true: nothing is left to ask.
func (s *State) clashAsk(p uint8) bool {
	if s.pl[p].libLen() > 0 {
		s.res.ask, s.res.askP = raGenClash, p
		return false
	}
	if p == s.res.e.ctrl {
		if o := s.clashOpp(p); o != noSeat {
			return s.clashAsk(o)
		}
	}
	return true
}

// clashOptions poses gorge's options, bottom then top.
func (s *State) clashOptions(d *Decision) {
	d.Sub = ChGenClash
	pl := &s.pl[s.res.askP]
	top := pl.lib[pl.libLo]
	d.add(MakeAction(AkChoose, TCode(top), 0, 0))
	d.add(MakeAction(AkChoose, TCode(top), 0, 1))
}

// clashAnswer places the asked player's card, then asks the opponent when the
// controller answered.
func (s *State) clashAnswer(k *prep, a Action) {
	r := &s.res
	p := r.askP
	r.ask = raNone
	pl := &s.pl[p]
	if a.Mode() == 0 && pl.libLen() > 1 {
		o := pl.lib[pl.libLo]
		pl.libLo++
		s.touch(o)
		pl.libBottom(o)
	}
	next := true
	if p == r.e.ctrl {
		if o := s.clashOpp(p); o != noSeat {
			next = s.clashAsk(o)
		}
	}
	if next {
		s.continueRes(k, 0)
	}
}
