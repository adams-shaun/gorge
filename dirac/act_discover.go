package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// discover is OpDiscover (gorge effDiscover / effDiscoverBottom). C=0: the
// player's library cards are exiled from the top until a nonland card with
// mana value A or less (printed, X = 0), the others go to the library
// bottom in a random order drawn from the engine rng (CR 701.57a, exactly
// gorge's Fisher-Yates), and the found card becomes the remembered set
// (replacing it). C=1: the remembered card still in exile goes to its
// owner's hand (the declined or uncastable discovery).
func (s *State) discover(k *prep, in *kop.Instr) {
	if in.C == 2 || in.C == 3 {
		s.cascade(k, in)
		return
	}
	if in.C == kop.DiscoverDugBottom {
		s.dugBottom(k) // act_discoverbottom.go
		return
	}
	if in.C == kop.DiscoverRemBottom {
		s.remBottom() // act_discoverbottom.go
		return
	}
	if in.C == 1 {
		for i := 0; i < int(s.g.nrem); i++ {
			if o := s.g.rem[i]; s.objs[o].zone == kop.ZoneExile && !s.freeQueued(o) {
				from := s.detach(o)
				s.placeFrom(o, kop.ZoneHand, false, from, false)
			}
		}
		if k.gen != nil && k.gen.anyKwAct {
			p := s.playerRef(k, in.Obj)
			if p >= MaxSeats {
				p = s.res.e.ctrl
			}
			s.fireKwAct(k, kop.TrigDiscover, p, int16(s.value(k, in.A))) // trig_modes.go
		}
		return
	}
	p := s.playerRef(k, in.Obj)
	if p >= MaxSeats {
		p = s.res.e.ctrl
	}
	n := s.value(k, in.A)
	pl := &s.pl[p]
	var ex [MaxDeck]Obj
	ne := 0
	found := NoObj
	for pl.libLo < pl.libHi {
		o := pl.lib[pl.libLo]
		pl.libLo++
		s.touch(o)
		s.objs[o].zone = kop.ZoneNone
		s.placeFrom(o, kop.ZoneExile, false, kop.ZoneLibrary, false)
		pc := &k.cards[s.objs[o].card]
		if !pc.land && int(pc.total) <= n {
			found = o
			break
		}
		ex[ne] = o
		ne++
	}
	for i := ne - 1; i > 0; i-- {
		j := s.rng.IntN(i + 1)
		ex[i], ex[j] = ex[j], ex[i]
	}
	for _, o := range ex[:ne] {
		from := s.detach(o)
		s.placeFrom(o, kop.ZoneLibrary, true, from, false)
	}
	s.g.nrem, s.g.remCnt = 0, 0
	if found != NoObj {
		s.remember(found)
	}
}

// payManaResume goes on with a resolution's mana payment after a source's order ask.
func (s *State) payManaResume(k *prep) {
	c := s.resPayCost(k) // cost_trighybrid.go
	paid := s.payReplanned(k, s.g.pickP, &c)
	if s.parkedWaiting() {
		s.ctl = ctlExileOrder
		return
	}
	s.payDone(k, paid)
}
