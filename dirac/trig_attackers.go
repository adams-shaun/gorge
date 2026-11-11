package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// RefTriggerAttackers, gorge's
// Defined$ TriggeredAttackers (effects definedSpecTriggeredAttackers).
//
// Gorge reads the queue entry's Remembered, which an AttackersDeclared
// trigger fills with the whole DeclareAttackers event group (rules
// triggerRemembered / triggerRememberedFor): a per-defender line's
// defender group, the batch line's whole declaration (CR 508.3b, 508.3d,
// 603.2c: one trigger for the event). It is NOT the subset the line's
// ValidAttackers$ matched; that count is the trigger amount (trigAmt).
//
// The group is captured when the trigger is queued, as a set of objects in
// a small pool (State's cold atkGrp) the pending trigger and its stack entry
// index (pendingTrig.grp, stackEntry.grp, index+1; 0 = none). A record is
// free when nothing pending, building, on the stack or resolving holds its
// index. The body reads the group at resolution: an attacker that left the
// battlefield since is skipped (a no-op on a gone permanent).

// maxAtkGrp bounds the attacker groups held at once: two defenders and a
// handful of listeners.
const maxAtkGrp = 16

// StallAtkGrp: a seventeenth attacker group held at once (State.grpNew).
const StallAtkGrp = 107

func init() { stallNames[StallAtkGrp] = "atkGrp" }

// prepTrigAtk sets genPrep.anyTriggerAttackers: some ability program reads
// RefTriggerAttackers.
func prepTrigAtk(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if readsAtkGrp(&t.Cards[i].Abilities[j]) {
				g.anyTriggerAttackers = true
				return
			}
		}
	}
}

// readsAtkGrp reports whether a's program reads the attacker group.
func readsAtkGrp(a *kop.Ability) bool {
	for i := range a.Prog {
		if a.Prog[i].Obj == kop.RefTriggerAttackers {
			return true
		}
	}
	return false
}

// grpUsed reports whether record i (index+1) is held.
func (s *State) grpUsed(i uint8) bool {
	// a spell's grp is its promised gift opponent (kw_gift.go), not a record
	if s.ctl == ctlBuild && s.bld().e.kind != seSpell && s.bld().e.grp == i || s.res.active && s.res.e.kind != seSpell && s.res.e.grp == i {
		return true
	}
	for j := 0; j < int(s.sp); j++ {
		if s.stack[j].kind != seSpell && s.stack[j].grp == i {
			return true
		}
	}
	for j := 0; j < int(s.npend); j++ {
		if s.pend[j].grp == i {
			return true
		}
	}
	return false
}

// grpNew captures the attackers grp as a pool record and returns its index
// + 1: a held record with the same objects is shared; 0 when the pool is
// full (the game stalls, StallAtkGrp).
//
//go:noinline
func (s *State) grpNew(k *prep, grp []Slot) uint8 {
	var b objBits
	for _, sl := range grp {
		b.set(s.perms[sl].obj)
	}
	free := uint8(0)
	for i := uint8(1); i <= maxAtkGrp; i++ {
		if !s.grpUsed(i) {
			if free == 0 {
				free = i
			}
		} else if s.cold.atkGrp[i-1] == b {
			return i
		}
	}
	if free == 0 {
		s.capFull(StallAtkGrp)
		return 0
	}
	s.cold.atkGrp[free-1] = b
	s.g.atkHi = max(s.g.atkHi, free)
	return free
}

var noAtkGrp objBits

// grpOf is entry e's attacker group (empty when it has none).
func (s *State) grpOf(e *stackEntry) *objBits {
	if e.grp == 0 {
		return &noAtkGrp
	}
	return &s.cold.atkGrp[e.grp-1]
}

// fireAtkDecl queues listener l's AttackersDeclared ability a with the
// event's attacker group grp when its body reads the group (otherwise it
// is playerTrigGen). The index is stamped on every pending entry the call
// appended, including a Panharmonicon echo queueTrig adds inside it.
func (s *State) fireAtkDecl(k *prep, l trigListener, a *kop.Ability, amt int16, grp []Slot) {
	if !k.gen.anyTriggerAttackers || !readsAtkGrp(a) {
		s.playerTrigGen(k, kop.TrigAttackersDeclared, l, a, s.active, amt)
		return
	}
	gi := s.grpNew(k, grp)
	if gi == 0 {
		return // the game is stalled: a trigger whose body would do nothing is not queued
	}
	n0 := s.npend
	s.playerTrigGen(k, kop.TrigAttackersDeclared, l, a, s.active, amt)
	for i := n0; i < s.npend; i++ {
		s.pend[i].grp = gi
	}
}

// atkGrpHash mixes the held attacker groups into h: the index in a hashed
// pending or stack entry names a record whose objects differ between states.
func (s *State) atkGrpHash(h uint64) uint64 {
	for i := uint8(1); i <= s.g.atkHi; i++ {
		if s.grpUsed(i) {
			h = hmix(h, uint64(i))
			for _, w := range s.cold.atkGrp[i-1] {
				h = hmix(h, w)
			}
		}
	}
	return h
}

// execTriggerAttackers runs OpPumpUntilEOT, OpPutCounter or OpGrantKeyword
// with Obj RefTriggerAttackers once per object of the resolving trigger's
// attacker group, in object order, those still on the battlefield. The
// amounts are read once, before the walk (gorge computes them before its
// recipient loop; kgen refuses a Double read per object).
//
//go:noinline
func (s *State) execTriggerAttackers(k *prep, in *kop.Instr) {
	g := s.grpOf(&s.res.e)
	var a, b int
	switch in.Op {
	case kop.OpPumpUntilEOT:
		a, b = s.value(k, in.A), s.value(k, in.B)
	case kop.OpPutCounter:
		s.g.lastCK = kop.CounterKind(in.C) // CounterAddedOnce reads the kind
		a = s.value(k, in.A)
	case kop.OpGrantKeyword:
	default:
		panic("kernel: RefTriggerAttackers under an op that does not run it")
	}
	for w, word := range g {
		for ; word != 0; word &= word - 1 {
			o := Obj(w*64 + bits.TrailingZeros64(word))
			if s.objs[o].zone != kop.ZoneBattlefield {
				continue
			}
			sl := s.objs[o].slot
			p := &s.perms[sl]
			switch in.Op {
			case kop.OpPumpUntilEOT:
				op, ot := s.eotPT(sl)
				np, nt := op+a, ot+b
				if np != int(int16(np)) || nt != int(int16(nt)) {
					s.capFull(StallPT)
					return
				}
				s.touchSl(sl)
				if !s.setEotPT(sl, np, nt) {
					return
				}
			case kop.OpGrantKeyword:
				m := kop.KwMask(uint16(in.C)) | kop.KwMask(uint16(in.B.N))<<16
				s.grantEOT(sl, m)
				s.promoKw(k, sl, m)
				s.touchSl(sl)
			case kop.OpPutCounter:
				m := s.placeCounters(k, sl, int(in.C), a, p.ctrl, ctrPutEffect)
				if m > 0 && k.anyCtrAdded {
					s.ctrNote(k, sl, int(in.C))
				}
				if m > 0 && s.subs[kop.TrigCounterAddedOnce].any() {
					s.fireObj(k, kop.TrigCounterAddedOnce, sl, p.obj, p.ctrl, int16(m))
				}
			}
		}
	}
}
