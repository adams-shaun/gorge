package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Sneak (CR 702.190), cold: reached only when the table has a
// card with printed sneak (genPrep.anySneak).
//
// In its controller's own declare-blockers step, at instant speed, a hand
// card with sneak may be cast for its sneak cost plus returning an
// unblocked attacker they control to its owner's hand (gorge rules
// kw_sneak.go sneakCosts / sneakTimingOK, legal_walk_hand.go's "sneak" cast
// option). The cast is AkSneak. Its provenance and the captured defender
// ride two stack-entry fields no spell uses (0 B): trigPl = trigPlSneak
// marks the cast and trigObj holds the returned creature's perm.atk (a
// PlayerTarget* or a planeswalker Obj), captured when the return pick is
// made, while the creature is still attacking (gorge cast_payment.go's
// sneak-defender fold). A copy keeps neither (CR 707.10: gorge's
// CastProvenanceFlags; mintSpellCopy).
//
// The return is the activation Return<N/Spec> machinery (act_energy.go
// pkReturn / ChCostReturn / payReturnPicks): one pick, asked even with one
// candidate (gorge cast_asks.go returnAsk skips only a CARDNAME
// singleton), paid at payCast after the mana and before the sacrifices,
// so a reversed cast moves nothing (CR 733.1).
//
// A permanent spell cast this way enters tapped (its entry state: no Taps
// or TapAll trigger fires, CR 603.2e; gorge emitTap's entering flag) and
// then attacking the captured defender when that
// is still legal (CR 702.190b; gorge kw_sneak.go sneakEnter,
// effects.EnteringAttackLegal). It was never declared as an attacker: no
// Attacks trigger, no attacked count (CR 508.4); and no blocker blocks it.
//
// Gorge's planner prices the sneak route too
// (potentialModeCastPlan's sneak arm), so under the unpriced surface the
// option exists when the mana census pays the adjusted sneak cost (the
// floating pool plus untapped sources, offerSneakHand); a cast the pool
// alone cannot pay is lowered by tapping the witness at priority
// (beginSneak -> lowerSneakCast, cast_faceroute.go).

// trigPlSneak (trigpl.go) marks a spell entry cast for its sneak cost.

// DebugSneakNoAttack makes a sneaked permanent enter tapped but not
// attacking (mutation testing only: the lockstep must catch it).
var DebugSneakNoAttack bool

// DebugSneakAnyStep also offers the sneak cast in the combat-damage steps
// (mutation testing only).
var DebugSneakAnyStep bool

// DebugSneakBlockedReturn lets a blocked attacker pay the return
// (mutation testing only).
var DebugSneakBlockedReturn bool

// sneakStep reports whether the current step is the sneak window (the
// active player's declare-blockers step; p == s.active is the caller's).
func (s *State) sneakStep() bool {
	return s.step == kop.StepBlockers || DebugSneakAnyStep && (s.step == kop.StepFirstStrikeDamage || s.step == kop.StepDamage)
}

// sneakEntry reports whether e is a spell cast for its sneak cost.
func sneakEntry(e *stackEntry) bool {
	return e.kind == seSpell && e.trigPl == trigPlSneak
}

// sneakProv reports whether e's provenance is a sneak cast: e itself when it
// is the spell, else the source spell entry still on the stack (a self cast
// trigger resolves above its spell, CR 603.2; gorge reads the spell's
// CastFlags there too). An entry with no source spell on the stack (a
// delayed trigger, a permanent's ability) has no sneak provenance.
func (s *State) sneakProv(e *stackEntry) bool {
	if e == nil {
		return false
	}
	if e.kind == seSpell {
		return sneakEntry(e)
	}
	for i := 0; i < int(s.sp); i++ {
		if se := &s.stack[i]; se.kind == seSpell && se.obj == e.obj {
			return sneakEntry(se)
		}
	}
	return false
}

// sneakReturnable reports whether slot sl (controlled by the payer) can be
// returned for a sneak cost: a creature, attacking and unblocked (gorge
// "Creature.YouCtrl+attacking+unblocked"; a blocker that left keeps the
// attacker blocked, filter.go's BlockedBy tombstone = pfBlocked).
func (s *State) sneakReturnable(k *prep, sl Slot) bool {
	pp := &s.perms[sl]
	return pp.flags&pfAttacking != 0 && (pp.flags&pfBlocked == 0 || DebugSneakBlockedReturn) && s.typesGen(k, sl)&kop.TCreature != 0
}

// sneakHasReturn reports whether p controls a creature the return can
// take.
func (s *State) sneakHasReturn(k *prep, p uint8) bool {
	for m := s.ctrlM[SeatIx(p)]; m.any(); m = m.rest() {
		if s.sneakReturnable(k, m.first()) {
			return true
		}
	}
	return false
}

// sneakPoolPays reports whether p's floating pool alone pays cost for
// card c, object o (FeatPoolPays's pool, the restriction rider included;
// the cast's own delve credit prices the twobrid faces like the offer).
func (s *State) sneakPoolPays(k *prep, p uint8, c CardID, o Obj, cost *kop.ManaCost) bool {
	pl := &s.pl[SeatIx(p)]
	var a avail
	for i := 0; i < kop.NumManaKinds; i++ {
		if pl.pool[i] > 0 {
			a.addUnits(1<<i, pl.pool[i])
		}
	}
	if s.rbAny(p) {
		s.restrAvail(k, SeatIx(p), c, restrOrigin{from: kop.ZoneHand}, &a)
	}
	a.delve = s.tbDelveCredit(k, SeatIx(p), c, o, s.objs[o].zone == kop.ZoneHand) // the faces' credit (CR 702.66a)
	return a.affords(cost, pipMaskOf(cost), 0)
}

// offerSneakHand offers hand card o's sneak cast (gorge legal_walk_hand.go,
// above the sorcery-speed timing gate). The caller checked the window
// (p active, sneakStep) and that the card has sneak.
func (s *State) offerSneakHand(k *prep, d *Decision, p uint8, o Obj, c CardID, av *avail) {
	if s.castHeldOut(o) || k.gen.anyCantCast && s.castForbidden(k, p, c, o) {
		return // gorge castSuppressed / castRestricted
	}
	if !s.spellTargetsFeasible(k, c, p) || !s.sneakHasReturn(k, p) {
		return // castTargetsAvailable; offerCastable's Return census
	}
	cost := s.adjustCost(k, p, c, o, k.t.Cards[c].SneakCost) // spellScope("sneak")
	if len(k.gen.grantPC) != 0 && s.g.availX&availWalk != 0 {
		// gorge's potential walk counts a restricted mana ability only for a
		// payment its restriction admits (potentialRestrictionAdmits): a
		// hand-barred grant puts no cast on the walk grantCap reads
		var ha avail
		s.walkHandAvail(k, p, payDesc{class: pdSpell, card: c, src: NoSlot, og: s.restrOriginOfObj(o)}, &ha)
		av = &ha
	}
	// the avail is the caller's walk picture, or the hand-barred grant's
	// local one above: the cast's own delve credit prices its twobrid faces
	// and is restored after (offerPlay's shape). Set after the swap, so the
	// avail actually used for pricing carries it.
	saved := av.delve
	defer func() { av.delve = saved }()
	av.delve = s.tbDelveCredit(k, p, c, o, true) // the faces' credit (CR 702.66a)
	if !av.tbLeafCredited(&cost) {
		cost = s.delveOfferCredit(k, p, c, o, true, cost) // gorge's choke point credits the generic at this walk too
	}
	if !av.affords(&cost, pipMaskOf(&cost), 0) && (!s.rbAny(p) && !k.restrSrc || !s.restrFallback(k, p, c, s.restrOriginOfObj(o), &cost, av)) {
		// The restrAffords fallback matches the plot/warp hand offers (a
		// restricted mana source can pay); it is dead in the sneak pool,
		// which has no State.rb source. Keeping it makes the priced offer the
		// same shape as those two rather than faceCastable's narrower one.
		return
	}
	// gorge prices a sneak cast through the potential planner as well as
	// the floating pool (potential_plan.go potentialModeCastPlan), so the
	// offer is the mana census, not the pool alone.
	if DebugFacePoolOnly && !s.sneakPoolPays(k, p, c, o, &cost) {
		return
	}
	d.add(MakeAction(AkSneak, TCode(o), 0, 0))
}

// beginSneak casts hand card o for its sneak cost (gorge cast_begin.go
// castModeSneak): an ordinary cast, never lowered (castFace keeps it out
// of beginCast's lowering), marked so buildCost prices the sneak cost and
// the cost stage asks the return.
func (s *State) beginSneak(k *prep, o Obj) {
	s.beginCast(k, o, castFace, 0)
	s.bld().e.trigPl = trigPlSneak
	s.bld().e.trigObj = NoObj
	s.lowerSneakCast(k, s.prio, o, s.bld().e.card) // cast_faceroute.go
}

// sneakReturnOptions poses the sneak (or web-slinging) cast's return pick (gorge returnAsk,
// Min = Max = 1, asked even with one candidate); false once it is made.
// With no candidate left the cast is reversed (CR 733.1, gorge's abort:
// sacShort's precedent).
//
//go:noinline
func (s *State) sneakReturnOptions(k *prep, d *Decision) bool {
	e := &s.bld().e
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().pickKs[i] == pkReturn {
			return false
		}
	}
	d.N = 0
	for m := s.ctrlM[SeatIx(e.ctrl)]; m.any(); m = m.rest() {
		sl := m.first()
		if s.altReturnable(k, sl, e) {
			d.add(MakeAction(AkChoose, TCode(s.perms[sl].obj), 0, 0))
		}
	}
	if d.N == 0 {
		s.sacShort(k, d)
		return true
	}
	s.bld().pickK, s.bld().pickLeft = pkReturn, 1
	d.Kind, d.Sub, d.Player = DkChoose, ChCostReturn, e.ctrl
	return true
}

// sneakCapture records the returned creature's defender on the sneak
// entry as the return pick of object o is made (gorge cast_payment.go:
// read while it is still on the battlefield, attacking).
func (s *State) sneakCapture(o Obj) {
	if ob := &s.objs[o]; ob.zone == kop.ZoneBattlefield {
		s.bld().e.trigObj = Obj(s.perms[ob.slot].atk)
		s.syncBuild()
	}
}

// sneakEnter is CR 702.190b's entry rider on permanent sl, just entered
// from sneak entry e: tapped first (the entry state, not a tapping: gorge
// emitTap(.., entering) fires no Taps or TapAll trigger), then attacking the
// captured defender if that is still legal. Nothing is declared.
func (s *State) sneakEnter(k *prep, sl Slot, e *stackEntry) {
	pp := &s.perms[sl]
	if pp.flags&pfTapped == 0 {
		pp.flags |= pfTapped
		s.touchSl(sl)
	}
	if DebugSneakNoAttack || !s.enteringAttackLegal(k, e.ctrl, e.trigObj) {
		return
	}
	pp.flags |= pfAttacking
	pp.atk = uint8(e.trigObj)
	pp.bord = s.nAttackers
	s.nAttackers++
	s.touchSl(sl)
}

// enteringAttackLegal is gorge effects.EnteringAttackLegal for a
// permanent controlled by ctrl entering attacking defender d: a player
// (two players: the opponent), or a planeswalker still on the
// battlefield, still a planeswalker and controlled by the opponent. A
// battle defender cannot occur: no battle is attackable in the kernel
// (adds that arm).
//
// gorge's Lost check is ob.zone != ZoneBattlefield, which is tested here;
// its walker-version check (the same planeswalker object, not a re-entry
// that reused the Obj) is unreachable in this pool: no accepted sneak pool
// card can remove and return a planeswalker during the return pick, and
// the kernel does not reuse a slot for a different object without the
// zone check catching it. A pool that gains such a card must add that
// check (or refuse the card) before it can be lowered.
func (s *State) enteringAttackLegal(k *prep, ctrl uint8, d Obj) bool {
	t := uint8(d)
	if isPlayerTarget(TCode(t)) {
		return CodeSeat(TCode(t)) != SeatIx(ctrl)
	}
	if d == NoObj {
		return false
	}
	ob := &s.objs[d]
	return ob.zone == kop.ZoneBattlefield && s.typesGen(k, ob.slot)&kop.TPlaneswalker != 0 && s.oppsM(ctrl).has(ob.slot)
}

// the per-permanent `sneaked` mark (CR 702.190a), one slot
// mask on the mechanic-bit arena (mechSneaked, 0 B). gorge's filter reads
// CastFlags&FlagSneaked, which the pay-time CastInfo stamps and which survives
// the stack-to-battlefield move (a stack copy has none, CR 707.10; a
// re-entered card is a new object, CR 400.7; a token never has it). enterGen
// sets the bit for the resolving spell's own permanent; arenaClrSlot clears it
// for every other new permanent. flags2Perm reads it.

// DebugSneakMarkAll marks every spell's permanent sneaked (mutation testing
// only: the lockstep must catch it).
var DebugSneakMarkAll bool

// sneakMarks reports whether the resolving spell e leaves a sneaked mark.
func sneakMarks(e *stackEntry) bool {
	return sneakEntry(e) || DebugSneakMarkAll
}

// sneakFilterRead reports whether some table filter reads F2Sneaked, the
// only reader of the mark; a pool with printed sneak but no such filter
// keeps no mask.
func sneakFilterRead(t *kop.Table) bool {
	for i := range t.Filters {
		if t.Filters[i].Flags2&kop.F2Sneaked != 0 {
			return true
		}
	}
	return false
}
