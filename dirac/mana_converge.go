package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// the colours of mana spent to cast a spell (CR 107.4f-family
// converge; gorge Object.ConvergeColours). gorge captures the payment's
// per-colour spend delta (rules/cast_commit.go payManaCastSpent, then
// convergeColours: the colours with a nonzero delta, a generic pip paid
// from a coloured unit counting toward that colour) and folds it from the
// pay-time FlagConverged CastInfo (events/apply_cast.go). The kernel
// tallies the same delta into the cast's stack entry (stackEntry.spc) at
// each pool spend of the build (payFromPoolRider, the funnel of payRider,
// payRestrSpend, payCastWindow, the convoke, painland and restricted
// payments): the pool is lockstep-equal to gorge's, so the units taken are
// gorge's. A stack copy zeroes it (mintSpellCopy).
//
// Readers:
//   - Count$Converge (ExConverge) in the spell's chain and in its
//     K:etbCounter enters replacement: the resolving spell entry, which is
//     the object entering (gorge reads c.Source; stack->battlefield keeps
//     the count, apply_zone.go). An object entering uncast reads 0.
//   - Count$Converge in a self-ETB trigger (Tf3ConvergeBound): bound into
//     the pending trigger's x as it triggers; gorge reads the source live
//     (effects/count_body_paid.go) and clears the count when it leaves the
//     battlefield, so it reads 0 once the source has left (as Tf2SpentBound).
//   - TriggeredCard$Converge in a SpellCast trigger (Tf3CastConverge): the
//     trigger amount, the spell's count as the trigger fires (gorge's
//     fire-time TriggerConverge snapshot, rules/trigger_referents.go).
//
// gorge's planner declines a converge face and every cast while a
// TriggeredCard$Converge reader is on a battlefield ("shape:
// mana_spent_reader"): the face is CardDef.SpendRead (kgen) and the reader
// is gfSpendRd (spendReaderOut).

// DebugConvergeGenericColourless is TestConvergeMutation's switch: a
// generic pip paid from a coloured unit is not counted toward its colour
// (only the coloured pips are). It must make the lockstep diverge.
var DebugConvergeGenericColourless bool

// prepConverge sets genPrep.anyColSpent (and the planner's reader flag for
// a TriggeredCard$Converge listener).
func prepConverge(t *kop.Table, g *genPrep) {
	for i := range t.Exprs {
		if t.Exprs[i].Kind == kop.ExConverge {
			g.anyColSpent = true
		}
	}
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if a := &t.Cards[i].Abilities[j]; a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigSpellCast && a.Trig.Flags3&kop.Tf3CastConverge != 0 {
				g.setFlag(CardID(i), gfSpendRd) // gorge triggeredConvergeReaderOut: the planner declines
				g.anyCastSpent, g.anyColSpent = true, true
			}
		}
	}
}

// payFromPoolSpc is payFromPoolRider while a cast is built in a table
// reading the colours spent: the pool's per-colour drop is added to the
// spell's build entry.
//
//go:noinline
func (s *State) payFromPoolSpc(p uint8, cost *kop.ManaCost, x int, anyColor bool) {
	pl := &s.pl[p]
	pre := pl.pool
	if anyColor {
		s.payFromPoolAnyColor(p, cost, x)
	} else {
		s.payFromPool(p, cost, x)
	}
	b := s.bld()
	if b.e.kind != seSpell {
		return
	}
	for c := kop.W; c <= kop.G; c++ {
		d := int(pre[c]) - int(pl.pool[c])
		if DebugConvergeGenericColourless {
			d = min(d, int(cost.Pips[c]))
		}
		if d > 0 {
			b.e.spc = spcAdd(b.e.spc, c, d)
		}
	}
}

// spcAdd adds n units of colour c (W..G) to spc, saturating at 3.
func spcAdd(spc uint16, c, n int) uint16 {
	sh := uint(2 * c)
	v := min(int(spc>>sh&3)+n, 3)
	return spc&^(3<<sh) | uint16(v)<<sh
}

// spcColours is the number of colours spent (Converge).
func spcColours(spc uint16) int {
	n := 0
	for c := 0; c < 5; c++ {
		if spc>>(2*c)&3 != 0 {
			n++
		}
	}
	return n
}

// spellSpc is the colours spent to cast the spell object o is or is
// becoming (the resolving spell, else a spell entry on the stack), 0 if it
// was never cast.
func (s *State) spellSpc(o Obj) uint16 {
	if s.res.active && s.res.e.kind == seSpell && s.res.e.obj == o {
		return s.res.e.spc
	}
	for i := int(s.sp) - 1; i >= 0; i-- {
		if e := &s.stack[i]; e.kind == seSpell && e.obj == o {
			return e.spc
		}
	}
	return 0
}

// bindTrigConverge binds the pending trigger just queued (Trigger.Flags3):
// a self-ETB trigger's x (Tf3ConvergeBound), a SpellCast trigger's amount
// (Tf3CastConverge).
//
//go:noinline
func (s *State) bindTrigConverge(a *kop.Ability, tobj Obj) {
	if s.npend == 0 || s.pend[s.npend-1].trigObj != tobj || tobj == NoObj {
		return
	}
	n := spcColours(s.spellSpc(tobj))
	if a.Trig.Flags3&kop.Tf3ConvergeBound != 0 {
		s.pend[s.npend-1].x = objWord(uint8(n))
	}
	if a.Trig.Flags3&kop.Tf3CastConverge != 0 {
		s.pend[s.npend-1].trigAmt = int16(n)
	}
}

// exConverge is ExConverge in ctx.
//
//go:noinline
func (s *State) exConverge(k *prep, ctx evalCtx) int {
	e := ctx.e
	if e == nil {
		return 0
	}
	switch e.kind {
	case seSpell:
		if r := s.g.replaced; r != NoObj && r != e.obj {
			return 0 // another object entering during this spell's resolution was not cast
		}
		return spcColours(e.spc)
	case seTrigger:
		if int(e.ab) >= len(k.t.Cards[e.card].Abilities) || k.t.Cards[e.card].Abilities[e.ab].Trig.Flags3&kop.Tf3ConvergeBound == 0 {
			return 0
		}
		// what it was as it left (CR 608.2h); a new object if it came back
		if ob := &s.objs[e.obj]; ob.zone == kop.ZoneBattlefield && ob.ver != e.srcVer {
			return 0
		}
		return int(e.x)
	}
	return 0
}

// The kinds of converge read ConvergeRead reports (oracle counts).
const (
	ConvergeSpell    uint8 = iota // a cast spell entry: its colours spent
	ConvergeCopy                  // a stack copy of a spell (never cast: 0)
	ConvergeCastTrig              // a Tf3CastConverge trigger: its amount
	ConvergeETB                   // a Tf3ConvergeBound trigger whose source is still on the battlefield: its bound x
	ConvergeETBLeft               // a Tf3ConvergeBound trigger whose source has left (reads 0); n is the bound x
)

// ConvergeRead reports stack entry i's converge read for the oracle
// counts: the count n and the kind of read, ok false for an entry that
// reads none (or a table that reads no colours spent).
func (s *State) ConvergeRead(i int) (n int, kind uint8, ok bool) {
	k := s.k()
	if k.gen == nil || !k.gen.anyColSpent || i < 0 || i >= int(s.sp) {
		return 0, 0, false
	}
	e := &s.stack[i]
	switch e.kind {
	case seSpell:
		if s.isSpellCopy(e.obj) {
			return spcColours(e.spc), ConvergeCopy, true
		}
		return spcColours(e.spc), ConvergeSpell, true
	case seTrigger:
		if int(e.ab) >= len(k.t.Cards[e.card].Abilities) {
			return 0, 0, false
		}
		f := k.t.Cards[e.card].Abilities[e.ab].Trig.Flags3
		if f&kop.Tf3CastConverge != 0 {
			return int(e.trigAmt), ConvergeCastTrig, true
		}
		if f&kop.Tf3ConvergeBound != 0 {
			if ob := &s.objs[e.obj]; ob.zone != kop.ZoneBattlefield || ob.ver != e.srcVer {
				return int(e.x), ConvergeETBLeft, true
			}
			return int(e.x), ConvergeETB, true
		}
	}
	return 0, 0, false
}
