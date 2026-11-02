package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugIgnoreCascade disables the native cascade trigger (the oracle's
// mutation test: the lockstep must diverge without it).
var DebugIgnoreCascade bool

// cascadeProg is the resolution of a cascade trigger (CR 702.85a, gorge
// effects/cascade.go): exile until a cheaper nonland card (OpDiscover C=2),
// offer its free cast, then bottom it if it is still in exile (C=3).
var cascadeProg = [3]kop.Instr{
	{Op: kop.OpDiscover, Obj: kop.RefController, C: 2},
	{Op: kop.OpCastFree, Obj: kop.RefRemembered, C: int16(kop.ZoneExile)},
	{Op: kop.OpDiscover, Obj: kop.RefController, C: 3},
}

// queueCascade queues one cascade trigger per printed Cascade instance of
// the spell e just cast, after the ordinary cast triggers (gorge
// queueCascadeTriggers). The spell's mana value is fixed here: printed
// value plus the announced X (CR 202.3b).
func (s *State) queueCascade(k *prep, e *stackEntry) {
	n := int(k.t.Cards[e.card].Cascade)
	if k.gen.anyStackKw {
		g, _ := s.stackKwN(k, kop.StackKwCascade, e.card, e.obj, e.ctrl, s.castFromHandBy(e, e.ctrl)) // one more per granting static (cascadeInstances)
		n += g
	}
	if n == 0 || s.isSpellCopy(e.obj) {
		return
	}
	mv := int16(int(k.cards[e.card].total) + s.stackXMV(e.obj, k.cards[e.card].cost.X))
	for i := 0; i < n; i++ {
		s.enqueue(e.obj, s.objs[e.obj].ver, e.card, abCascade, e.ctrl, e.obj, e.ctrl, mv)
	}
}

// cascade is OpDiscover C=2/3. C=2: the controller's library cards are
// exiled from the top until a nonland card with mana value strictly below
// the trigger's; the others go to the library bottom in exile order (gorge's
// stand-in for CR's random order, GORGE-FINDINGS.md) and the found card becomes the
// remembered set. C=3: the remembered card still in exile (not cast) goes
// to the bottom.
func (s *State) cascade(k *prep, in *kop.Instr) {
	if in.C == 3 {
		for i := 0; i < int(s.g.nrem); i++ {
			if o := s.g.rem[i]; s.objs[o].zone == kop.ZoneExile && !s.freeQueued(o) {
				from := s.detach(o)
				s.placeFrom(o, kop.ZoneLibrary, true, from, false)
			}
		}
		return
	}
	mv := int(s.res.e.trigAmt)
	pl := &s.pl[s.res.e.ctrl]
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
		if !pc.land && int(pc.total) < mv {
			found = o
			break
		}
		ex[ne] = o
		ne++
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
