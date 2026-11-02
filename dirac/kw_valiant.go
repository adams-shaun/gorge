package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Valiant (CR 603.2 ability word; gorge,
// trigmatch.valiantFirstTimeTargetedByYouThisTurn). Cold and out of line
// the hot site is checkWard's existing subs[TrigBecomesTarget] gate.

// valiantMark is the used-record ability byte of a Valiant mark: valiantMark
// | player records "player's spell or ability targeted this object this
// turn" (ability indices stay below it; resolveMark records sit at 0x80+).
const valiantMark = 0x7E

// becameTarget handles a TrigBecomesTarget event: the permanent at sl became
// a target of stack entry e (checkWard's per-target walk, in target order).
//
// Gorge's Valiant gate, per object and per player: a targeting counts when
// the targeting spell or ability's controller (stackObjControllerAt, the
// controller when it targeted: the kernel's e.ctrl) is the trigger's
// controller, and no earlier targeting this turn by that same player named
// the same object (a new object after a zone change has none: the mark is
// keyed by ver; the turn reset clears the pool). The same entry naming the
// object twice is one targeting (gorge's per-spell dedupe): the first
// occurrence marks it, so the second finds the mark. An opponent's
// targeting marks only the opponent, so it neither fires nor disarms.
// Marks are kept only for an object whose card prints a Valiant trigger
// (the pool is the shared once-per-turn pool, cap maxUsed, stalls loud).
//
//go:noinline
func (s *State) becameTarget(k *prep, sl Slot, e *stackEntry) {
	p := &s.perms[sl]
	if k.gen == nil {
		s.fireObj(k, kop.TrigBecomesTarget, sl, p.obj, p.ctrl, 0)
		return
	}
	if k.gen.btWatch {
		s.btWatchers(k, sl, p.obj, e) // trig_becomestarget.go
	} else if !s.printsValiant(k, s.cardAt(sl)) {
		s.fireObjGen(k, kop.TrigBecomesTarget, sl, p.obj, p.ctrl, 0)
		return
	}
	if !s.printsValiant(k, s.cardAt(sl)) {
		return
	}
	ab := uint8(valiantMark | e.ctrl)
	if s.usedThisTurn(p.obj, ab) {
		return
	}
	s.markUsed(p.obj, ab)
	if e.ctrl != p.ctrl {
		return
	}
	s.slotTriggers(k, sl, isBecomesTarget, func(l trigListener, a *kop.Ability) {
		if l.ctrl == e.ctrl {
			s.objTrigGen(k, kop.TrigBecomesTarget, l, a, sl, p.obj, e.ctrl, 0)
		}
	})
}

func isBecomesTarget(a *kop.Ability) bool { return a.Trig.Event == kop.TrigBecomesTarget }

// printsValiant: card c has a printed Valiant trigger.
func (s *State) printsValiant(k *prep, c CardID) bool {
	cd := &k.t.Cards[c]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigBecomesTarget && a.Trig.N == kop.TrigNValiant {
			return true
		}
	}
	return false
}
