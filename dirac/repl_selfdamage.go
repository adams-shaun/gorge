package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A permanent's own DamageDone replacement on damage to itself
// (ValidTarget$ Card.Self):
//
//   - ReplPreventDamageCounters (Anti-Venom, Horrifying Healer): the damage
//     is prevented and that many +1/+1 counters are put on it instead. gorge
//     (rules/replacement.go applyNonMoveReplacements) runs the PutCounter
//     body at the proposal and hands back a stopped event: no damage, no
//     lifelink, no damage trigger. Under a can't-prevent static the body
//     still runs and the damage stands (CR 615.12).
//   - ReplDamageHealFirst (Wolverine, Fierce Fighter): the HealDamage body
//     heals the damage already marked on it (and its deathtouch mark) and,
//     ReplacementResult$ Updated, the held event is still dealt: lifelink and
//     damage triggers see it.
//
// Both apply after protection, once per damage event (combat: per source).
// Beside another damage replacement that can match the same event gorge
// asks the CR 616.1 order: initStall refuses that pair (StallReplPlus), so
// here each is the only replacement on its host's damage.

// DebugSelfDamageReplOff turns both replacements off in the kernel (the
// oracle mutation test); false in normal play.
var DebugSelfDamageReplOff bool

// rbSelfDmg is either self-damage replacement.
const rbSelfDmg = rbPrevCtrSelf | rbHealSelf

// rbSelfDmgCompete are the damage replacements that can match an event to a
// self-damage host beside its own: gorge asks their CR 616.1 order.
const rbSelfDmgCompete = rbDblDmg | rbPlusDmg | rbPrevNC | rbPrevDmgTo | rbPrevCol | rbShield | rbPrevCombatTo | rbPrevCombatBy

// selfDamage applies the permanent at sl's own damage replacement to n
// damage it would be dealt; the amount dealt.
func (s *State) selfDamage(k *prep, sl Slot, combat bool, n int) int {
	b := k.gen.replBits[s.cardAt(sl)]
	if DebugSelfDamageReplOff || b&rbSelfDmg == 0 || n <= 0 || s.genLost(k, sl) {
		return n
	}
	if b&rbHealSelf != 0 {
		s.healMarked(sl)
	}
	if b&rbPrevCtrSelf != 0 {
		s.selfDamageCounters(k, sl, n)
		if !s.noPrev(k, combat) {
			return 0
		}
	}
	return n
}

// selfDamageHost: the permanent at sl carries a self-damage replacement
// that still functions.
func (s *State) selfDamageHost(k *prep, sl Slot, b uint32) bool {
	return !DebugSelfDamageReplOff && k.gen.replBits[s.cardAt(sl)]&b != 0 && !s.genLost(k, sl)
}

// healMarked heals the damage marked on sl and its deathtouch mark (gorge
// effects.HealMarkedDamage: the negative Damage event, then the
// Deathtouched counter removed to 0).
func (s *State) healMarked(sl Slot) {
	p := &s.perms[sl]
	if p.damage == 0 && p.flags&(pfDeathtouched|pfDtMore) == 0 {
		return
	}
	s.touchSl(sl)
	if p.flags&pfDeathtouched != 0 {
		p.cseen |= cseenDT // gorge lists the removed mark at 0
	}
	p.damage = 0
	p.flags &^= pfDeathtouched | pfDtMore
}

// selfDamageCounters puts the n +1/+1 counters of the prevent-and-counters
// body on sl, the counter replacements applying. The putter is gorge's
// in-flight counter adder: the resolving object's controller, else the
// replacement's controller (rules/engine_damage.go inFlightCounterAdder).
// A placement that would pose the counter replacements' order ask stalls:
// the ask inside a damage event is not modelled.
func (s *State) selfDamageCounters(k *prep, sl Slot, n int) {
	kind := int(kop.CtrP1P1)
	// a prohibited placement (CantPutCounter) places nothing, so it asks no order
	if k.gen.anyRepl&(rbCtrOn|rbPlusCtrEOT) != 0 && !s.putCounterBlocked(k, sl, kind) {
		if c := s.ctrGather(k, sl, kind); c.ok && c.n == 2 && c.mixed() {
			s.capFull(StallReplCtr)
			return
		}
	}
	adder := s.counterAdder()
	if adder > 1 {
		adder = s.perms[sl].ctrl
	}
	m := s.placeCounters(k, sl, kind, n, adder, ctrPutBy)
	if m > 0 && k.anyCtrAdded {
		s.ctrNoteBy(k, sl, kind, adder) // the ledger's actor is the putter used above, not counterAdder()
	}
	s.g.lastCK = kop.CtrP1P1
	if m > 0 && s.subs[kop.TrigCounterAddedOnce].any() {
		s.fireObj(k, kop.TrigCounterAddedOnce, sl, s.perms[sl].obj, s.perms[sl].ctrl, int16(m))
	}
}

// selfDamageCombat is replCombatGen's arm for combat event i of r to a
// self-damage host, before replDamage: a heal host's second event of a pass
// inside the CR 616.1 ordered walk (s.g.ro.on) is not modelled: that stalls.
// Outside the walk healCombat does the pass's heals. true when the pass
// stalled.
func (s *State) selfDamageCombat(k *prep, r *dmgRec, i int) bool {
	ev := &r.ev[i]
	if !s.g.ro.on || ev.dst == NoSlot || !s.selfDamageHost(k, ev.dst, rbHealSelf) {
		return false
	}
	for j := 0; j < i; j++ {
		if r.ev[j].dst == ev.dst && r.ev[j].n > 0 {
			s.capFull(StallReplPlus)
			return true
		}
	}
	return false
}

// healCombat applies a heal host's per-event heal to the pass r after
// prevention (non-ordered pass). gorge deals the pass event by event and each
// event's heal wipes the damage and the deathtouch mark of the events before
// it, so of the live events to one host only the last one gorge deals keeps
// its damage and its deathtouch mark (healDealtAfter orders them). Infect/wither
// hits to a creature are counters, which no heal removes (infectCombat takes
// them out of toPerm itself). It never touches dealt, dealtPl or ev.n: every
// event was dealt (lifelink, damage triggers).
func (s *State) healCombat(k *prep, r *dmgRec) {
	for i := 0; i < r.nev; i++ {
		ev := &r.ev[i]
		if ev.n <= 0 || ev.dst == NoSlot || !s.selfDamageHost(k, ev.dst, rbHealSelf) {
			continue
		}
		dt := s.kw(k, ev.src)&kop.KwDeathtouch != 0
		if s.healDealtAfter(r, i) {
			if !s.combatHitIsCounters(k, ev) {
				r.toPerm[ev.dst] -= ev.n
			}
			if dt {
				s.touchSl(ev.dst)
				s.perms[ev.dst].cseen |= cseenDT // gorge lists the healed mark at 0
			}
			continue
		}
		if dt {
			r.dtPerm.set(ev.dst)
		} else {
			r.dtPerm.clr(ev.dst)
		}
	}
}

// healDealtAfter: gorge deals another live event of r to event i's host after
// event i. gorge's pass (rules/combat.go damageStep) lists each attacker's own
// assignments, then its blockers' hit-backs in block declaration order (bord);
// the kernel adds the blockers' events in slot order, so a blocker's event is
// ranked by bord, after every attacker's event.
func (s *State) healDealtAfter(r *dmgRec, i int) bool {
	rank := func(ev *dmgEv) int {
		if s.blocks(ev.src, ev.dst) {
			return 1 + int(s.blockKey(ev.src, ev.dst))
		}
		return 0
	}
	ri := rank(&r.ev[i])
	for j := 0; j < r.nev; j++ {
		o := &r.ev[j]
		if j == i || o.dst != r.ev[i].dst || o.n <= 0 {
			continue
		}
		if rj := rank(o); rj > ri || rj == ri && j > i {
			return true
		}
	}
	return false
}

// selfDamagePair is pairRefusal's halves for initStall's self-damage
// StallReplPlus: the self-damage hosts, and the cards carrying a damage
// replacement that can compete with them (a doubler, plus-N, prevention, a
// prevention spell, a SHIELD counter placer).
func selfDamagePair(g *genPrep) (string, func(*kop.CardDef, int) bool, func(*kop.CardDef, int) bool) {
	text := "a self-damage replacement (prevent-and-counters, heal-then-deal) beside another damage replacement: gorge asks the CR 616.1 order of the two"
	a := func(_ *kop.CardDef, i int) bool { return g.replBits[i]&rbSelfDmg != 0 }
	b := func(c *kop.CardDef, i int) bool {
		return g.replBits[i]&rbSelfDmgCompete != 0 || abilityOn(func(x *kop.Ability) bool {
			return x.Kind == kop.AbReplacement && x.Repl.Kind == kop.ReplEntersWithCounters && x.Repl.C == int16(kop.CtrShield) ||
				x.Cost.Flags&kop.CostAddCtr != 0 && x.Cost.Ctr == kop.CtrShield ||
				progHas(x, func(in *kop.Instr) bool {
					e := in.Ext()
					return e == kop.ExtPreventColor || e == kop.ExtPreventDamageScope || (in.Op == kop.OpPutCounter || in.Op == kop.OpPutCounterAll || in.Op == kop.OpEnterWithCounters) && in.C == int16(kop.CtrShield)
				})
		})(c, i)
	}
	return text, a, b
}
