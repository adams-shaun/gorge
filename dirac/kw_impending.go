package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Impending (CR 702.176a): "If you cast this spell for its
// impending cost, it enters with N time counters and isn't a creature
// until the last is removed. At the beginning of your end step, remove a
// time counter from it." (gorge rules/impending.go, the altImpending row
// of altcast_modes.go.)
//
//   - The hand offers the "impended" cast (offerImpendingHand) exactly as
//     it offers a warp cast: the alternative cost against the potential
//     sources, before the plain affordability gate. It rides AkCast mode 0
//     with castImpTgt in the target byte (the mode byte is full); legal.go
//     sets genState.impCast for beginCast.
//   - Cast provenance rides stackEntry.mode (modeImpended, bit 2: a
//     permanent spell's chosen-mode bits are unused, a face-down one's
//     family sits in bits 0-1). buildCost prices CardDef.ImpendingCost; a
//     copy of the spell clears the bit (CR 707.10, copy_gen.go).
//   - The resolving permanent enters with N time counters (genState.
//     enterCtr, placed through the counter replacements like gorge's
//     staged entry-counter plan).
//   - The type switch (impendTy): an Impending card's permanent with a
//     time counter is not a creature. Gorge reads FlagImpending AND the
//     counter (state.Object.ImpendingDormant); the kernel reads the counter
//     alone, which kgen makes equivalent by refusing every other card that
//     puts time counters (dirac/kgen buildCard). Proliferate adds one only to
//     a permanent that already has one: an impended one.
//   - The end-step removal is the card's own synthesized trigger (kgen
//     lower.go impendingTick: gorge's granted trigger line, gated on a time
//     counter remaining).
//   - Types are cached (refreshTy): the dormant slots join its mask
//     (impendSlots), and the state-based pass re-derives them once the last
//     counter left (impendRefresh).

// castImpTgt in an AkCast's target byte (mode 0, from the hand) is the
// cast for the card's impending cost (offerImpendingHand).
const castImpTgt = 2

// modeImpended in stackEntry.mode: a permanent spell cast for its
// impending cost.
const modeImpended = 1 << 2

// impended reports whether e is a permanent spell cast impended.
func (e *stackEntry) impended() bool {
	return e.kind == seSpell && e.ab == 255 && e.mode&(modeImpended|seFaceDown) == modeImpended
}

// offerImpendingHand offers hand card o's impending cast (gorge's
// "impended" option, potential like "warped"). The caller has checked the
// card's timing and the cast prohibition; it runs before the plain
// affordability gate.
//
//go:noinline
func (s *State) offerImpendingHand(k *prep, d *Decision, p uint8, o Obj, c CardID, av *avail) {
	cd := &k.t.Cards[c]
	ic := s.adjustCost(k, p, c, o, cd.ImpendingCost) // the reduction applies to an alternative cost too (CR 601.2f)
	if len(k.gen.grantPC) != 0 && s.g.availX&availWalk != 0 {
		// gorge's potential walk counts a restricted mana ability only for a
		// payment its restriction admits (potentialRestrictionAdmits): a
		// hand-barred grant puts no cast on the walk grantCap reads
		var ha avail
		s.walkHandAvail(k, p, payDesc{class: pdSpell, card: c, src: NoSlot, og: s.restrOriginOfObj(o)}, &ha)
		av = &ha
	}
	if !av.affords(&ic, pipMaskOf(&ic), 0) && (!s.rbAny(p) && !k.restrSrc || !s.restrFallback(k, p, c, s.restrOriginOfObj(o), &ic, av)) {
		return
	}
	if !s.spellTargetsFeasible(k, c, p) || !s.spellCostsFeasible(k, c, p, o) || !s.altAddCastable(k, c, p, o, &ic, av) {
		return
	}
	if s.castHeldOut(o) {
		return
	}
	d.add(MakeAction(AkCast, uint8(o), castImpTgt, 0))
}

// impendEnter readies the resolving impended spell's time counters for its
// entry (s.enter places genState.enterCtr through the replacements).
func (s *State) impendEnter(k *prep, e *stackEntry) {
	if n := k.t.Cards[e.card].Impending; n > 0 && e.impended() {
		s.g.enterCtr.n, s.g.enterCtr.kind = n, kop.CtrTime
	}
}

// impendDormant reports whether the permanent at sl is impending-dormant:
// an Impending card with a time counter.
func (s *State) impendDormant(k *prep, sl Slot) bool {
	return k.t.Cards[s.cardAt(sl)].Impending > 0 && s.perms[sl].ctrN(s, int(kop.CtrTime)) > 0
}

// impendTy is CR 702.176a's type switch over the derived types t of sl.
func (s *State) impendTy(k *prep, sl Slot, t kop.TypeMask) kop.TypeMask {
	if s.impendDormant(k, sl) && !DebugNoImpendTy {
		return t &^ kop.TCreature
	}
	return t
}

// DebugNoImpendTy (TestImpendLockstepTypeMutation): the impending type
// switch is ignored, an impending-dormant permanent stays a creature.
var DebugNoImpendTy bool

// impendSlots is the battlefield's impending-dormant permanents (refreshTy
// keeps their ovGenTy types).
//
//go:noinline
func (s *State) impendSlots(k *prep) (m slotMask) {
	for o := s.occ; o.any(); o = o.rest() {
		if sl := o.first(); s.impendDormant(k, sl) {
			m = m.with(sl)
		}
	}
	return m
}

// impendRefresh re-derives the types once a dormant permanent lost its last
// time counter (the end-step removal): gorge derives the switch from live
// state, so the state-based pass sees a creature again.
//
//go:noinline
func (s *State) impendRefresh(k *prep) bool {
	for m := s.g.tyM; m.any(); m = m.rest() {
		sl := m.first()
		if s.perms[sl].ovr.flags&ovGenTy != 0 && k.t.Cards[s.cardAt(sl)].Impending > 0 && !s.impendDormant(k, sl) &&
			s.perms[sl].gty&kop.TCreature == 0 && k.cards[s.cardAt(sl)].types&kop.TCreature != 0 {
			s.refreshTy(k)
			return true
		}
	}
	return false
}

// The shared gates (genPrep.tyPost, prep.altCost): one flag test on the
// hot paths for bestow and impending together.

// tyPostTy applies the bestow and impending type switches to sl's types t.
//
//go:noinline
func (s *State) tyPostTy(k *prep, sl Slot, t kop.TypeMask) kop.TypeMask {
	if k.gen.anyBestow {
		t = s.bestowTy(k, sl, t) // CR 702.103b (kw_bestow.go)
	}
	if k.gen.anyImpending {
		t = s.impendTy(k, sl, t) // CR 702.176a
	}
	return t
}

// tyPostSlots is the slots whose types the switches change (refreshTy).
//
//go:noinline
func (s *State) tyPostSlots(k *prep) (m slotMask) {
	if k.gen.anyBestow {
		m = s.bestowSlots(k)
	}
	if k.gen.anyImpending {
		m = m.or(s.impendSlots(k))
	}
	return m
}

// tyPostRefresh re-derives the types once a switch ended since they were
// last derived (the state-based pass): a bestow card detached, the last
// time counter left. It reports whether one did.
//
//go:noinline
func (s *State) tyPostRefresh(k *prep) bool {
	return k.gen.anyBestow && s.bestowRefresh(k) || k.gen.anyImpending && s.impendRefresh(k)
}

// offerAltHand offers hand card o's own alternative casts (prep.altCost):
// the impended cast before the plain affordability gate, like warp (CR
// 702.176a), and the card's own alternative cost (offerAltFreeHand).
//
//go:noinline
func (s *State) offerAltHand(k *prep, d *Decision, p uint8, o Obj, c CardID, av *avail) {
	if k.t.Cards[c].Impending > 0 {
		s.offerImpendingHand(k, d, p, o, c, av)
	}
	if k.t.Cards[c].HasEvoke {
		s.offerEvokeHand(k, d, p, o, c, av) // kw_evoke.go
	}
	if k.t.Cards[c].HasDash {
		s.offerDashHand(k, d, p, o, c, av) // kw_dash.go
	}
	if k.t.Cards[c].HasOverload {
		s.offerOverloadHand(k, d, p, o, c, av) // kw_overload.go
	}
	if k.t.Cards[c].HasSurge {
		s.offerSurgeHand(k, d, p, o, c, av) // kw_surge.go
	}
	if k.gen.altFree != nil && k.gen.altFree[c] != 0 {
		s.offerAltFreeHand(k, d, p, o, c, av) // cast_altfree.go
	}
}
