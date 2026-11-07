package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// "Whenever this Mount becomes saddled [for the first
// time each turn]" (CR 702.171b; gorge Mode$ BecomesSaddled, trigmatch/
// saddled.go). gorge matches the Saddled grant each Saddle resolution emits;
// FirstTimeSaddled is the object's first grant since the turn changed, which
// is the cseenSaddled bit's value before the saddle op sets it. A Mount
// that changed zones is a new object (CR 400.7) with a fresh bit, and gorge's
// latch stops at that zone change too.

// fireBecomesSaddled queues the saddled Mount's own BecomesSaddled triggers;
// was is the saddled bit before this saddle. OpSaddle is cold, so the
// ability scan needs no prep flag (a pcard bool pushed pcard past 64 bytes
// and cost ~1.6% instructions on the hot card lookups).
//
//go:noinline
func (s *State) fireBecomesSaddled(k *prep, sl Slot, was bool) {
	o := s.perms[sl].obj
	ob := &s.objs[o]
	card := s.cardAt(sl) // a Mount that is a copy fires the copied card's triggers (CR 707.2)
	cd := &k.t.Cards[card]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbTriggered || a.Trig.Event != kop.TrigBecomesSaddled {
			continue
		}
		if was && a.Trig.Flags2&kop.Tf2FirstTime != 0 {
			continue
		}
		np := s.npend
		s.enqueue(o, ob.ver, card, uint8(i), s.ctrlOfObj(o), o, s.ctrlOfObj(o), 0)
		s.panharmKeyword(k, ob.slot, kop.TrigBecomesSaddled, o, np)
	}
}
