package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// mayUntilForgetExile marks a DurWhileGrave record whose effect forgets the
// card when it moves to exile (ForgetOnMoved$ Exile; Skyclave Shade's
// landfall grant). It rides the low bit of a limited grant's used count, so a
// DurWhileGrave record carries no cap (kgen lowers only an unlimited Effect
// to it).
const mayUntilForgetExile uint8 = 0x10

// mayPersistMoved re-arms an Effect-delivered graveyard may-play grant after
// its card returns to the graveyard: the record survives the cast and the
// card's zone changes, and this refreshes its version, as gorge re-reads the
// still-live effect static (CR 611.2a). A ForgetOnMoved$ Exile grant is
// dropped when the card moves to exile; every other destination leaves it
// for a later return. Called from placeFromK for the two zones a record's
// grant can name.
func (s *State) mayPersistMoved(o Obj, z kop.Zone) {
	if z != kop.ZoneGraveyard && z != kop.ZoneExile {
		return
	}
	n := int(s.g.nmay)
	w := 0
	for i := 0; i < n; i++ {
		r := *s.mayAt(i)
		if r.obj == o && r.dur() == kop.DurWhileGrave && r.kind&kop.MayPlayLimitMask == 0 {
			if z == kop.ZoneExile && r.until&mayUntilForgetExile != 0 {
				continue // ForgetOnMoved$ Exile: the effect forgets the card
			}
			if z == kop.ZoneGraveyard {
				r.ver = s.objs[o].ver
				*s.mayAt(i) = r
			}
		}
		if w != i {
			*s.mayAt(w) = *s.mayAt(i)
		}
		w++
	}
	s.g.nmay = uint8(w)
}
