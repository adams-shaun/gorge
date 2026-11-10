package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// dropEOTEmblems removes the emblems that end at cleanup (kop.Emblem.
// UntilEOT: gorge's Effect without a Duration$), keeping the others in
// their order.
//
//go:noinline
func (s *State) dropEOTEmblems(k *prep) {
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		var drop uint8
		for i := 0; i < int(s.g.nemb[p]); i++ {
			if k.t.Emblems[s.embIdx(p, i)-1].UntilEOT {
				drop |= 1 << uint(i)
			}
		}
		if drop != 0 {
			s.embCompact(uint8(p), drop)
		}
	}
}

// fireEmblemsObj fires the emblems' triggers of object event ev (ETB, dies,
// leaves, sacrificed) for the permanent at evSl, after the battlefield
// listeners (as fireEmblems does for spells cast): each emblem trigger
// whose filter the permanent matches (for the emblem's owner; it names no
// source), queued as the creating card's latent copy with the permanent as
// its trigger object.
//
//go:noinline
func (s *State) fireEmblemsObj(k *prep, ev kop.TrigEvent, evSl Slot, evObj Obj) {
	for pp := uint8(0); pp < s.nSeats(); pp++ {
		if s.out(pp) {
			continue
		}
		for i := 0; i < int(s.g.nemb[pp]); i++ {
			e := &k.t.Emblems[s.embIdx(pp, i)-1]
			for j := range e.Abilities {
				ea := &e.Abilities[j]
				if ea.Kind != kop.AbTriggered || ea.Trig.Event != ev {
					continue
				}
				if e.Capture != kop.RefNone && !s.embRemHolds(pp, i, evObj) {
					continue // the trigger's ValidCard$ ...IsRemembered (static_emblemrem.go)
				}
				if e.Source && s.embObj(pp, i) != evObj {
					continue // ValidCard$ Card.EffectSource: the object that made the emblem
				}
				if ea.Trig.Filter != 0 && !s.matchPermGen(k, ea.Trig.Filter, evSl, pp, NoSlot, layerFull) {
					continue
				}
				c := s.embCard(pp, i)
				cd := &k.t.Cards[c]
				for ab := range cd.Abilities {
					la := &cd.Abilities[ab]
					if la.Kind == kop.AbTriggered && la.AFlags&kop.AfLatent != 0 && la.Trig.Event == ev && la.Trig.Zone == kop.ZoneCommand {
						o := s.embObj(pp, i)
						n0 := s.npend
						s.enqueueLKI(o, s.objs[o].ver+1, c, uint8(ab), pp, evObj, s.perms[evSl].ctrl, 0)
						if e.CaptureRem && s.npend > n0 {
							s.embRemCarry(pp, i, &s.pend[n0]) // trig_emblemrem.go
						}
						if e.Capture != kop.RefNone && s.objs[evObj].of&ofToken != 0 {
							s.embRemDrop(pp, i, evObj) // a token that died is gone for good: its id may name a later token (CR 111.7)
						}
						break
					}
				}
			}
		}
	}
}
