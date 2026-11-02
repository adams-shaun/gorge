package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Mode$ CounterAdded with CounterAmount$ EQn: "when
// the Nth plan counter is put on this enchantment" (CR 122.6, 603.2c). It
// rides TrigCounterAddedOnce: the counter sites already fire that event once
// per placement batch with the batch size and the kind in lastCK, and
// objTrigGen has already matched the kind (Trig.N). The threshold is the
// ability's Gate {Ctr: kind, N: n} (AfCheckOnTrigger, so it is not rechecked
// on resolution); this file decides the crossing.
//
// gorge counterAddedMatches: after = the kind's total on the object after the
// event, before = after-Amount clamped at 0; the line fires once, when
// before < n && after >= n (9+2 crosses EQ10; 7+1 does not). The plain
// per-counter shape (no CounterAmount$) is refused by kgen, except with
// ActivationLimit$ 1 (gorge the first counter queues the turn's one
// trigger, the rest hit the limit), which rides this event under TfOncePerTurn.

// DebugCounterAddedNoCross is the mutation switch: the crossing test drops
// its before < n half, so a plan card re-fires on every later counter. Read
// only on the gated counter path, never in a walk without a gated line.
var DebugCounterAddedNoCross bool

// crossingOK reports whether a gated counter-added line a on slot sl fires
// for a batch of amt counters of the kind just put (a Saga chapter, or an
// Nth-counter line). Gates of other kinds (Class level, max speed) are not
// crossings and pass.
//
//go:noinline
func (s *State) crossingOK(sl Slot, a *kop.Ability, amt int) bool {
	switch a.Gate.Ctr {
	case kop.CtrLore:
		return s.chapterCrossed(sl, a, amt)
	case kop.CtrLevel, kop.CtrMaxSpeed, kop.CtrMode:
		return true
	}
	return s.counterAddedCrossed(sl, a, amt)
}

// counterAddedCrossed: the total of kind a.Gate.Ctr on sl just reached
// a.Gate.N.
//
//go:noinline
func (s *State) counterAddedCrossed(sl Slot, a *kop.Ability, amt int) bool {
	if sl == NoSlot || amt <= 0 {
		return false
	}
	after := s.perms[sl].ctrN(s, int(a.Gate.Ctr))
	before := after - amt
	if before < 0 {
		before = 0
	}
	n := int(a.Gate.N)
	if DebugCounterAddedNoCross {
		return after >= n
	}
	return before < n && after >= n
}
