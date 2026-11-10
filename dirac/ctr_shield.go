package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The SHIELD counter (CR 122.1c, shield): "If a permanent with a shield
// counter on it would be dealt damage or destroyed, instead remove a shield
// counter from it." Gorge (pin 513a2a0c3) splits it in two:
//
//   - destruction: effects.ReplaceShieldCounter, consulted by effDestroy and
//     effDestroyAll per victim AFTER regeneration (ReplaceDestruction, unless
//     NoRegen$ True) and BEFORE umbra armor. It is not regeneration: no tap,
//     no removal from combat, damage stays marked. State-based lethal damage
//     consults it too, after regeneration (rules/sba.go destroyLethalDamage,
//     turn.go State.sba).
//   - damage: rules/shield_counter.go, one more DamageDone prevention
//     replacement over ANY damage (combat and noncombat, any source) dealt
//     to a battlefield permanent holding a SHIELD counter: the event is
//     prevented whole and one counter removed. Protection prevents before the
//     replacements (gorge emit), so a protected recipient keeps its counter.
//     It competes with the other damage replacements under CR 616.1 (two or
//     more matches: gorge's order ask); the kernel appends the shield to that
//     ask as its last candidate (gorge appends shieldCounterDamageMatches
//     after the printed and granted-keyword matches), so a doubler or the
//     recipient's combat-damage prevention gets the ask (shieldCombat,
//     shieldFrozen, replOrderCandsFz, parkNC). A plus-N replacement or the
//     source's combat prevention is not one of the ask's candidates, so that
//     competition still stalls (StallShield). Damage that can't be prevented
//     skips it, as in gorge (noPrev, static_cantprevent.go).
//
// Combat damage is a sequence of events in gorge (runCombatAssignments,
// replOrderOrder's order): an attacker double-blocked onto a shielded
// creature spends the counter on the first event only, the second lands.
//
// Everything here sits behind genPrep.anyRepl&rbShield (prepShield: some
// program or entry replacement places a SHIELD counter), unset on FDN and
// BLB, whose tables carry no SHIELD (TestShieldPrepOffOnFrozenPools).

// DebugIgnoreShield is the lockstep's mutation switch (TestShieldMutation):
// set, prepShield leaves rbShield unset, so SHIELD counters are placed but
// never read. Read at prep only, never in a walk.
var DebugIgnoreShield bool

// prepShield sets rbShield in genPrep.anyRepl when the table can place a
// SHIELD counter, and marks the cards whose Continuous static gives its
// controller hexproof (StYouHexproof: Captain America, Super-Soldier's
// "you and other Heroes you control have hexproof" while he has a shield
// counter; Shalai and Sigarda the same shape) as rbPlHex sources.
func prepShield(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if a := &t.Cards[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && a.Static.StFlags&kop.StYouHexproof != 0 {
				g.replBits[i] |= rbPlHex
				g.anyYouHex = true
			}
		}
	}
	if DebugIgnoreShield {
		return
	}
	sh := int16(kop.CtrShield)
	prog := func(p []kop.Instr) bool {
		for i := range p {
			switch in := &p[i]; in.Op {
			case kop.OpPutCounter, kop.OpPutCounterAll, kop.OpEnterWithCounters, kop.OpBolster:
				if in.C == sh {
					return true
				}
			case kop.OpMoveCounter:
				if in.B.N == sh {
					return true
				}
			}
		}
		return false
	}
	abil := func(a *kop.Ability) bool {
		return prog(a.Prog) || a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplEntersWithCounters && a.Repl.C == sh ||
			a.Cost.Flags&kop.CostAddCtr != 0 && a.Cost.Ctr == kop.CtrShield
	}
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if abil(&t.Cards[i].Abilities[j]) {
				g.anyRepl |= rbShield
			}
		}
	}
	for i := range t.Emblems {
		for j := range t.Emblems[i].Abilities {
			if abil(&t.Emblems[i].Abilities[j]) {
				g.anyRepl |= rbShield
			}
		}
	}
}

// shieldN is the SHIELD count of the permanent at sl.
func (s *State) shieldN(sl Slot) int { return s.perms[sl].ctrN(s, int(kop.CtrShield)) }

// spendShield removes one SHIELD counter from sl (gorge: CounterChange
// SHIELD -1; the kind stays listed at 0, as gorge's counter list keeps it).
func (s *State) spendShield(sl Slot) {
	s.touchSl(sl)
	p := &s.perms[sl]
	p.seeCtr(s)
	p.subCtrN(s, int(kop.CtrShield), 1)
}

// shieldDestroy is the destruction half: true when sl held a SHIELD counter
// and one was removed instead of the destruction.
//
//go:noinline
func (s *State) shieldDestroy(sl Slot) bool {
	if s.shieldN(sl) <= 0 {
		return false
	}
	s.spendShield(sl)
	return true
}

// shieldAll is OpDestroyAll's shield pass (after regenAll): each victim of
// hit holding a SHIELD counter loses one instead and leaves hit.
//
//go:noinline
func (s *State) shieldAll(hit slotMask) slotMask {
	var out slotMask
	for m := hit; m.any(); m = m.rest() {
		sl := m.first()
		if s.shieldDestroy(sl) {
			continue
		}
		out = out.with(sl)
	}
	return out
}

// shieldOther reports whether a damage replacement other than the shield
// also matches the event (gorge would pose its CR 616.1 order ask): a
// doubler of the source's controller against an opponent's permanent, a
// plus-N that changes it, a Crystal Barricade (noncombat), the recipient's
// or the source's combat prevention (combat).
func (s *State) shieldOther(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, sl Slot, combat bool, n int) bool {
	g := k.gen
	rcp := s.perms[sl].ctrl
	if g.anyRepl&rbDblDmg != 0 && s.dblDmgN2(k, srcCtrl, srcSl, srcCard, rcp, sl, combat) > 0 {
		return true
	}
	if g.anyRepl&rbPlusDmg != 0 && s.replPlus(k, srcCtrl, srcSl, srcCard, rcp, sl, combat, n) != n {
		return true
	}
	if g.anyRepl&rbPrevNC != 0 && !combat && s.isCreature(k, sl) {
		for m := s.statM.and(s.ctrlM[rcp]); m.any(); m = m.rest() {
			if b := m.first(); b != sl && g.replBits[s.cardAt(b)]&rbPrevNC != 0 && !s.genLost(k, b) && !s.noPrev(k, combat) {
				return true
			}
		}
	}
	if combat && s.prevCombatTo(k, sl) {
		return true
	}
	if combat && g.anyPrevFrom && srcSl != NoSlot && s.prevCombatFrom(k, srcSl) {
		return true
	}
	return false
}

// shieldOrderable reports whether the other replacement matching a combat
// event is one of replOrderCandsFz's candidates, so the CR 616.1 ask can
// model the shield too: a doubler of the source's controller, or the
// recipient's combat-damage prevention. A plus-N or the source's prevention
// is not a candidate, so shieldCombat still stalls on those.
func (s *State) shieldOrderable(k *prep, src, dst Slot) bool {
	g := k.gen
	sc, rcp := s.perms[src].ctrl, s.perms[dst].ctrl
	if g.anyRepl&rbDblDmg != 0 && s.dblDmgN2(k, sc, src, s.cardAt(src), rcp, dst, true) > 0 {
		return true
	}
	return s.prevCombatTo(k, dst)
}

// ncShieldSlot is the parked noncombat event's recipient slot, or NoSlot
// when it is a player or has left the battlefield.
func (s *State) ncShieldSlot() Slot {
	nc := &s.g.nc
	if nc.code >= PlayerTarget0 || int(nc.code) >= MaxObj {
		return NoSlot
	}
	if s.objs[nc.code].zone != kop.ZoneBattlefield {
		return NoSlot
	}
	return s.objs[nc.code].slot
}

// replNCPickShield spends the counter when the player picked the shield
// candidate of a parked noncombat event. A non-shield prevention picked
// first stops the event, so the shield is not spent.
func (s *State) replNCPickShield(i int) {
	ro := &s.g.ro
	if ro.shld&(1<<i) == 0 {
		return
	}
	if sl := s.ncShieldSlot(); sl != NoSlot && s.shieldN(sl) > 0 {
		s.spendShield(sl)
	}
}

// replNCAutoShield spends the counter when the ask ends after a non-prev
// pick: the shield is the last remaining candidate and applies
// automatically, unless another prevention was picked first (then the
// event already stopped and the shield never applies).
func (s *State) replNCAutoShield() {
	ro := &s.g.ro
	if ro.shld == 0 {
		return
	}
	for j := 0; j < int(ro.n); j++ {
		if ro.prev&(1<<j) != 0 && ro.shld&(1<<j) == 0 && ro.used&(1<<j) == 0 {
			return // another prevention remains: it applies, not the shield
		}
	}
	for i := 0; i < int(ro.n); i++ {
		if ro.shld&(1<<i) != 0 && ro.used&(1<<i) == 0 {
			if sl := s.ncShieldSlot(); sl != NoSlot && s.shieldN(sl) > 0 {
				s.spendShield(sl)
			}
			return
		}
	}
}

// shieldNC is replDamage's shield arm for noncombat damage n to the
// shielded permanent at sl (protection already checked): the damage is
// prevented and one counter removed; another matching replacement stalls.
//
//go:noinline
func (s *State) shieldNC(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, sl Slot, n int) int {
	if s.shieldOther(k, srcCtrl, srcSl, srcCard, sl, false, n) {
		s.capFull(StallShield)
		return n
	}
	s.spendShield(sl)
	return 0
}

// shieldFrozen applies the shield to a combat event of a resumed ordered
// pass that has no order ask (shieldCombat skipped the pass while ro.on):
// spend the counter when no other replacement matches, otherwise stall (an
// orderable one would have been a candidate and asked).
func (s *State) shieldFrozen(k *prep, ctrl uint8, srcSl Slot, srcCard CardID, dst Slot, n int) bool {
	if k.gen.anyRepl&rbShield == 0 || dst == NoSlot || n <= 0 || !s.occ.has(dst) || s.shieldN(dst) <= 0 || s.noPrev(k, true) {
		return false
	}
	if s.kw(k, dst)&kop.KwProtectionAll != 0 && !s.noPrev(k, true) ||
		k.gen.anyRepl&rbProtCol != 0 && s.protFrom(k, dst, srcSl, srcCard, s.srcColors(k, srcSl, srcCard), true) {
		return true // protection prevents it before the replacements: keep the counter
	}
	if s.shieldOther(k, ctrl, srcSl, srcCard, dst, true, n) {
		s.capFull(StallShield)
		return true
	}
	s.spendShield(dst)
	return true
}

// shieldCombat is the combat arm, run before replCombatGen: r's events to a
// shielded permanent, in gorge's assignment order, are prevented one at a
// time while a counter remains (each spends one). An event protection
// prevents is left to replCombatGen (it prevents first, keeping the
// counter); an event another replacement also matches is left to the
// replacement-order ask when that replacement is one of the ask's
// candidates (a doubler or the recipient's combat prevention), otherwise
// the kernel still stalls.
//
//go:noinline
func (s *State) shieldCombat(k *prep, r *dmgRec) {
	if s.g.ro.on {
		// a resumed ordered pass: replOrderCombat/dealAsked owns every
		// shielded event in it, so do not spend a counter twice
		return
	}
	var ord [MaxPerm]uint8
	for _, i := range ord[:s.replOrderOrder(r, &ord)] {
		ev := &r.ev[i]
		if ev.n <= 0 || ev.dst == NoSlot || !s.occ.has(ev.dst) || s.shieldN(ev.dst) <= 0 || s.noPrev(k, true) {
			continue
		}
		d, src := ev.dst, ev.src
		if s.kw(k, d)&kop.KwProtectionAll != 0 && !s.noPrev(k, true) ||
			k.gen.anyRepl&rbProtCol != 0 && s.protFrom(k, d, src, s.cardAt(src), s.colorsGen(k, src), true) {
			continue
		}
		if s.shieldOther(k, s.perms[src].ctrl, src, s.cardAt(src), d, true, int(ev.n)) {
			if !s.shieldOrderable(k, src, d) {
				s.capFull(StallShield)
				return
			}
			continue // replOrderFreeze/replOrderCombat pose the ask
		}
		s.preventCombatEv(k, r, ev) // combat_prevention.go
		s.spendShield(d)
	}
}

// youHexproof is playerHexproof in a table with a StYouHexproof static:
// player p controls a StatPlayerHexproof permanent, or one whose
// StYouHexproof static's condition holds (gorge: the static's Affected$
// You grants the controller Hexproof while its Condition$/IsPresent$ holds).
//
//go:noinline
func (s *State) youHexproof(k *prep, p uint8) bool {
	g := k.gen
	for m := s.statM.and(s.ctrlM[p]); m.any(); m = m.rest() {
		sl := m.first()
		if g.replBits[s.cardAt(sl)]&rbPlHex == 0 || s.genLost(k, sl) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(sl)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbStatic {
				continue
			}
			if a.Static.Kind == kop.StatPlayerHexproof {
				return true
			}
			if a.Static.Kind == kop.StatContinuous && a.Static.StFlags&kop.StYouHexproof != 0 &&
				s.statCond(k, statSrc{a: a, sl: sl, ctrl: p, obj: s.perms[sl].obj}) {
				return true
			}
		}
	}
	return false
}
