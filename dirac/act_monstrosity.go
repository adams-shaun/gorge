package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Monstrosity N (CR 701.37), the monstrous designation
// (gorge Object.Monstrous, the events.AlterAttribute "Monstrous" fold) and
// the IsMonstrous filter word. Cold and out of line: a table without an
// OpMonstrosity or a TrigBecomeMonstrous never takes these paths
// (genPrep.anyMonstrosity).

// DebugIgnoreMonstrosity makes the kernel read every permanent as not
// monstrous. The lockstep mutation test flips it and requires a divergence.
var DebugIgnoreMonstrosity bool

// prepMonstrosity sets anyMonstrosity: some program becomes monstrous or
// some trigger watches for it. The mask clear on leave and the op are then
// live; a table without either keeps them off the hot paths.
func prepMonstrosity(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			g.anyMonstrosity = g.anyMonstrosity || a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigBecomeMonstrous
			for _, in := range a.Prog {
				g.anyMonstrosity = g.anyMonstrosity || in.Op == kop.OpMonstrosity
			}
		}
	}
}

// monstrousAt reports whether the permanent at sl is monstrous.
func (s *State) monstrousAt(sl Slot) bool {
	if DebugIgnoreMonstrosity {
		return false
	}
	return s.mSl(s.k(), mechMonstrous, sl)
}

// monstrosity is OpMonstrosity (gorge effPutCounter's MonstrositySet arm):
// a permanent that is not monstrous gets A +1/+1 counters through
// OpPutCounter's path (the counter replacements and CounterAddedOnce), then
// the monstrous designation, then the BecomeMonstrous event with amount A.
// An already-monstrous permanent does nothing (gorge's resolve-time skip);
// A <= 0 places no counters and never marks (gorge's n > 0 gate).
func (s *State) monstrosity(k *prep, in *kop.Instr) {
	sl := s.objRef(k, in.Obj)
	if sl == NoSlot || s.monstrousAt(sl) {
		return
	}
	amt := s.value(k, in.A)
	if amt <= 0 {
		return
	}
	s.g.lastCK = kop.CtrP1P1
	n := s.placeCounters(k, sl, int(kop.CtrP1P1), amt, s.perms[sl].ctrl, ctrPutEffect)
	p := &s.perms[sl]
	if n > 0 && k.anyCtrAdded {
		s.ctrNote(k, sl, int(kop.CtrP1P1))
	}
	if n > 0 && s.subs[kop.TrigCounterAddedOnce].any() {
		s.fireObj(k, kop.TrigCounterAddedOnce, sl, p.obj, p.ctrl, int16(n))
	}
	s.mSlSet(k, mechMonstrous, sl)
	s.fireBecomeMonstrous(k, sl, int16(amt))
}

// fireBecomeMonstrous fires "when <filter> becomes monstrous" (gorge
// Mode$ BecomeMonstrous: the AlterAttribute event's object matched against
// ValidCard$). No slot subscription: the battlefield is scanned as for a
// gift or a proliferate.
//
//go:noinline
func (s *State) fireBecomeMonstrous(k *prep, sl Slot, amt int16) {
	if !k.gen.anyMonstrosity {
		return
	}
	o, p := s.perms[sl].obj, s.perms[sl].ctrl
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.occ, &ord)
	for _, ls := range ord[:n] {
		s.slotTriggers(k, ls, func(a *kop.Ability) bool { return a.Trig.Event == kop.TrigBecomeMonstrous }, func(l trigListener, a *kop.Ability) {
			s.objTrigGen(k, kop.TrigBecomeMonstrous, l, a, sl, o, p, amt)
		})
	}
}
