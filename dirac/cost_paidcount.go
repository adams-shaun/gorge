package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The paid-count of an optional additional cost, per object: how many times
// the spell's Squad or Replicate payment was made (CR 702.157a, 702.56a), or 1
// when its Conspire cost was (CR 702.78a). Count$SquadPaid, Count$ReplicatePaid
// and Count$Conspired read it (kop.ExKicked N 2; kgen vPaidN).
//
//   - coldPaidN is a cold region, one byte per object, allocated only for a
//     table with a Squad, Replicate or Conspire card (usePaidN), so every
//     other table keeps its layout.
//   - a Squad/Replicate cast stores its payment count beside the build's own
//     cold.mkObj (setMultikick), which still bounds and prices the ask; a
//     conspired cast stores 1 as the spell goes on the stack, before its cast
//     triggers are read (notePaidConspire).
//   - it ends where cold.mkObj ends: a card cast again (beginCast), an entry
//     that is not the resolving spell (turn_gen.go), a reused object id
//     (zone_objreuse.go).
//   - the read mirrors timesKicked's provenance: a spell's own cast trigger
//     reads the stack entry's byte, a Squad ETB trigger the count it captured
//     as it was queued, a Squad permanent its object's byte. A copy that was
//     never cast reads 0.
var coldPaidN = newCold[uint8]("paidN", func(g *genPrep) int {
	if g.uses.has(usePaidN) {
		return MaxObj
	}
	return 0
})

// usePaidN: the table has a Squad, Replicate or Conspire card.
var usePaidN = newGenUse(tableAnyPaidN)

// tableAnyPaidN reports whether any card of t pays a Squad, Replicate or
// Conspire cost.
func tableAnyPaidN(t *kop.Table) bool {
	for i := range t.Cards {
		c := &t.Cards[i]
		if c.Conspire || (c.HasMultikicker && (c.RepeatedKind == kop.RepeatSquad || c.RepeatedKind == kop.RepeatReplicate)) {
			return true
		}
	}
	return false
}

// setPaidN records spell e's paid count.
func (s *State) setPaidN(e *stackEntry, n int) {
	if pn := coldPaidN.of(s); int(e.obj) < len(pn) {
		pn[e.obj] = uint8(n)
	}
}

// clearPaidN ends object o's paid count.
func (s *State) clearPaidN(o Obj) {
	if pn := coldPaidN.of(s); int(o) < len(pn) {
		pn[o] = 0
	}
}

// DebugPaidConspireOff is the mutation hook: a paid conspire stores no count,
// so a printed Conspire trigger's Count$Conspired reads 0 and the lockstep must
// diverge.
var DebugPaidConspireOff bool

// notePaidConspire stores 1 for a spell whose conspire election was paid (both
// creatures tapped, CR 702.78a): the cast trigger's Count$Conspired reads it.
func (s *State) notePaidConspire(k *prep, e *stackEntry) {
	if k.gen != nil && k.gen.anyConspire && s.bld().cas == casConspPaid && !DebugPaidConspireOff {
		s.setPaidN(e, 1)
	}
}

// paidCount is the paid count a Count$SquadPaid / ReplicatePaid / Conspired
// read finds in ctx.
func (s *State) paidCount(ctx evalCtx) int {
	pn := coldPaidN.of(s)
	if pn == nil {
		return 0
	}
	// a Squad ETB trigger captured the count as it was queued (the source can
	// leave the battlefield first: Wasteland Raider's own sacrifice)
	if e := ctx.e; e != nil && e.kind == seTrigger && e.defSeat != 0 {
		if k := s.k(); k != nil && int(e.card) < len(k.t.Cards) && capturesKickCount(&k.t.Cards[e.card]) &&
			int(e.ab) < len(k.t.Cards[e.card].Abilities) {
			if a := &k.t.Cards[e.card].Abilities[e.ab]; a.Trig.Event == kop.TrigETB && a.Trig.Flags&kop.TfSelf != 0 {
				return int(e.defSeat)
			}
		}
	}
	if ctx.src != NoObj && int(ctx.src) < len(pn) {
		if z := s.objs[ctx.src].zone; z == kop.ZoneBattlefield || z == kop.ZoneStack {
			return int(pn[ctx.src])
		}
	}
	if e := ctx.e; e != nil && e.kind == seSpell && int(e.obj) < len(pn) {
		return int(pn[e.obj])
	}
	return 0
}
