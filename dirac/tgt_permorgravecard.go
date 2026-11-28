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
		return s.matchPermTgt(k, t.Filter, ob.slot, p, src) && s.objTargetable(k, ob.slot, p)
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

// permOrGraveCardCands lists the choices: the battlefield in gorge's offer
// order, then each graveyard in order (targetCands' TgtCard walk).
//
//go:noinline
func (s *State) permOrGraveCardCands(k *prep, t *kop.TargetSpec, p uint8, src Slot, self Obj, out *[MaxOptions]TCode, n int) int {
	n = s.permTgtCands(k, t, p, src, self, out, n) // tgt_permorder.go
	for pp := 0; pp < int(s.nSeats()); pp++ {
		if s.out(uint8(pp)) {
			continue
		}
		pl := &s.pl[pp]
		for i := 0; i < int(pl.graveN); i++ {
			o := pl.grave[i]
			if o+1 == s.g.tgtSelf {
				continue
			}
			if s.targetOK(k, t, p, src, self, TCode(o)) && n < MaxOptions-1 {
				out[n] = TCode(o)
				n++
			}
		}
	}
	return n
}
