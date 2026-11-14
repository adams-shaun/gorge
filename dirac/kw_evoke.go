package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Evoke (CR 702.74a) and the Adamant enters gate (CR 702.5);
// cold, reached only when the table has a card with printed evoke or an
// ExAdamant expression (genPrep.anyEvoke).
//
// Evoke: the hand offers an alternative cast at CardDef.EvokeCost (gorge's
// "evoked" option, altcast_modes.go altEvoke: a potential play, offered like
// warp and impending before the plain affordability gate). It rides AkCast
// mode 0 with castEvokeTgt in the target byte; legal.go sets genState.impCast
// (a card has either impending or evoke, never both), and beginCast marks the
// build entry spcEvoked, bit 10 of stackEntry.spc (0 B: the bit
// reserved). A stack copy clears spc, so a copy is not evoked. buildCost
// prices EvokeCost; the cast is lowered like a warp cast (cast.go).
//
// The "sacrifice it" trigger (gorge altcast.go altCostEnter): queued as the
// evoked permanent enters, AFTER the card's own enters triggers (gorge's
// checkTriggers face scan runs before the altCostEnter hook), as an abDelayed
// pending trigger whose action is OpSacrifice. It resolves like a delayed
// sacrifice: nothing happens unless the source is still that object on the
// battlefield (gorge stack.go: SourceIncarnation).
//
// Adamant: ExAdamant reads count(colour) >= n of the cast's spend (spc's 2-bit
// saturating counts). As the self-ETB trigger is queued the colour's count
// is bound into the pending trigger's x (evokeEnter); the intervening-if
// recheck at resolution reads what gorge reads (rules/stack.go
// bindSourceLKI + effects adamantSpendVector): the departure snapshot, i.e.
// the bound cast-time count, while the source is off the battlefield, and
// the live object while it is on it - the same object (the bound count) or a
// new one, whose spend leaving wiped (0). A new object that was cast again
// would read its own new spend in gorge; the kernel keeps no spend for a
// permanent, so it reads 0 (untested: the pool has no way to return the
// source to hand with its triggers still waiting).

// spcEvoked in stackEntry.spc: the spell was cast for its evoke cost.
const spcEvoked uint16 = 1 << 10

// castEvokeTgt in an AkCast's target byte (mode 0, from the hand) is the
// cast for the card's evoke cost (offerEvokeHand).
const castEvokeTgt = 3

// EvokeStats counts what a lockstep run exercised (TestEvokeLockstep sets
// DebugEvokeStats; nil costs one test on this cold path).
type EvokeStats struct {
	Evoked, Hard         int // evoke-card permanents entered cast for / not for the evoke cost
	GateTrue, GateFalse  int // Adamant intervening-ifs read as a self-ETB trigger is queued
	SnapHolds, SnapFails int // rechecks of a trigger whose source left the battlefield: the cast-time count held / did not
}

// DebugEvokeStats, when non-nil, tallies the evoke and Adamant reads.
var DebugEvokeStats *EvokeStats

// DebugEvokeNoSac skips the evoke sacrifice trigger (TestEvokeMutation: the
// lockstep must catch it).
var DebugEvokeNoSac bool

// DebugAdamantLiveOnly makes the Adamant recheck read the live, wiped spend
// once the source has left the battlefield: gorge before
// (TestEvokeLKIMutation: the lockstep must catch it).
var DebugAdamantLiveOnly bool

// prepEvoke sets genPrep.anyEvoke (and the payment tally) for a table with
// an Adamant read; a card with evoke sets anyEvoke in the card loop.
func prepEvoke(t *kop.Table, g *genPrep) {
	for i := range t.Exprs {
		if t.Exprs[i].Kind == kop.ExAdamant {
			g.anyEvoke, g.anyColSpent = true, true
		}
	}
}

// evoked reports whether e is a spell cast for its evoke cost.
func (e *stackEntry) evoked() bool { return e.kind == seSpell && e.spc&spcEvoked != 0 }

// offerEvokeHand offers hand card o's evoked cast (gorge's "evoked" option).
// The caller has checked the card's timing and the cast prohibition; it runs
// before the plain affordability gate, like impending's.
//
//go:noinline
func (s *State) offerEvokeHand(k *prep, d *Decision, p uint8, o Obj, c CardID, av *avail) {
	cd := &k.t.Cards[c]
	ec := s.adjustCost(k, p, c, o, cd.EvokeCost) // the reduction applies to an alternative cost too (CR 601.2f)
	if len(k.gen.grantPC) != 0 && s.g.availX&availWalk != 0 {
		// gorge's potential walk counts a restricted mana ability only for a
		// payment its restriction admits (potentialRestrictionAdmits): a
		// hand-barred grant puts no cast on the walk grantCap reads
		var ha avail
		s.walkHandAvail(k, p, payDesc{class: pdSpell, card: c, src: NoSlot, og: s.restrOriginOfObj(o)}, &ha)
		av = &ha
	}
	if !av.affords(&ec, pipMaskOf(&ec), 0) && (!s.rbAny(p) && !k.restrSrc || !s.restrFallback(k, p, c, s.restrOriginOfObj(o), &ec, av)) {
		return
	}
	if !s.spellTargetsFeasible(k, c, p) || !s.spellCostsFeasible(k, c, p, o) || !s.altAddCastable(k, c, p, o, &ec, av) {
		return
	}
	if s.castHeldOut(o) {
		return
	}
	d.add(MakeAction(AkCast, TCode(o), castEvokeTgt, 0))
}

// adamantOf is the ExAdamant expression the ability's trigger condition
// reads, if any.
func adamantOf(t *kop.Table, id kop.CondID, depth int) (*kop.Expr, bool) {
	if id == 0 || depth > 4 {
		return nil, false
	}
	c := &t.Conds[id]
	if c.Kind == kop.CondAnd {
		for _, sub := range c.Sub {
			if x, ok := adamantOf(t, sub, depth+1); ok {
				return x, true
			}
		}
		return nil, false
	}
	for _, v := range [2]kop.ValueRef{c.A, c.B} {
		if v.K == kop.VExpr && t.Exprs[v.N].Kind == kop.ExAdamant {
			return &t.Exprs[v.N], true
		}
	}
	return nil, false
}

// abAdamantOf is the ExAdamant expression ability a reads: its trigger
// condition's (an Adamant intervening-if), else a body SkipUnless gate's
// (ConditionManaSpent$ / ConditionManaNotSpent$, kgen lower_manaspentcond.go;
// Azorius Herald). Either reads the cast's spend bound into the trigger; the
// body gate's carries kop.AdamantLiveRead (exAdamant).
func abAdamantOf(t *kop.Table, a *kop.Ability) (*kop.Expr, bool) {
	if x, ok := adamantOf(t, a.Trig.Cond, 0); ok {
		return x, true
	}
	for i := range a.Prog {
		if in := &a.Prog[i]; in.Op == kop.OpSkipUnless {
			if x, ok := adamantOf(t, kop.CondID(in.C), 0); ok {
				return x, true
			}
		}
	}
	return nil, false
}

// spcCount is colour col's (W U B R G = 0-4) saturating spend count in spc.
func spcCount(spc uint16, col uint) int { return int(spc >> (2 * col) & 3) }

// evokeEnter runs after the resolving permanent spell e entered: it binds
// the cast-time Adamant count into the self-ETB triggers it just queued,
// then queues the evoked permanent's sacrifice trigger behind them.
//
//go:noinline
func (s *State) evokeEnter(k *prep, e *stackEntry) {
	ver := s.objs[e.obj].ver
	for i := 0; i < int(s.npend); i++ {
		pt := &s.pend[i]
		if pt.obj != e.obj || pt.srcVer != ver || pt.ab >= abCascade {
			continue
		}
		if x, ok := abAdamantOf(k.t, &k.t.Cards[pt.card].Abilities[pt.ab]); ok {
			pt.x = objWord(spcCount(e.spc, uint(x.N&7)))
		}
	}
	if st := DebugEvokeStats; st != nil && k.t.Cards[e.card].HasEvoke {
		if e.spc&spcEvoked != 0 {
			st.Evoked++
		} else {
			st.Hard++
		}
	}
	if e.spc&spcEvoked != 0 && !DebugEvokeNoSac {
		np := s.npend
		s.enqueue(e.obj, ver, e.card, abDelayed, e.ctrl, e.obj, e.ctrl, int16(kop.OpSacrifice))
		s.panharmKeyword(k, s.objs[e.obj].slot, kop.TrigETB, e.obj, np)
	}
}

// exAdamant is ExAdamant in ctx. Queueing a self-ETB trigger (ctx.e nil)
// reads the entering spell's spend; the recheck at resolution (a trigger
// entry) reads the bound count as gorge's snapshot-or-live rule does, or for
// a kop.AdamantLiveRead body gate only while the source is still the same
// battlefield object (gorge conditionManaSpentMet reads the live, wiped spend).
//
//go:noinline
func (s *State) exAdamant(k *prep, x *kop.Expr, ctx evalCtx) int {
	n := 0
	if e := ctx.e; e != nil && e.kind == seTrigger {
		ob := &s.objs[e.obj]
		stale := ob.zone == kop.ZoneBattlefield && ob.ver != e.srcVer || ob.zone != kop.ZoneBattlefield && (DebugAdamantLiveOnly || x.N&kop.AdamantLiveRead != 0)
		if !stale {
			n = int(e.x)
		}
		holds := n >= int(x.N>>3&3)
		if st := DebugEvokeStats; st != nil && ob.zone != kop.ZoneBattlefield {
			if holds {
				st.SnapHolds++
			} else {
				st.SnapFails++
			}
		}
		return s.evalV(k, x.Sub[b2i(!holds)], ctx)
	} else if ctx.src != NoObj {
		n = spcCount(s.spellSpc(ctx.src), uint(x.N&7))
	}
	holds := n >= int(x.N>>3&3)
	if st := DebugEvokeStats; st != nil {
		if holds {
			st.GateTrue++
		} else {
			st.GateFalse++
		}
	}
	return s.evalV(k, x.Sub[b2i(!holds)], ctx)
}
