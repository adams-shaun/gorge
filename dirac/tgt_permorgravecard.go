package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// kop.TgtPermOrGraveCard, ChangeZone Origin$ Battlefield,Graveyard with its
// own ValidTgts$ (Aurelia's Vindicator: "exile up to X other target
// creatures from the battlefield and/or creature cards from graveyards").
// gorge changeZoneTargetZones names both zones; the one Filter serves both
// arms (matchPerm ignores its zone, the card matcher reads it).

// targetOKPermOrGraveCard: a permanent matching t.Filter, or a graveyard card
// matching it. The card matcher reads neither self word, so they apply here.
//
//go:noinline
func (s *State) targetOKPermOrGraveCard(k *prep, t *kop.TargetSpec, p uint8, src Slot, self Obj, o Obj) bool {
	ob := &s.objs[o]
	switch ob.zone {
	case kop.ZoneBattlefield:
		return s.matchPerm(k, t.Filter, ob.slot, p, src) && s.objTargetable(k, ob.slot, p)
	case kop.ZoneGraveyard:
		if s.graveCantTargetGen(k) {
			return false // Ground Seal's class (kw_canttgtgrave.go)
		}
		fl := k.t.Filters[t.Filter].Flags
		if o == self && fl&kop.FOther != 0 || o != self && fl&kop.FSelf != 0 {
			return false
		}
		return s.matchCardTgt(k, t.Filter, ob.card, ob.owner(), p, o, src, self)
	}
	return false
}

// permOrGraveCardCands lists the choices: the battlefield in slot order, then
// each graveyard in order (targetCands' TgtCard walk).
//
//go:noinline
func (s *State) permOrGraveCardCands(k *prep, t *kop.TargetSpec, p uint8, src Slot, self Obj, out *[MaxOptions]uint8, n int) int {
	for m := s.occ; m.any(); m = m.rest() {
		o := s.perms[m.first()].obj
		if s.targetOK(k, t, p, src, self, uint8(o)) {
			out[n] = uint8(o)
			n++
		}
	}
	for pp := 0; pp < int(s.nSeats()); pp++ {
		if s.out(uint8(pp)) {
			continue
		}
		pl := &s.pl[pp]
		for i := 0; i < int(pl.graveN); i++ {
			o := pl.grave[i]
			if uint8(o)+1 == s.g.tgtSelf {
				continue
			}
			if s.targetOK(k, t, p, src, self, uint8(o)) && n < MaxOptions-1 {
				out[n] = uint8(o)
				n++
			}
		}
	}
	return n
}
