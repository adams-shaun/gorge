package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// the combat-state filter words of the MTGA benchmark
// decks. Cold and out of line; every site is behind a genPrep bool, so a
// table without these filters never reaches it.
//
//   - F2BlockedBySrc (gorge blockedBySourceLKI, filter_blockhistory.go: the
//     candidate is attacking and the source is in its BlockedByThisTurn):
//     with the source on the battlefield, the source is blocking the
//     candidate; with the source gone (Tinder Wall's sacrifice cost) any
//     attacking creature, because the target was chosen while the source
//     blocked it and gorge's block history survives the source's departure
//     for the rest of the turn. The kernel keeps no per-combat history, so
//     a second combat in one turn (no pool card has one) could differ.
//   - F2DamagedBy (gorge DamagedBy: the source is in the candidate's
//     DamageTakenThisTurnBy) and F2CrewedSrc (gorge CrewedThisTurn: the
//     candidate's crew pairing names the source this turn) read the
//     source-mark records below.
//
// A source-mark record is one (kind, source object) pair and the slotMask
// of the battlefield permanents it marked this turn. gorge keys both
// histories by ObjID, which survives a zone change, so the record is keyed
// by the source's Obj (not its ver). A marked permanent that leaves the
// battlefield loses its mark (gorge clears both lists on the zone change,
// CR 400.7), and every record ends as a turn begins (gorge TurnChange).
// Only sources that a filter of the table can name are recorded: a card
// whose ability reads F2DamagedBy (genPrep.dmgBy), a vehicle whose
// ability reads F2CrewedSrc (genPrep.crewBy). A fourth record stalls
// (StallSrcMark).

const maxSrcMark = 3

// Source-mark kinds (srcMarks.kind; 0 = a free record).
const (
	smNone uint8 = iota
	smDamaged
	smCrewed
)

type srcMarks struct {
	kind [maxSrcMark]uint8
	src  [maxSrcMark]Obj
	m    [maxSrcMark]slotMask
}

// prepMcombat sets the source-mark prep: the cards whose abilities' filters
// read F2DamagedBy or F2CrewedSrc.
func prepMcombat(t *kop.Table, g *genPrep) {
	for i := range t.Overrides {
		g.anyWhileHost = g.anyWhileHost || t.Overrides[i].WhileHost
		g.anyWhileTapped = g.anyWhileTapped || t.Overrides[i].WhileTapped
		g.anyOvCantAtk = g.anyOvCantAtk || t.Overrides[i].CantAttack
	}
	for i := range t.Emblems {
		e := &t.Emblems[i]
		g.anyWhileHost = g.anyWhileHost || e.HostBound() || e.CaptureLapse
		g.anyEmbCtrl = g.anyEmbCtrl || e.AsLongAsControl
	}
	for i := range t.Cards {
		c := &t.Cards[i]
		for j := range c.Abilities {
			a := &c.Abilities[j]
			var f kop.FilterFlags2
			f |= chainFlags2(t, a.Trig.Filter) | chainFlags2(t, a.Trig.Filter2)
			for _, ts := range a.Targets {
				f |= chainFlags2(t, ts.Filter)
			}
			for _, in := range a.Prog {
				f |= chainFlags2(t, in.F)
				// a per-card expression may read
				// the Flags2 word through a ValueRef (Count$CrewSize is
				// an ExCount whose filter is F2CrewedSrc), not only
				// through an instruction filter.
				f |= valueFlags2(t, in.A, 0) | valueFlags2(t, in.B, 0)
			}
			if f&kop.F2DamagedBy != 0 {
				if g.dmgBy == nil {
					g.dmgBy = make([]bool, len(t.Cards))
				}
				g.dmgBy[i] = true
				g.anySrcMark = true
			}
			if f&kop.F2CrewedSrc != 0 {
				if g.crewBy == nil {
					g.crewBy = make([]bool, len(t.Cards))
				}
				g.crewBy[i] = true
				g.anySrcMark = true
			}
		}
	}
}

// valueFlags2 is the union of the Flags2 of every filter a ValueRef's
// expression reads (its F..F4, and its Sub operands), so a card whose
// per-card expression names F2CrewedSrc records its crews (Count$CrewSize).
func valueFlags2(t *kop.Table, v kop.ValueRef, depth int) kop.FilterFlags2 {
	if depth > 8 {
		return 0
	}
	if v.K == kop.VCountFilter {
		return chainFlags2(t, kop.FilterID(v.N)) // Count$Valid / Count$CrewSize
	}
	if v.K != kop.VExpr || int(v.N) < 0 || int(v.N) >= len(t.Exprs) {
		return 0
	}
	x := &t.Exprs[v.N]
	return chainFlags2(t, x.F) | chainFlags2(t, x.F2) | chainFlags2(t, x.F3) | chainFlags2(t, x.F4) |
		valueFlags2(t, x.Sub[0], depth+1) | valueFlags2(t, x.Sub[1], depth+1)
}

// chainFlags2 is the union of the Flags2 of filter f's Or chain.
func chainFlags2(t *kop.Table, f kop.FilterID) kop.FilterFlags2 {
	var out kop.FilterFlags2
	for n := 0; f != 0 && int(f) < len(t.Filters) && n < len(t.Filters); n++ {
		out |= t.Filters[f].Flags2
		f = t.Filters[f].Or
	}
	return out
}

// markSrc records that source object src marked the permanent at sl.
//
//go:noinline
func (s *State) markSrc(kind uint8, src Obj, sl Slot) {
	g := &s.g.marks
	free := -1
	for i := 0; i < maxSrcMark; i++ {
		if g.kind[i] == kind && g.src[i] == src {
			g.m[i] = g.m[i].with(sl)
			return
		}
		if g.kind[i] == smNone && free < 0 {
			free = i
		}
	}
	if free < 0 {
		s.capFull(StallSrcMark)
		return
	}
	g.kind[free], g.src[free], g.m[free] = kind, src, slotMask{}.with(sl)
}

// srcMarked reports whether source object src marked the permanent at sl.
func (s *State) srcMarked(kind uint8, src Obj, sl Slot) bool {
	g := &s.g.marks
	for i := 0; i < maxSrcMark; i++ {
		if g.kind[i] == kind && g.src[i] == src {
			return g.m[i].has(sl)
		}
	}
	return false
}

// srcMarkLeave drops the marks of the permanent at sl as it leaves the
// battlefield (CR 400.7).
//
//go:noinline
func (s *State) srcMarkLeave(sl Slot) {
	g := &s.g.marks
	for i := 0; i < maxSrcMark; i++ {
		g.m[i].clr(sl)
	}
}

// srcMarksTurn ends every record as a turn begins.
func (s *State) srcMarksTurn() {
	if s.g.marks.kind != [maxSrcMark]uint8{} {
		s.g.marks = srcMarks{}
	}
}

// noteDmgBy records that the permanent at src dealt damage to the
// permanent at dst (n > 0 dealt, after prevention), when src's card is a
// recorded source.
//
//go:noinline
func (s *State) noteDmgBy(k *prep, src, dst Slot) {
	if src == NoSlot || dst == NoSlot || !s.occ.has(src) || !s.occ.has(dst) {
		return
	}
	if c := s.cardAt(src); int(c) < len(k.gen.dmgBy) && k.gen.dmgBy[c] {
		s.markSrc(smDamaged, s.perms[src].obj, dst)
	}
}

// noteDmgByRec records a combat damage pass's events (dmgRec.ev: each
// (source, permanent) pair with what was dealt).
//
//go:noinline
func (s *State) noteDmgByRec(k *prep, r *dmgRec) {
	for i := 0; i < r.nev; i++ {
		if ev := &r.ev[i]; ev.n > 0 && ev.dst != NoSlot {
			s.noteDmgBy(k, ev.src, ev.dst)
		}
	}
}

// noteCrew records the creatures tapped to pay a Crew cost of the vehicle
// src (gorge's Crew fold: each crewer's CrewedVehicles names it).
//
//go:noinline
func (s *State) noteCrew(k *prep, src Obj, crewer Slot) {
	if src == NoObj || int(src) >= MaxObj {
		return
	}
	c := s.objs[src].card
	if o := &s.objs[src]; o.zone == kop.ZoneBattlefield {
		c = s.cardAt(o.slot) // a vehicle that is a copy counts its crew for the copied card's abilities (CR 707.2)
	}
	if int(c) < len(k.gen.crewBy) && k.gen.crewBy[c] {
		s.markSrc(smCrewed, src, crewer)
	} else if s.crewerRecorded(k, crewer) { // W7CrewedBySrc (filter_props.go)
		s.markSrc(smCrewed, src, crewer)
	}
}

// mcombatF2 checks the Flags2 conditions for the permanent
// at sl relative to the source at src (NoSlot: the source has left the
// battlefield).
//
//go:noinline
func (s *State) mcombatF2(k *prep, f kop.FilterFlags2, sl, src Slot) bool {
	p := &s.perms[sl]
	if f&kop.F2BlockedBySrc != 0 {
		if p.flags&pfAttacking == 0 {
			return false
		}
		if src != NoSlot && (s.perms[src].flags&pfBlocking == 0 || s.perms[src].blocking != sl) {
			return false
		}
	}
	if f&(kop.F2DamagedBy|kop.F2CrewedSrc) != 0 {
		if src == NoSlot {
			return false
		}
		o := s.perms[src].obj
		if f&kop.F2DamagedBy != 0 && !s.srcMarked(smDamaged, o, sl) {
			return false
		}
		if f&kop.F2CrewedSrc != 0 && (DebugCountHeads10Off || !s.srcMarked(smCrewed, o, sl)) {
			return false
		}
	}
	return true
}

// noteCrewPick records the creature at sl, tapped to pay the building
// ability's tap-power cost, when that cost is a Crew or a Saddle cost (gorge
// folds both markers through foldCrew into one per-turn list).
//
//go:noinline
func (s *State) noteCrewPick(k *prep, sl Slot) {
	if pc := s.buildPickCost(k); pc != nil && pc.Flags&(kop.CostCrew|kop.CostSaddle) != 0 {
		s.noteCrew(k, s.bld().e.obj, sl)
	}
}

// execMcombat runs the ops (the tail of execPrepare's
// chain); done=false means the op is not one of these.
//
//go:noinline
func (s *State) execMcombat(k *prep, in *kop.Instr) (ok, done bool) {
	switch in.Op {
	case kop.OpAtkPWGuard:
		if sl := s.objRef(k, kop.RefTriggerObj); sl == NoSlot || isObjTarget(s.perms[sl].atk) {
			s.capFull(StallDefenderPW)
		}
	default:
		return s.execUnread(k, in) // act_mayask.go
	}
	return true, true
}

// tokenAttacks is TokAttacking: the token just minted at sl, controlled by
// p, enters attacking (gorge foldTokenAttacks: IsAttacking, Attacking = the
// defender; nothing is declared, so no attack trigger fires, CR 508.4). The
// defender is p's opponent, or, for TokenAttacking$ True with two or more
// CR 508.4 candidates, the one p named to tokenDefenderAsk (gorge
// resolveEnteringDefender). gorge's fallback (enteringDefenderFallback) for
// TokenAttacking$ True: the trigger's defending player when a trigger
// carries one, else only from declare attackers to end of combat on the
// controller's own turn; otherwise the token enters without attacking. An
// attack trigger always resolves inside that window, so the gate
// (tokenCombatWindow) costs the trigger carriers nothing and is what a spell
// carrier (The Last Ronin's Technique, sneaked) needs. Adeline's
// RememberedPlayer form (no TokAttackAsk) names its defender and is not gated.
//
//go:noinline
func (s *State) tokenAttacks(k *prep, in *kop.Instr, sl Slot, p uint8) {
	if !s.occ.has(sl) {
		return
	}
	if in.B.N&kop.TokAttackAsk != 0 && !s.tokenCombatWindow(p) {
		return
	}
	atk := PlayerCode(s.nextAlive(SeatIx(p)))
	if s.g.tokDefSet {
		atk = s.g.tokDef
	} else if MaxSeats > 2 && in.B.N&kop.TokAttackAsk != 0 {
		if atk = s.enterDefenderFallback(k, p); atk == NoTarget || s.out(CodeSeat(atk)) {
			return // no defender, or the captured one left the game (gorge EnteringAttackLegal)
		}
	}
	s.setEnterAttacker(k, sl, atk)
}

// enterDefenderFallback is gorge's enteringDefenderFallback past two seats:
// a trigger's captured defending player (the seat its triggering attacker
// attacks, a planeswalker's or battle's attack naming that seat), else the
// first CR 508.4 candidate. A trigger the kernel cannot read that seat for
// (its attacker has left combat, or an event other than Attacks) stalls
// (StallDefenderPW). NoTarget: none.
func (s *State) enterDefenderFallback(k *prep, p uint8) uint8 {
	if e := &s.res.e; s.res.active && e.kind == seTrigger {
		if e.ab < abCascade && k.t.Cards[e.card].Abilities[e.ab].Trig.Event == kop.TrigAttacks && e.trigObj != NoObj {
			if ob := &s.objs[e.trigObj]; ob.zone == kop.ZoneBattlefield && s.perms[ob.slot].flags&pfAttacking != 0 {
				if q := s.atkSeat(k, ob.slot); q != NoTarget {
					return PlayerCode(q)
				}
			}
		}
		s.capFull(StallDefenderPW)
		return NoTarget
	}
	var buf [MaxOptions]uint8
	if s.enteringDefenderCands(k, p, &buf) == 0 {
		return NoTarget
	}
	return buf[0]
}

// enterDefenderOrder puts fb first among the n candidates in buf, as gorge's
// chooseEnteringDefender orders its ask (fallback first, the rest in
// enteringDefenders order); fb absent is inserted. It returns the new count.
func enterDefenderOrder(buf *[MaxOptions]uint8, n int, fb uint8) int {
	i := 0
	for i < n && buf[i] != fb {
		i++
	}
	if i == n {
		if n == MaxOptions {
			i = n - 1
		} else {
			n++
		}
	}
	copy(buf[1:i+1], buf[:i])
	buf[0] = fb
	return n
}

// tokenCombatWindow reports whether p is the active player within declare
// attackers .. end of combat, where gorge's enteringDefenderFallback names a
// defender for an entering attacker with no trigger context.
func (s *State) tokenCombatWindow(p uint8) bool {
	return p == s.active && s.step >= kop.StepAttackers && s.step <= kop.StepEndCombat
}

// tokenDefenderAsk poses TokenAttacking$ True's CR 508.4 defender pick before
// the mint (gorge effToken freezes it in resolveAttack ahead of runTokenMints,
// once per call, whatever the amount): with two or more candidates (the
// opponent, its planeswalkers, the battles it protects) it suspends the
// resolution on raGenEnterDefender with res.chosen = NoSlot, whose answer
// stores the defender and re-runs this instruction. true: the ask was posed.
//
//go:noinline
func (s *State) tokenDefenderAsk(k *prep, in *kop.Instr, p uint8) bool {
	if in.B.N&(kop.TokAttacking|kop.TokAttackAsk) != kop.TokAttacking|kop.TokAttackAsk || s.g.tokDefSet || !s.tokenCombatWindow(p) {
		return false
	}
	var buf [MaxOptions]uint8
	n := s.enteringDefenderCands(k, p, &buf)
	if MaxSeats > 2 {
		fb := s.enterDefenderFallback(k, p)
		if fb == NoTarget {
			return false
		}
		n = enterDefenderOrder(&buf, n, fb)
	}
	if n < 2 {
		return false
	}
	s.res.ask, s.res.askP, s.res.chosen = raGenEnterDefender, p, uint8(NoSlot)
	return true
}

// sacSelfMana reports whether the permanent at sl is a mana source whose
// mana ability sacrifices it (a Treasure untapped, or a sacrifice-only
// source: Tinder Wall's "Sacrifice this: Add {R}{R}").
func (s *State) sacSelfMana(k *prep, sl Slot) bool {
	pc := s.manaPCAt(k, sl)
	return pc.manaSac && s.canTapForManaPC(k, sl, pc) || s.canSacOnlyNow(k, sl, pc)
}

// abilityAffordsSacSelf is abilityAffords for an ability whose cost
// sacrifices its own source when that source is a sacrifice mana source:
// gorge prices the mana with the cost's own sacrifice held back
// (potential_plan.go planComposedCost: the sacrificed source is gone), so
// the source's mana cannot pay its own cost (Tinder Wall's "{R}, Sacrifice
// this" with no other red source is not offered).
//
//go:noinline
func (s *State) abilityAffordsSacSelf(k *prep, sl Slot, a *kop.Ability, cost *kop.ManaCost, av *avail) bool {
	av2 := *av
	pc := s.manaPCAt(k, sl)
	s.removeSrc(k, &av2, sl, pc)
	pm := pipMaskOf(cost)
	if !av2.affords(cost, pm, 0) {
		return false
	}
	if a.Cost.Flags&kop.CostTapX != 0 || a.Cost.Flags&kop.CostSac != 0 {
		return s.costManaDisjoint(k, sl, a, &av2, pm)
	}
	return true
}

// tapForSacSelf pays such an ability's mana with its source withheld from
// the planner (it is sacrificed by the cost after the mana, CR 601.2g-h).
//
//go:noinline
func (s *State) tapForSacSelf(k *prep, p uint8, cost *kop.ManaCost, x int, sl Slot) {
	held := s.ctrlM[SeatIx(p)].has(sl)
	s.ctrlM[SeatIx(p)].clr(sl)
	s.tapFor(k, p, cost, x, NoObj)
	if held {
		s.ctrlM[SeatIx(p)].set(sl)
	}
}

// --- Duration$ UntilHostLeavesPlay ---
//
// gorge anchors such an effect to its source (the host: Extraction
// Specialist's Animate, ContinuousEffect.DurationSource; Gingerbrute's
// Effect, UntilHostLeavesPlayOrEOT): it ends as the host leaves the
// battlefield, and an effect whose host has already left does nothing. A
// host-bound override record keeps the host's Obj and ver in its P/T
// fields (a WhileHost override sets no P/T: kgen refuses one); a host-bound
// emblem is State.g.embO's object.

// hostHere reports whether the resolving entry's source is still the
// battlefield object that created the ability.
func (s *State) hostHere(e *stackEntry) bool {
	if e.obj == NoObj || int(e.obj) >= MaxObj {
		return false
	}
	ob := &s.objs[e.obj]
	return ob.zone == kop.ZoneBattlefield && ob.ver == e.srcVer
}

// applyWhileHost applies the host-bound override idx to the permanent at sl.
//
//go:noinline
func (s *State) applyWhileHost(k *prep, sl Slot, idx int, e *stackEntry) {
	if !s.hostHere(e) || k.t.Overrides[idx].WhileTapped && s.perms[s.objs[e.obj].slot].flags&pfTapped == 0 {
		return // a WhileTapped host that is not tapped starts nothing (gorge registerAnimateEffects)
	}
	s.applyOverridePT(k, sl, idx, e.card, 0, int16(e.obj), int16(e.srcVer))
}

// hostLeft ends the effects whose host is object o, leaving the
// battlefield: its host-bound override records and emblems.
//
//go:noinline
func (s *State) hostLeft(k *prep, o Obj) {
	if s.fxM.any() {
		s.dropFx(func(i int) bool {
			r := &s.g.fx[i]
			return r.kind == fxOverride && k.t.Overrides[r.r.idx-1].WhileHost && Obj(r.r.p) == o
		})
		s.refreshTy(k)
	}
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		var drop uint8
		for i := 0; i < int(s.g.nemb[p]); i++ {
			if k.t.Emblems[s.embIdx(p, i)-1].HostBound() && s.embObj(p, i) == o {
				drop |= 1 << uint(i)
			}
		}
		if drop != 0 {
			s.embCompact(uint8(p), drop)
		}
	}
	s.embCaptureLapse(k, o) // static_emblemcapture.go
}

// ovCantAttack reports whether an override record on the permanent at sl
// says it can't attack (kop.Override.CantAttack).
func (s *State) ovCantAttack(k *prep, sl Slot) bool {
	for i := 0; i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.sl == sl && r.kind == fxOverride && k.t.Overrides[r.r.idx-1].CantAttack {
			return true
		}
	}
	return false
}

// hostEmblemDone reports whether a host-bound emblem (Table.Emblems index+1
// emb) for player p need not be created: its host already left (the effect
// ends at once), or p already has the same emblem from the same host. The
// second is gorge's second identical effect (Gingerbrute activated twice in
// a turn): both end together and apply the same restriction, so one record
// stands for both (no projection reads the count) and the emblem pool does
// not fill.
func (s *State) hostEmblemDone(e *stackEntry, p uint8, emb uint8) bool {
	if !s.hostHere(e) {
		return true
	}
	if s.k().t.Emblems[emb-1].AsLongAsControl && s.perms[s.objs[e.obj].slot].ctrl != p {
		return true // CR 611.2b: its player no longer controls the host
	}
	for i := 0; i < int(s.g.nemb[p]); i++ {
		if s.embIdx(p, i) == emb && s.embObj(p, i) == e.obj {
			return true
		}
	}
	return false
}

// coldF2Perm is flags2Perm's cold arm: the bits and
// F2Warped.
//
//go:noinline
func (s *State) coldF2Perm(k *prep, f kop.FilterFlags2, sl, src Slot) bool {
	if f&kop.F2Warped != 0 && !s.warpedPerm(s.perms[sl].obj) {
		return false
	}
	return f&(kop.F2BlockedBySrc|kop.F2DamagedBy|kop.F2CrewedSrc) == 0 || s.mcombatF2(k, f, sl, src)
}
