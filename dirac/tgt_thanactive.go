package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The Oath cycle's player target (Oath of Druids: ValidTgts$
// Player.OpponentToActive+withMoreCreaturesThanActive): a TgtPlayer slot whose
// candidates are narrowed against the ACTIVE player, not the ability's
// controller. gorge evaluates the same words in engine/effects/
// player_than_active.go, every comparison strict (a tie names nobody).
// kop.TgfOppToActive keeps the opponents of the active player;
// kop.TgfThanMask selects the quantity compared.

// playerThanActiveOK reports whether seat pp passes the TgfOppToActive /
// ThanActive reading of tf against the active seat.
func (s *State) playerThanActiveOK(k *prep, tf kop.TargetFlags, pp uint8) bool {
	a := s.active
	if tf&kop.TgfOppToActive != 0 && pp == a {
		return false
	}
	what := int(tf&kop.TgfThanMask) >> kop.TgfThanShift
	if what == 0 {
		return true
	}
	if a >= MaxSeats || pp >= MaxSeats {
		return false
	}
	switch what {
	case kop.ThanCreatures:
		return s.thanActiveCount(k, pp, kop.TCreature) > s.thanActiveCount(k, a, kop.TCreature)
	case kop.ThanLands:
		return s.thanActiveCount(k, pp, kop.TLand) > s.thanActiveCount(k, a, kop.TLand)
	case kop.ThanHandCards:
		return s.pl[SeatIx(pp)].handN > s.pl[SeatIx(a)].handN
	case kop.ThanLife:
		return s.pl[SeatIx(pp)].life > s.pl[SeatIx(a)].life
	case kop.ThanYardFewer:
		return s.yardCreatures(k, pp) < s.yardCreatures(k, a)
	}
	return false
}

// thanActiveCount counts seat p's battlefield permanents of type ty after
// continuous effects (CR 613.1d), the layered types gorge's
// player_than_active.go reads.
func (s *State) thanActiveCount(k *prep, p uint8, ty kop.TypeMask) int {
	n := 0
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		if s.typesAt(k, m.first())&ty != 0 {
			n++
		}
	}
	return n
}

// yardCreatures counts the creature cards in seat p's graveyard.
func (s *State) yardCreatures(k *prep, p uint8) int {
	pl := &s.pl[SeatIx(p)]
	n := 0
	for i := 0; i < int(pl.graveN); i++ {
		if k.t.Cards[s.objs[pl.grave[i]].card].Types&kop.TCreature != 0 {
			n++
		}
	}
	return n
}
