package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// destroyAllRemember is OpDestroyAll with RememberDestroyed$ (Starfall
// Invocation): gorge effDestroyAll remembers each victim, in its
// battlefield walk order, that actually reached the graveyard. It is also
// the C&2 form (NoRegen$ True: no regeneration replacement).
func (s *State) destroyAllRemember(k *prep, in *kop.Instr) {
	e := &s.res.e
	src := s.objRef(k, kop.RefSelf)
	var hit slotMask
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		if s.matchPerm(k, in.F, sl, e.ctrl, src) && s.kw(k, sl)&kop.KwIndestructible == 0 {
			hit = hit.with(sl)
		}
	}
	hit = s.destroyHealAll(k, hit)
	if in.C&2 == 0 && s.anyRegen(hit) {
		hit = s.regenAll(hit)
	}
	if k.gen != nil && k.gen.anyRepl&rbShield != 0 {
		hit = s.shieldAll(hit) // CR 122.1c: NoRegen$ suppresses regeneration only
	}
	var ord [MaxPerm]Slot
	n := s.listenerOrder(hit, &ord)
	var objs [MaxPerm]Obj
	for i := 0; i < n; i++ {
		objs[i] = s.perms[ord[i]].obj
	}
	s.leaveAll(k, hit, kop.ZoneGraveyard)
	if in.C&1 == 0 { // seat-ok: an instruction flag bit
		return
	}
	for _, o := range objs[:n] {
		if s.objs[o].zone == kop.ZoneGraveyard {
			s.remember(o)
		} else {
			// The victims a CR 616.1 order ask still holds move later; remember
			// one only when its parked move reaches the graveyard (runParked).
			s.parkMassRemember(o)
		}
	}
}

// parkMassRemember marks o's parked mass move as a RememberDestroyed$ victim
// that is still on the battlefield because an order ask holds it. False when o
// is not in the queue.
func (s *State) parkMassRemember(o Obj) bool {
	for i := range s.cold.parked {
		if r := &s.cold.parked[i]; r.kind == parkMass && r.obj == o {
			r.fl |= parkRemember
			return true
		}
	}
	return false
}

// discardRemember is RememberDiscarded$ (gorge discardAndRememberEvent):
// a card discarded by the resolving OpDiscard with B.N&1 joins the
// remembered set. The resolving instruction is the one at res.pc (the
// discard ask suspends on it); a discard outside a resolution, or by
// another op, does not remember.
//
//go:noinline
func (s *State) discardRemember(k *prep, o Obj) {
	if !s.res.active {
		return
	}
	prog := s.prog(k)
	if int(s.res.pc) >= len(prog) {
		return
	}
	if in := &prog[s.res.pc]; in.Op == kop.OpDiscard && in.B.N&1 != 0 || unlessRemembers(in) { // seat-ok: an instruction flag bit
		s.remember(o)
	}
}

// tableRemembersDiscards reports whether some program of t has an
// OpDiscard with RememberDiscarded$ (genPrep.anyRemDisc).
func tableRemembersDiscards(t *kop.Table) bool {
	return tableRemembersOp(t, kop.OpDiscard) || tableRemembersUnlessDisc(t) // trig_reflexiveremembered.go
}

// tableRemembersOp reports whether some program of t has an op with the
// B.N&1 remember bit (genPrep.anyRemDisc / anyRemSac).
func tableRemembersOp(t *kop.Table, op kop.Op) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == op && in.B.N&1 != 0 { // seat-ok: an instruction flag bit
					return true
				}
			}
		}
	}
	return false
}

// sacRemember is RememberSacrificed$ (gorge effSacrifice): a permanent
// sacrificed by the resolving OpSacrifice with B.N&1 joins the remembered
// set (the object keeps its id into the graveyard; a token's id still
// counts, as gorge's remembered entry does). fireSac runs it before the
// permanent leaves.
//
//go:noinline
func (s *State) sacRemember(k *prep, sl Slot) {
	if !s.res.active {
		return
	}
	prog := s.prog(k)
	if int(s.res.pc) >= len(prog) {
		return
	}
	if in := &prog[s.res.pc]; in.Op == kop.OpSacrifice && in.B.N&1 != 0 { // seat-ok: an instruction flag bit
		if in.B.N&(kop.SacSumPower|kop.SacSumToughness|kop.SacSumMV) != 0 {
			s.remLKAdd(k, sl, int(in.B.N)) // RememberedLKI$CardPower/Toughness (zone_rememberlki.go)
		}
		s.remember(s.perms[sl].obj)
	} else if in.Op == kop.OpIfPaid && in.C&0xF == kop.PaySac &&
		in.B.N&(kop.SacSumPower|kop.SacSumToughness|kop.SacSumMV) != 0 {
		// a printed trigger's own IfPaid Sacrifice cost
		// (Venom, Lethal Protector; Bolg of the North). gorge's trigger_cost
		// sacrifice feeds Ctx.Sacrificed, not the remembered set, so only the
		// characteristic accumulator is written here.
		s.remLKAdd(k, sl, int(in.B.N))
	}
}

// digUntilRemember is DigUntil's RememberFound$ (gorge effDigUntil): the
// found cards replace the resolution's remembered set, accumulated across
// the players walked (first: the walk's first player clears it).
//
//go:noinline
func (s *State) digUntilRemember(first bool, found []Obj) {
	if first {
		s.g.nrem, s.g.remCnt = 0, 0
	}
	for _, o := range found {
		s.remember(o)
	}
}

// digRememberRevealed is DigUntil's RememberRevealed$ (DuRememberRevealed,
// OpDigUntil C>>3): every scanned card — found and revealed, in scan order —
// joins the remembered set, per player, without clearing (gorge effDigUntil's
// RememberRevealed arm keeps its append semantics across the player walk).
//
//go:noinline
func (s *State) digRememberRevealed(revealed []Obj) {
	for _, o := range revealed {
		s.remember(o)
	}
}

// digImprintFound is DigUntil's ImprintFound$ (DuImprintFound): the found
// cards join the chosen set, deduped (gorge's seek-found Imprint accumulates
// ImprintFound across the player walk; only a later Effect reads it).
//
//go:noinline
func (s *State) digImprintFound(found []Obj) {
	for _, o := range found {
		s.imprintChosen(o)
	}
}

// imprintChosen appends o to the chosen set unless it is already there.
func (s *State) imprintChosen(o Obj) {
	if !s.inSet(&s.g.chosen, s.g.nchosen, o) && s.g.nchosen < maxRem {
		s.g.chosen[s.g.nchosen] = o
		s.g.nchosen++
	}
}

// Destroy's RememberDestroyed$ (OpDestroy C&2) and
// PutCounter's RememberAmount$ (OpPutCounter B=1). Cold code, reached
// through one operand test at each interp.go site.

// DebugRem2Always is TestRememberDestroyedAmountMutation's switch: RememberDestroyed
// remembers the target even when it was not destroyed, and RememberAmount
// counts CounterNum$ instead of the landed count. Never set outside a test.
var DebugRem2Always bool

// destroyRemember is OpDestroy with RememberDestroyed$ True (gorge
// effDestroy, engine/effects/zone_bulk.go): the target joins the
// remembered set only when the destroy put it into a graveyard (not an
// indestructible, regenerated, shield-saved or replaced-away one). C&1 is
// NoRegen$.
//
//go:noinline
func (s *State) destroyRemember(k *prep, sl Slot, c int16) {
	o := s.perms[sl].obj
	if c&1 != 0 { // seat-ok: the NoRegen flag bit
		s.destroyNoRegen(k, sl)
	} else {
		s.destroy(k, sl)
	}
	if DebugRem2Always || s.objs[o].zone == kop.ZoneGraveyard {
		s.remember(o)
		if c&4 != 0 {
			s.markRemLKI(o) // RememberLKI$: a read takes the last controller
		}
	}
}

// putCounterRemAmount is OpPutCounter with RememberAmount$ True (gorge
// counters_put.go putCounterLanded: the recipient is remembered once per
// counter that landed): the landed count n joins the resolution's
// remembered count, the remCnt channel ExRememberedCount sums.
//
//go:noinline
func (s *State) putCounterRemAmount(k *prep, in *kop.Instr, n int) {
	if DebugRem2Always {
		n = s.value(k, in.A)
	}
	if n > 0 {
		s.g.remCnt += uint16(n)
	}
}
