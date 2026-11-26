package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// fightSource is OpFight / OpBite's source slot. dealt reports the op done
// already: a Bite whose source left the battlefield dealt its last known
// power (CR 608.2h). A reflexive capture that changed zones since is no
// source (zone_rememberwrites.go).
func (s *State) fightSource(k *prep, in *kop.Instr) (a Slot, dealt bool) {
	a = s.objRef(k, in.Obj)
	if in.Obj == kop.RefDelayedRemembered && (a == NoSlot || s.reflexStale()) {
		return NoSlot, in.Op == kop.OpBite && s.reflexBiteGone(k, in)
	}
	if a == NoSlot && in.Op == kop.OpBite && in.Obj == kop.RefTriggerObj {
		return NoSlot, s.trigBiteGone(k, in)
	}
	return a, false
}

// DebugNoTrigBiteLKI (test-only mutation) skips the departed trigger
// object's bite, as before its last known power was dealt.
var DebugNoTrigBiteLKI bool

// trigBiteGone is OpBite from the trigger object after it left the
// battlefield (Lothlórien Blade: "whenever equipped creature attacks, it
// deals damage equal to its power to target creature defending player
// controls", the attacker destroyed before the trigger resolves). The
// ability exists independently of its source (CR 113.7a) and the creature
// deals its last known power (CR 608.2h; gorge triggeredAttackerLKI) to the
// creature or planeswalker in Ref C, from the entry's snapshot: its last
// known keywords, controller and infect (dmgSrcObject's departed-source
// path, as the player mode dmgSrcPlayers reads them). It reports false (the
// bite is skipped as before) without a snapshot of that object, or on a
// bite over several targets (biteEachTarget).
//
//go:noinline
func (s *State) trigBiteGone(k *prep, in *kop.Instr) bool {
	e := &s.res.e
	if e.li == 0 || e.trigObj == NoObj || DebugNoTrigBiteLKI || s.xtOf(e).n != 0 {
		return false
	}
	l := s.lk(e.li)
	if l.obj != e.trigObj {
		return false
	}
	b := s.objRef(k, kop.Ref(in.C))
	if b == NoSlot || !s.isCreature(k, b) && s.typesAt(k, b)&kop.TPlaneswalker == 0 {
		return true
	}
	n := int(l.power)
	if DebugDmgAmountPlusOne {
		n++
	}
	if n <= 0 {
		return true
	}
	s.dmgSrcObject(k, s.perms[b].obj, e.trigObj, NoSlot, l.kw, l.ctrl, l.bits&lkiInfect != 0, n)
	return true
}

// biteExcess splits a trampling biter's hit (kop.BiteExcessTrample, Ram
// Through; gorge emitObjectDamageExcess): when the source a has trample and
// its power n exceeds what is lethal to b before the hit (excessLethalGen:
// toughness less marked damage, or loyalty; a deathtouch source makes it 1,
// CR 120.10), b is dealt exactly the lethal amount and b's controller the
// rest, from the same source, in the one batch (CR 120.4a). DebugBiteNoExcess
// keeps the plain hit.
func (s *State) biteExcess(k *prep, hb *hitBatch, a, b Slot, n int) {
	if DebugBiteNoExcess || s.kw(k, a)&kop.KwTrample == 0 {
		return
	}
	lethal := max(s.excessLethalGen(k, b, s.kw(k, a)&kop.KwDeathtouch != 0), 0) // gorge excessLethal clamps at 0
	if n <= lethal {
		return
	}
	hb.h[0].n = int16(lethal)
	hb.add(s, a, NoSlot, n-lethal)
	hb.h[hb.n-1].pl = s.perms[b].ctrl + 1
}

// DebugBiteNoExcess (test only, TestRamThroughExcessLockstep): a trampling
// biter's excess stays on the bitten creature.
var DebugBiteNoExcess bool
