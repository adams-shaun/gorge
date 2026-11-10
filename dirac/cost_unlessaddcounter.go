package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// An UnlessCost$ AddCounter<N/KIND/recipient> (CR 118.12a, 122.1): the
// payer elects pay / decline, and paying puts N counters of KIND on the
// recipient — the resolving source (CARDNAME/NICKNAME) or the parent
// ability's target (Card.targetedBy). Gorge (pay/costparts.go
// PayManaSourceParts and unlessspecial.go chargeUnlessSpecial) places the
// counters through the ordinary CounterChange path; the stun counter on a
// non-source target is payable only while that target still exists on the
// battlefield, the same live candidate filter the unless walk reads
// (unlessCandidatesFor). A declined or unpayable election removes none.
//
// It rides PayTapX (Instr.C's nibble is full) with B.N packing the counter
// kind and the recipient (kop.PayAddCounterB); F stays a clean filter operand.
// A source recipient is the resolving source; a target recipient is resolved
// live, so a gone target makes the option unpayable.

// payKindAddCounter is the pay kind (execPay's kind, pickZ) of an unless
// AddCounter cost. It is past kop.PayAddCounter and the earlier internal
// kinds, clear of their values.
const payKindAddCounter = 23

// addCounterTarget is the live recipient slot for an encoded B.N: the
// resolving source, else the target Ref's current battlefield object. NoSlot
// means the recipient is gone, so the cost is unpayable.
func (s *State) addCounterTarget(k *prep, bn int16) Slot {
	if _, r := kop.PayAddCounterFrom(bn); r != kop.RefSelf {
		return s.objRef(k, r)
	}
	return s.resSrcSlot(k)
}

// payAddCounter settles a paid unless AddCounter: place the counters as a
// cost (the payer is the putter, so an EffectOnly$ doubler is excluded and a
// putter-scoped replacement sees the payer), then publish the ordinary
// counter-added triggers.
//
//go:noinline
func (s *State) payAddCounter(k *prep, in *kop.Instr) {
	sl := s.addCounterTarget(k, in.B.N)
	if sl == NoSlot {
		s.payDone(k, false)
		return
	}
	kind, _ := kop.PayAddCounterFrom(in.B.N)
	n := int(s.g.pickLeft)
	m := s.placeCounters(k, sl, int(kind), n, s.g.pickP, ctrPutCost)
	if m > 0 && k.anyCtrAdded {
		s.ctrNoteBy(k, sl, int(kind), s.g.pickP)
	}
	s.g.lastCK = kind
	if m > 0 && s.subs[kop.TrigCounterAddedOnce].any() {
		s.fireObj(k, kop.TrigCounterAddedOnce, sl, s.perms[sl].obj, s.perms[sl].ctrl, int16(m))
	}
	s.payDone(k, true)
}
