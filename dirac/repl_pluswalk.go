package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The CR 616.1 order walk's plus-N additions beside a non-commuting
// competitor (a doubling): gorge applies each picked replacement as it is
// picked (replacement.go handleDamageReplacementChoice) and re-collects the
// rest, so the picked order decides the amount (2(n+1) against 2n+1). The
// walk therefore carries the running amount (ro.an) and applies an addition
// or doubling pick at the pick (replOrderPick); dealAsked applies the last
// remaining candidate and aligns the recomputed pass's copy of the event,
// which replCombatGen dealt in the fixed additions-first order (replPlus).
//
// The parked NONCOMBAT event's mixed competition is walked the same way
// (parkNC's ncPend park): the picks apply as made (ncPlusPickNC) and the
// ask's end writes the picked order's amount into the record (ncDmg.n); a
// damageTo sweep deals it at dealNC, while an op-resume park
// (replPlusSrcPass / replPlusBatch) is answered by re-executing the op,
// whose asked event deals from the record (replDamagePend).

// ncPend is a parked noncombat damage event's ncDmg.n while its mixed CR
// 616.1 order ask is in progress: the amount is the picked order's, known
// only when the ask ends (replOrderPick), so the park records a sentinel.
// (ncDmg.n -1 is a parked, prevented event.)
const ncPend int16 = -2

// parkNCPend parks a noncombat damage event whose collected competition mixes
// an addition or a raise with a non-commuting amount changer (a candidate
// with neither bit is a doubling): gorge applies each picked replacement as
// it is picked (CR 616.1e), so the order decides the amount and it is not
// known here. The park carries the running amount in ro.an and the sentinel
// ncDmg.n; the picks apply as made (replOrderPick's ncPlusPickNC) and the
// ask's end writes the final amount back into the record (the op-resume
// shape's answer re-executes the op, whose asked event deals from it —
// replDamagePend; a damageTo sweep's parked event is dealt at dealNC). gorge
// re-collects the rest after each pick, and a raise's match (the amount below
// the host's power) can expire mid-ask, so the re-collection the walk mirrors
// reads the raise against the running amount (ncPlusLive). A candidate
// replDamage's fixed order cannot carry (pendBlocked) keeps the event on
// replDamage's loud paths. It reports whether it parked.
func (s *State) parkNCPend(k *prep, ctrl uint8, srcSl Slot, srcCard CardID, code TCode, rcp uint8, sl Slot, n int, kw kop.KwMask, plus, rs, prev, shld, pnx, spv uint8, cand [4]Obj, c int) bool {
	if plus|rs|spv == 0 || prev != 0 || shld != 0 || pnx != 0 {
		return false
	}
	mixed := rs != 0 || spv&spShieldAsk != 0 // an all-but shield beside an amount changer: the order decides the amount
	for i := 0; i < c; i++ {
		if plus&(1<<i) == 0 && rs&(1<<i) == 0 && spv&(1<<i) == 0 {
			mixed = true // a doubling: the order decides the amount
		}
	}
	if !mixed || s.pendBlocked(k, ctrl, srcSl, srcCard, code, rcp, sl) {
		return false
	}
	ro := &s.g.ro
	ro.replHdr = replHdr{n: uint8(c), plus: plus | rs<<plRaiseShift, spv: spv, p: rcp, cand: cand,
		ac: uint8(c), an: int16(min(n, 1<<14)), askS: srcSl, askD: sl, actl: ctrl, apl: rcp}
	nc := ncDmg{code: code, ctrl: ctrl, ll: kw&kop.KwLifelink != 0, dt: kw&kop.KwDeathtouch != 0, n: ncPend}
	if srcSl != NoSlot {
		nc.src = s.perms[srcSl].obj
	} else {
		nc.src, nc.spell = s.res.e.obj, s.res.e.kind == seSpell
	}
	s.g.nc = nc
	return true
}

// ncPlusPickNC applies the parked event's picked candidate i to the running
// amount ro.an, as gorge applies each pick as it is picked (CR 616.1e): an
// addition adds its Repl.A read now, a raise replaces the amount with the
// host's power while the amount is still below it (Ojer Axonil), a doubling
// doubles. The source and recipient are the parked event's (ro.askS, ro.askD,
// ro.actl, ro.apl); a raise host's match reads the running amount, so a
// raise picked after a doubling has pushed it past the power applies nothing.
func (s *State) ncPlusPickNC(i int) {
	ro := &s.g.ro
	if ro.spv&(1<<i) != 0 {
		s.spSpend(i) // a static prevention beside the amount changers (repl_staticprevent.go)
		return
	}
	if ro.plus&(plRaise(1<<i)|1<<i) == 0 {
		if ro.an < 1<<12 {
			ro.an *= 2 // a doubling
		}
		return
	}
	sl := s.objs[ro.cand[i]].slot
	if sl == NoSlot || !s.occ.has(sl) {
		return // a host gone from the battlefield applies nothing (gorge re-collects)
	}
	card := CardID(0)
	if ro.askS != NoSlot {
		card = s.cardAt(ro.askS)
	} else {
		card = s.objs[s.g.nc.src].card // the resolving spell's, or a departed trigger object's card
	}
	add, raise, ok := s.replPlusCand(s.k(), sl, ro.askS, card, ro.actl, ro.apl, ro.askD, false, int(ro.an))
	if !ok {
		return // a raise past its host's power no longer matches (gorge re-collects it out)
	}
	if raise != 0 {
		ro.an = int16(min(raise, 1<<14))
	} else {
		ro.an = s.sat16(int(ro.an) + add)
	}
}

// ncPlusLive reports whether the pend ask's unused candidate i is still one
// gorge re-collects at this pick: a raise matches only while the running
// amount is below its host's power (replPlusCand's raise arm), and every
// other candidate's match does not read the amount.
func (s *State) ncPlusLive(i int) bool {
	ro := &s.g.ro
	if ro.spv&(1<<i) != 0 {
		return true // a static prevention's match does not read the amount
	}
	if ro.plus&plRaise(1<<i) == 0 {
		return true
	}
	sl := s.objs[ro.cand[i]].slot
	if sl == NoSlot || !s.occ.has(sl) {
		return false
	}
	card := CardID(0)
	if ro.askS != NoSlot {
		card = s.cardAt(ro.askS)
	} else {
		card = s.objs[s.g.nc.src].card
	}
	_, raise, ok := s.replPlusCand(s.k(), sl, ro.askS, card, ro.actl, ro.apl, ro.askD, false, int(ro.an))
	return ok && raise != 0
}

// ncPlusLeftovers applies the ask's last remaining candidate(s) as gorge
// applies the re-collected rest when the ask ends (CR 616.1e; at most one is
// left, and a raise that no longer matches applies nothing).
func (s *State) ncPlusLeftovers() {
	ro := &s.g.ro
	for i := 0; i < int(ro.n); i++ {
		if ro.used&(1<<i) == 0 {
			s.ncPlusPickNC(i)
		}
	}
}

// replDamagePend is the resolving damage passes' replDamage while an answered
// park's record is live (replPlusSrcPass / replPlusBatch answered by
// re-executing the op): the asked event's amount is the picked order's
// (nc.n, written at the ask's end) and the record is consumed by it. Any
// other event, and any event of a pass without a record, deals through
// replDamage.
func (s *State) replDamagePend(k *prep, ctrl uint8, srcSl Slot, srcCard CardID, code TCode, n int, srcObj Obj) int {
	if s.g.plusBatchAsked && s.g.nc.n > 0 && s.g.nc.code == code && s.g.nc.src == srcObj {
		n = int(s.g.nc.n)
		s.g.nc = ncDmg{}
		if k.anyRedNC {
			s.tallyRedNC(k, ctrl, srcSl, srcCard, n) // the record never reached replDamage, the funnel that tallies
		}
		return n
	}
	if s.g.plusBatchAsked && s.g.nc.n < 0 && s.g.nc.n != ncPend && s.g.nc.code == code && s.g.nc.src == srcObj {
		// a parked, prevented event (a SHIELD candidate beside a plus-N
		// addition): the answer spent the counter as picked, so the event
		// deals nothing and replDamage must not read the spent counter again
		s.g.nc = ncDmg{}
		return 0
	}
	return s.replDamage(k, ctrl, srcSl, srcCard, code, false, n)
}

// replPlusAskedResume closes an answered replplus ask whose op resumes
// (plusBatchAsked, set by replPlusSrcPass and replPlusBatch): a mixed
// competition's park (ro.ac != 0) keeps its record, whose picks were applied
// as made (ncPlusPickNC) and whose amount the re-executed op's asked event
// deals from (replDamagePend); a pure plus-N batch order ask drops the parked
// event too, since the order cannot change the sum. The walk is dropped and
// the op re-executes.
func (s *State) replPlusAskedResume(k *prep) {
	if s.g.ro.ac != 0 || s.g.ro.shld != 0 && s.g.nc.n < 0 {
		s.g.ro = replOrd{}
	} else {
		s.g.ro, s.g.nc = replOrd{}, ncDmg{}
	}
	s.res.ask = raNone
	s.runProg(k)
}

// pendBlocked reports that the event is matched by a replacement kind the
// mixed walk's fixed candidate list cannot carry (a static prevention's lone
// candidate, a redirect, a self-damage host, the Mindskinner): the event
// stays with replDamage's loud paths, which a blocked pend park falls
// through to (parkNC's m branches, parkShape's replParkOther).
func (s *State) pendBlocked(k *prep, ctrl uint8, srcSl Slot, srcCard CardID, code TCode, rcp uint8, sl Slot) bool {
	g := k.gen
	if g.uses.has(useStaticPrev) && !g.uses.has(useStaticOrdered) {
		return true // a static prevention beside the competition is a candidate too (spPark applied lone ones); the ordered shapes are merged into the walk (spMergeNC)
	}
	if g.uses.has(useStatRdr) {
		if e, ok := s.rdrEvOf(ctrl, srcSl, srcCard, code, false); ok {
			var cands [rdrMax]rdrCand
			if s.statRdrCollect(k, &e, &cands) > 0 {
				return true // the redirect rewrites the recipient before the replacements (statRdrTake)
			}
		}
	}
	if g.anyRedir && s.redirLive() > 0 {
		return true // an Ext DmgRedirect record is a candidate too (redirTake)
	}
	if g.anyRepl&rbSelfDmg != 0 && sl != NoSlot {
		return true // a self-damage host on the recipient is a candidate too (selfDamageAll)
	}
	if g.anyRepl&rbPrevMill != 0 && sl == NoSlot && rcp != ctrl {
		return true // the Mindskinner prevents and mills beside the competition (preventMill)
	}
	return false
}

// roPlusPend reports that the ask in progress collected a plus-N addition
// beside a non-commuting amount changer (a candidate with no bits is a
// doubling): its picks are applied as made, not pre-applied.
func (s *State) roPlusPend() bool {
	ro := &s.g.ro
	sh := ro.spv&spShieldAsk != 0 // an all-but shield beside the amount changers
	if ro.plus == 0 && !sh {
		return false
	}
	if sh && ro.plus != 0 {
		return true
	}
	for i := uint(0); i < uint(ro.ac); i++ {
		b := uint8(1) << i
		if ro.plus&b == 0 && ro.prev&b == 0 && ro.shld&b == 0 && ro.pnx&b == 0 && ro.spv&b == 0 {
			return true
		}
	}
	return false
}

// roPlusAdd is the walk's plus-N candidate i's live addition (the host's
// Repl.A, or its counters or power, read now, as gorge applies each pick).
// The source's card is the frozen event's, so a source a state-based action
// removed at the pause still matches (replOrderCombat's LKI).
func (s *State) roPlusAdd(k *prep, i, n int) int {
	ro := &s.g.ro
	sl := s.objs[ro.cand[i]].slot
	if sl == NoSlot || !s.occ.has(sl) {
		return 0
	}
	card := s.cardAt(ro.askS)
	if fe := s.replOrderFz(ro.askS, ro.askD); fe != nil {
		card = fe.card
	}
	add, _, ok := s.replPlusCand(k, sl, ro.askS, card, ro.actl, ro.p, ro.askD, true, n)
	if !ok {
		return 0
	}
	return add
}

// walkAsksRepl reports whether the combat damage order walk poses gorge's
// CR 616.1 ask for this event: replPlus's additions then wait for the walk
// (replOrderCandsFz collects them beside the doublings), so replPlus must
// not cap. The probe saves and restores the ask scratch, the way
// dmgSrcOrderStalls probes parkNC; a candidate overflow inside it stops the
// game, as the walk's own collection would.
func (s *State) walkAsksRepl(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, rcp uint8, dst Slot, n int) bool {
	if !s.g.ro.on || srcSl == NoSlot {
		return false
	}
	save := s.g.ro
	fe := fzEv{src: srcSl, dst: dst, n: int16(min(n, 1<<14)), card: s.cardAt(srcSl), ctrl: s.perms[srcSl].ctrl, apl: rcp}
	c := s.replOrderCandsFz(k, &fe)
	s.g.ro = save
	return c >= 2
}

// syncAskedEv aligns the recomputed pass's copy of the asked event with the
// amount the picked order dealt: replCombatGen applies a fixed order (the
// additions before the doublings, replPlus; static preventions in scan order,
// spApply), while the walk applies each pick as made (CR 616.1e), so the
// amounts differ when a doubling or a non-commuting static shield is picked
// first. combatEvLess shrinks the accounting; the mirror arm grows it back (a
// fully prevented event).
func (s *State) syncAskedEv(k *prep, r *dmgRec, n int) {
	ro := &s.g.ro
	for i := 0; i < r.nev; i++ {
		if e := &r.ev[i]; e.src == ro.askS && e.dst == ro.askD {
			code := PlayerCode(ro.apl)
			if ro.askD != NoSlot {
				code = TCode(s.perms[ro.askD].obj)
			}
			if d := int(e.n) - n; d > 0 {
				s.combatEvLess(r, e, code, d)
			} else if d < 0 {
				d = -d
				e.n = s.sat16(int(e.n) + d)
				r.dealt[e.src] = s.addSat16(r.dealt[e.src], d)
				if e.dst == NoSlot {
					r.toPl[CodeSeat(code)] = s.addSat16(r.toPl[CodeSeat(code)], d)
					r.dealtPl[e.src] = s.addSat16(r.dealtPl[e.src], d)
				} else {
					r.toPerm[e.dst] = s.addSat16(r.toPerm[e.dst], d)
				}
			}
			return
		}
	}
}

// frozenDealtSync reports whether the order walk already dealt this event of
// the recomputed pass, and aligns the pass's copy of it with the amount the
// picked order gave it (the frozen event's, which dealAsked and
// dealCombatEvent write back). The combat step recomputes the pass on every
// resume (combat.go), and replCombatGen's fixed order (the additions before
// the doublings, replPlus) differs from the walk's picked order when
// roPlusPend.
func (s *State) frozenDealtSync(k *prep, r *dmgRec, ev *dmgEv) bool {
	ro := &s.g.ro
	bd := s.roBody()
	if !(bd.srcDone.has(ev.src) || ev.dst == NoSlot && bd.plDone.has(ev.src) ||
		ev.src == Slot(ro.cur) && ev.dst != NoSlot && bd.dstDone.has(ev.dst)) {
		return false
	}
	fe := s.replOrderFz(ev.src, ev.dst)
	if fe == nil {
		return true // dealt, nothing frozen to align with (none expected)
	}
	n := int(fe.n)
	code := PlayerCode(CodeSeat(s.perms[ev.src].atk)) // the player the source attacks (replCombatGen's)
	if ev.dst != NoSlot {
		code = TCode(s.perms[ev.dst].obj)
	}
	if d := int(ev.n) - n; d > 0 {
		s.combatEvLess(r, ev, code, d)
	} else if d < 0 {
		d = -d
		ev.n = s.sat16(int(ev.n) + d)
		r.dealt[ev.src] = s.addSat16(r.dealt[ev.src], d)
		if ev.dst == NoSlot {
			r.toPl[CodeSeat(code)] = s.addSat16(r.toPl[CodeSeat(code)], d)
			r.dealtPl[ev.src] = s.addSat16(r.dealtPl[ev.src], d)
		} else {
			r.toPerm[ev.dst] = s.addSat16(r.toPerm[ev.dst], d)
		}
	}
	return true
}
