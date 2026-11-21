package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The Phantom creatures' self-damage replacement (Phantom Tiger, Unbreathing
// Horde: "If CARDNAME would be dealt damage, prevent that damage and remove a
// +1/+1 counter from it") beside the damage replacements gorge orders it with
// (CR 616.1: the affected permanent's controller picks which applies next).
//
// gorge collects the line in its object scan beside every other match (a
// doubler or a plus-N addition, a combat-only scoped prevention record's Effect
// first), asks the order of two or more, and applies each pick as it is picked
// (CR 616.1e): the Phantom's pick prevents the whole event and removes a counter,
// any other full prevention picked first stops the event before the Phantom's
// counter goes, and the Phantom left over when the ask ends applies on its own.
// The kernel makes the line a candidate of the same static-prevention walks
// (spCollect, spCombatCands, parkNC's spMergeNC): a full prevention at the host's
// scan position whose pick, or last-remaining application, removes the counter
// (spSpend). A lone Phantom beside no other match is still replDamage's own
// selfDamageAll, and a frozen combat pass applies it as each unasked event is
// dealt (dealCombatEvent: selfPhDeal), not in the recomputed pass.
//
// Lifted for a table whose self-damage hosts are all this body and whose other
// damage replacements are doublers, plus-N additions, combat-only scoped
// prevention records (not both kinds) and a SHIELD counter, which stalls loud while the
// walks do not pose it beside the Phantom (selfPhLifted). A competition the walks cannot
// name stalls loud (StallReplPlus), as the static prevention's does.

// DebugSelfPhOff leaves the Phantom's line out of the order ask beside another
// damage replacement (the walk then asks the others alone and the Phantom's
// prevention applies on its own): the oracle mutation test. False in normal play.
var DebugSelfPhOff bool

// DebugShieldEarly poses the SHIELD counter's prevention before the merged static
// and Phantom candidates, the walks' old position: the oracle mutation test. False
// in normal play.
var DebugShieldEarly bool

// isSelfPh: a is the Phantom body, prevent and remove one +1/+1 counter.
func isSelfPh(a *kop.Ability) bool {
	return a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplPreventDamageCounters && a.Repl.C == kop.ReplPrevSelfRemoveOne
}

// useSelfPh: the table has a Phantom body and no other self-damage replacement.
var useSelfPh = newGenUse(func(t *kop.Table) bool {
	ph := false
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			switch {
			case isSelfPh(a):
				ph = true
			case a.Kind == kop.AbReplacement && (a.Repl.Kind == kop.ReplPreventDamageCounters || a.Repl.Kind == kop.ReplDamageHealFirst || a.Repl.Kind == kop.ReplPreventDamageToSelf):
				return false
			}
		}
	}
	return ph
})

// selfPh: the table's Phantom bodies compete with another damage replacement
// the walks pose (selfPhLifted): the Phantom is a candidate of the order ask.
func selfPh(g *genPrep) bool {
	return g != nil && !DebugSelfPhOff && g.uses.has(useSelfPh) && g.anyRepl&rbSelfDmgCompete != 0
}

// selfPhLifted: the table's self-damage pair is one the walks pose, so initStall
// does not refuse it.
func selfPhLifted(g *genPrep) bool {
	const posed = rbDblDmg | rbPlusDmg | rbPrevCol | rbShield
	return g.uses.has(useSelfPh) && g.anyRepl&rbSelfDmgCompete&^posed == 0 && !g.uses.has(usePrevColOpen) &&
		!(g.anyRepl&rbPrevCol != 0 && g.anyRepl&(rbDblDmg|rbPlusDmg) != 0) && !g.anyRedir
}

// spSelfMatch: the Phantom host at b is a candidate of event e, damage to it.
func (s *State) spSelfMatch(g *genPrep, b Slot, a *kop.Ability, e *spEv) bool {
	return selfPh(g) && isSelfPh(a) && e.sl == b && e.n > 0
}

// spSelfNoPrev stalls loud a Phantom host's event under a can't-prevent static
// (CR 615.12): gorge keeps the line as a candidate whose body runs while the
// damage stands, which the walks do not model. It is -1 after the stall, else 0.
func (s *State) spSelfNoPrev(k *prep, e *spEv) int {
	if !selfPh(k.gen) || e.sl == NoSlot || k.gen.replBits[s.cardAt(e.sl)]&rbPrevCtrSelf == 0 || s.genLost(k, e.sl) {
		return 0
	}
	s.capFull(StallReplPlus)
	return -1
}

// spSelfRemove is a Phantom candidate's application: the counter goes (the
// damage was prevented by the caller's amount).
func (s *State) spSelfRemove(sl Slot) {
	s.selfDamageRemoveOne(sl, 1)
}

// spSelfCand: the static-prevention candidate cs is the Phantom's line.
func (s *State) spSelfCand(k *prep, c spCand) bool {
	return c.b < MaxPerm && isSelfPh(s.spCandAb(k, c))
}

// selfPhWaits reports that replDamage's self-damage step must leave the event to
// the walks: a frozen combat pass applies the Phantom as each event is dealt
// (selfPhDeal, or the walk's pick), and a noncombat event another replacement
// also matches is a loud stall, never a Phantom that prevents before its ask.
func (s *State) selfPhWaits(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, sl Slot, combat bool, n int) bool {
	if !selfPh(k.gen) || sl == NoSlot || !s.selfDamageHost(k, sl, rbPrevCtrSelf) {
		return false
	}
	if combat {
		return s.g.ro.on
	}
	if s.shieldOtherBase(k, srcCtrl, srcSl, srcCard, sl, false, n) || s.shieldN(sl) > 0 && !s.noPrev(k, false) {
		s.capFull(StallReplPlus) // gorge asks the order; parkNC did not pose it
		return true
	}
	return false
}

// selfPhDeal is dealCombatEvent's Phantom step for a frozen pass's unasked
// event: the line applies on its own as the event is dealt. true when it
// prevented the event.
func (s *State) selfPhDeal(k *prep, srcCtrl uint8, srcSl Slot, dst Slot, n int) bool {
	if !selfPh(k.gen) || dst == NoSlot || !s.selfDamageHost(k, dst, rbPrevCtrSelf) {
		return false
	}
	return s.selfDamageAll(k, dst, srcCtrl, srcSl, s.cardAt(srcSl), true, n) == 0
}

// spShieldLate: the SHIELD counter's prevention is the last candidate, after the
// static prevention and Phantom candidates the walks merge in the object scan
// (gorge appends it after the printed matches, replacement.go).
func spShieldLate(g *genPrep) bool {
	return !DebugShieldEarly && g.anyRepl&rbShield != 0 && (selfPh(g) || g.uses.has(useStaticOrdered))
}

// spFullAsk: asked candidate j is a static full prevention or a Phantom's line,
// which stops the event once applied (a shield counter picked after it never applies).
func (s *State) spFullAsk(k *prep, j int) bool {
	ro := &s.g.ro
	if ro.spv&(1<<j) == 0 {
		return false
	}
	ob := &s.objs[ro.cand[j]]
	if ob.zone != kop.ZoneBattlefield {
		return false
	}
	a := s.spAbility(k, ob.slot)
	return a != nil && (a.Repl.Kind == kop.ReplPreventDamageFilt || isSelfPh(a))
}

// spShieldOther: a static prevention or a Phantom's line also matches the event
// to the shielded permanent at sl (an ordered table: the walks pose the shield
// beside them, so shieldCombat leaves the event to the walk).
func (s *State) spShieldOther(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, sl Slot, combat bool, n int) bool {
	if !(k.gen.uses.has(useStaticOrdered) || selfPh(k.gen)) {
		return false
	}
	e := spEv{srcCtrl, srcSl, srcCard, s.perms[sl].ctrl, sl, combat, n}
	var cs [spMax]spCand
	return s.spCollect(k, &e, &cs) != 0
}

// parkShieldAt appends the SHIELD counter's prevention (CR 122.1c) to parkNC's
// c candidates: late is the call after a static prevention or a Phantom's line
// is merged (spShieldLate), else the one before the other matches; the new
// count, -1 after a loud stall.
func (s *State) parkShieldAt(k *prep, sl Slot, c int, cand *[4]Obj, prev, shld *uint8, late bool) int {
	if spShieldLate(k.gen) != late || k.gen.anyRepl&rbShield == 0 || sl == NoSlot || s.shieldN(sl) == 0 || s.noPrev(k, false) {
		return c
	}
	if c == len(cand) {
		s.capFull(StallShield) // loud: never truncate the candidates
		return -1
	}
	cand[c] = s.perms[sl].obj
	*prev |= 1 << c
	*shld |= 1 << c
	return c + 1
}

// ncPickShieldPend applies the shield candidate of a parked noncombat pend ask: the
// counter goes and nothing is left of the damage.
func (s *State) ncPickShieldPend() {
	if sl := s.ncShieldSlot(); sl != NoSlot && s.shieldN(sl) > 0 {
		s.spendShield(s.k(), sl)
	}
	s.g.ro.an = 0
}

// shieldLateAt: the noncombat recipient code is a shielded permanent whose
// counter parkShieldLate appends after the merged candidates, so a static
// prevention alone beside it is already a competition.
func (s *State) shieldLateAt(k *prep, code TCode) bool {
	if !spShieldLate(k.gen) || isPlayerOrNone(code) || int(code) >= MaxObj || s.objs[code].zone != kop.ZoneBattlefield {
		return false
	}
	sl := s.objs[code].slot
	return s.shieldN(sl) > 0 && !s.noPrev(k, false)
}

// frozenCombatAmt is the combat damage source sl assigns while a paused pass's
// order walk is live (CR 510.2: every combat damage amount is fixed before any of
// it is dealt): the sum of its frozen events' amounts while none is dealt, live
// otherwise. A Phantom's pick removes a counter mid-walk, so the recomputed pass
// would read the Phantom's own damage from its smaller power.
func (s *State) frozenCombatAmt(sl Slot, live int) int {
	ro := &s.g.ro
	bd := s.roBody()
	if bd.srcDone.has(sl) || Slot(ro.cur) == sl {
		return live
	}
	sum := 0
	for i := range bd.fz {
		if fe := &bd.fz[i]; fe.n > 0 && fe.src == sl {
			sum += int(fe.n)
		}
	}
	if sum == 0 {
		return live
	}
	return sum
}

// shieldLateFz: frozen combat event fe's recipient holds a SHIELD counter that
// replShieldCand appends after the merged candidates.
func (s *State) shieldLateFz(k *prep, fe *fzEv) bool {
	return spShieldLate(k.gen) && fe.dst != NoSlot && s.shieldN(fe.dst) > 0 && !s.noPrev(k, true)
}
