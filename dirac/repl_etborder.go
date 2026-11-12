package dirac

// The CR 616.1 order between a cast spell's effect-carried enters-with
// counters and the entrant's own Updated enter body (Strife Scholar cast
// under a Wildgrowth Archaic's SpellCast trigger: the trigger's Effect
// registers a live Event$ Moved replacement that puts the counters on, the
// card's own "enters prepared" AlterAttribute is not a counter body, so
// gorge composes the entry's Updated replacements into two candidates that
// do not commute and asks the affected controller which applies first —
// replacement_updated.go composeUpdatedReplacements — with the spell still
// on the stack, CR 616.1c).
//
// The kernel records the rider's source on the spell's stack entry when it
// stamps the pending count (ctr_effectentry.go: etbSrc, stamped only when
// the entrant's own body is exactly one prepared line, etbOwnPrepares), and
// poses the same KReplacement ask before the move. The answer decides more
// than the order: gorge emits the move, then applies the chosen body, and
// only re-collects the rest afterwards (resumeUpdatedComposition) — and the
// rider's effect ends with the very move it upgrades (ExileOnMoved$ Stack,
// engine/rules/layers_effect.go effectMoveSweep: the sweep runs inside the
// composition, before any body). So when the own body applies first the
// rider's replacement is no longer there to re-collect: no counters. When
// the rider is chosen first its body runs without a re-check, and the own
// body (a printed replacement that still matches) applies after it. The
// kernel answers the same way: the rider's pick keeps the pending count
// (enterGen places it before the own programs), the own body's pick drops
// it.
//
// Not modelled, and left exactly as before: a second effect beside a stamped
// one stalls at the stamp (three candidates gorge asks over); a printed
// enters-tapped or enters-with rider on the card, or a second own line
// beside the prepared one keep the entry unstamped and unasked
// (etbOwnPrepares, entryComp) — a longer candidate list gorge would ask
// over, which the silent application is today. One other source's enter
// replacement beside the prepared line, with no rider stamped, does ask:
// etbOtherOrderPark poses the entrant and that source, and the entry path
// runs unchanged after the answer because both bodies apply in either order.

// etbEntryOrderPark poses the ask for the permanent spell o, which ctrl is
// about to put onto the battlefield, when its pending rider count names a
// source. It reports whether the resolution suspended on the ask. The
// resolve site (resolve.go) reads the stamped source off the copied entry
// first, so an entry with no rider pending pays one load there and never
// reaches this function.
func (s *State) etbEntryOrderPark(k *prep, o Obj, ctrl uint8) bool {
	if !k.gen.anyEtbOwnOther || s.g.pre.kind != 0 || s.g.pre.state != 0 {
		return false // the table has no such body, or an ask is already parked
	}
	e := &s.stack[s.sp-1]
	if e.kind != seSpell || e.obj != o || e.etbSrc == 0 || e.spc&spcEtbMask == 0 {
		return false // no rider count, or its source was not stamped
	}
	if c, _ := s.entryComp(k, o, ctrl, NoSlot, false); c.n > 0 || c.over {
		// another source's enter replacement matches the entrant: gorge asks
		// over three or more candidates, this ask is not that one
		s.stall(StallEntryOrder)
		return true
	}
	s.g.pre = preAsk{obj: o, kind: akEtbOrder, state: 1}
	s.res.active = false
	s.g.ctlRet = ctlPriority
	s.ctl = ctlGenAsk
	return true
}

// etbOtherOrderPark poses the same two-candidate ask when no rider was
// stamped on the spell: the entrant's own prepared line is the only body on
// its side and exactly one other source's enter replacement matches it
// (Authority of the Consuls beside a prepared creature, the Group A repros).
// Both bodies apply in either order, so the answer only settles the order
// gorge asks (CR 616.1) and the entry path runs as it does today. It reports
// whether the resolution suspended on the ask.
func (s *State) etbOtherOrderPark(k *prep, o Obj, ctrl uint8) bool {
	if !k.gen.anyEtbOwnOther || s.g.pre.kind != 0 || s.g.pre.state != 0 {
		return false // the table has no such body, or an ask is already parked
	}
	if !k.gen.has(s.objs[o].card, gfEtbOrder) {
		return false // the entrant's own line is not the one prepared body
	}
	e := &s.stack[s.sp-1]
	if e.kind != seSpell || e.obj != o || e.etbSrc != 0 || e.spc&spcEtbMask != 0 {
		return false // a rider count or an answered entry order is already on it
	}
	c, _ := s.entryComp(k, o, ctrl, NoSlot, false)
	if c.n > 1 || c.over {
		// more than one other source matches: gorge asks over three or more
		// candidates, this ask is not that one
		s.stall(StallEntryOrder)
		return true
	}
	if c.n != 1 || c.hard || c.untap != 0 || c.selfLive != 1 {
		return false // no competition, or one of a shape this ask does not order
	}
	e.etbSrc = c.cand[0] // reuse etbOrderOptions/etbOrderAnswer; the rider count is empty
	s.g.pre = preAsk{obj: o, kind: akEtbOrder, state: 1}
	s.res.active = false
	s.g.ctlRet = ctlPriority
	s.ctl = ctlGenAsk
	return true
}

// etbOrderOptions fills the ask: the entrant's own body (the entering spell)
// and the effect's source, the candidates gorge composes. gorge lists them
// in its scan order and the adapter canon-sorts both lists, so only the set
// has to match.
func (s *State) etbOrderOptions(k *prep, d *Decision) {
	e := &s.stack[s.sp-1]
	d.add(MakeAction(AkChoose, uint8(s.g.pre.obj), 0, 0))
	d.add(MakeAction(AkChoose, uint8(e.etbSrc), 0, 1))
}

// etbOrderAnswer consumes the answer and goes on with the resolution: a pick
// of the rider's source keeps the pending count (the counters are placed in
// enterGen, the own body runs after them in runOwnEnterPrograms); a pick of
// the entrant's own body drops the count — the rider's effect ended with the
// move it upgrades, so gorge's CR 616.1f re-collection finds nothing to
// apply. The answered ask stays parked as the marker that the entry's order
// is settled, until the resolution finishes (cloneEntryOrderAnswer's
// protocol).
func (s *State) etbOrderAnswer(k *prep, a Action) {
	p := &s.g.pre
	e := &s.stack[s.sp-1]
	if Obj(a.Source()) != e.etbSrc {
		e.spc &^= spcEtbMask // the own body applies first: the rider is dropped
	}
	p.state = 2
	s.ctl = ctlPriority
	s.resolveTop(k)
	if s.g.pre.state != 1 {
		s.g.pre = preAsk{}
	}
}
