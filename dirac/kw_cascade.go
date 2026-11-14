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
// queueCascadeTriggers). The trigger carries the spell's paid X, bound while
// the spell is still on the stack: cascade reads the spell's mana value
// where the spell resolved, not here (cascade; CR 202.3b).
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
	sx := uint8(s.stackXMV(e.obj, 1)) // the spell's paid X, for the battlefield arm
	for i := 0; i < n; i++ {
		base := s.npend
		s.enqueue(e.obj, s.objs[e.obj].ver, e.card, abCascade, e.ctrl, e.obj, e.ctrl, mv)
		if s.npend > base {
			s.pend[base].x = objWord(sx)
		}
	}
}

// cascade is OpDiscover C=2/3. C=2: the controller's library cards are
// exiled from the top until a nonland card with mana value strictly below
// the trigger's; the others go to the library bottom in a random order
// (CR 702.85a; a Fisher-Yates over s.rng, as gorge's effCascadeResidue) and
// the found card becomes the remembered set. C=3: the remembered card still in exile (not cast) goes
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
	// CR 202.3b: the spell's mana value is read where the spell resolved, not
	// where the trigger was queued: printed value plus the announced X while
	// the spell is on the stack, plus the X the permanent it became kept
	// (gorge's src.X; bound on the trigger entry by queueCascade); the card
	// alone once the spell has left the stack for anywhere else (CR 107.3m
	// zeroes X in the fold).
	e := &s.res.e
	mv := int(k.cards[e.card].total)
	switch s.objs[e.obj].zone {
	case kop.ZoneStack:
		mv += s.stackXMV(e.obj, k.cards[e.card].cost.X)
	case kop.ZoneBattlefield:
		mv += int(e.x)
	}
	pl := &s.pl[e.ctrl]
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
	// CR 702.85a: a random order -- gorge's seeded Fisher-Yates, the same
	// IntN(i+1) draws from the last index down to 1.
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
