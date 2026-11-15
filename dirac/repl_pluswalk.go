package dirac

// The CR 616.1 order walk's plus-N additions beside a non-commuting
// competitor (a doubling): gorge applies each picked replacement as it is
// picked (replacement.go handleDamageReplacementChoice) and re-collects the
// rest, so the picked order decides the amount (2(n+1) against 2n+1). The
// walk therefore carries the running amount (ro.an) and applies an addition
// or doubling pick at the pick (replOrderPick); dealAsked applies the last
// remaining candidate and aligns the recomputed pass's copy of the event,
// which replCombatGen dealt in the fixed additions-first order (replPlus).

// roPlusPend reports that the ask in progress collected a plus-N addition
// beside a non-commuting amount changer (a candidate with no bits is a
// doubling): its picks are applied as made, not pre-applied.
func (s *State) roPlusPend() bool {
	ro := &s.g.ro
	if ro.plus == 0 {
		return false
	}
	for i := uint(0); i < uint(ro.ac); i++ {
		b := uint8(1) << i
		if ro.plus&b == 0 && ro.prev&b == 0 && ro.shld&b == 0 && ro.pnx&b == 0 {
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
// amount the picked order dealt: replCombatGen applied the additions before
// the doublings (replPlus), while the walk applies each pick as made
// (CR 616.1e), so the amounts differ when the doubling was picked first.
// combatEvLess shrinks the accounting; the mirror arm grows it.
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
	code := PlayerCode(ro.apl)
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
