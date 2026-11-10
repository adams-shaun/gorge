package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Warp (CR 702.185a): the hand offers an alternative mana
// cost (gorge's "warped" cast option, FlagWarped). A permanent spell cast
// that way exiles itself at the beginning of the next end step; its owner
// may cast it from exile on a later turn at its normal mana cost (gorge's
// "warp_recast", rules legal_walk_alt.go exileCastsWalk).
//
// The recast permission is a DurWarp play record, not a static: the
// generic play surfaces skip it (legal_gen.go's permission loop and seen
// set, featview.go FeatMayPlayUntyped) and this file offers it alone.
//
// Cast provenance rides stackEntry.mode (modeWarped / modeWarpRecast): no
// stackEntry flag bit is free, and the oracle's stack projection ignores a
// spell's mode, so lockstep sees nothing. A copy of a warped spell clears
// the marker (copy_gen.go execCopySpell): a copy was not warped.

// AkCast mode bits for the two warp casts (face_castface.go's castFace
// is 1<<2, legal_gen.go's castMayPlay / castFree are 1<<3 / 1<<4).
const (
	castWarp       = 1 << 5 // a hand cast for its warp cost
	castWarpRecast = 1 << 6 // a warp-exiled card cast from exile
)

// stackEntry.mode bits for the warp casts (chosen modes are bits 0..3).
const (
	modeWarpRecast = 1 << 6
	modeWarped     = 1 << 7
)

// offerWarpHand offers hand card o's warp casts, one per warp instance
// (warpInstances: the printed one, then a battlefield grant's; kw_warpgrant.go),
// the instance's index on the action target (gorge's AltCostIndex). The
// caller has already checked the card's timing and the cast prohibition; a
// warp cast is offered even when the mana cost is not payable, so the caller
// runs this before its plain affordability gate.
func (s *State) offerWarpHand(k *prep, d *Decision, p uint8, o Obj, c CardID, av *avail) {
	wi := s.warpInstances(k, p, o)
	if wi.n == 0 {
		return
	}
	var ha avail
	if len(k.gen.grantPC) != 0 && s.g.availX&availWalk != 0 {
		// gorge's potential walk counts a restricted mana ability only for
		// a payment its restriction admits (potentialManaFor): a hand-barred
		// grant puts no warp cast on the walk grantCap reads
		s.walkHandAvail(k, p, payDesc{class: pdSpell, card: c, src: NoSlot, og: s.restrOriginOfObj(o)}, &ha)
		av = &ha
	}
	if !s.spellTargetsFeasible(k, c, p) || !s.spellCostsFeasible(k, c, p, o) || s.castHeldOut(o) {
		return
	}
	for i := uint8(0); i < wi.n; i++ {
		wc := s.adjustCost(k, p, c, o, wi.cost[i]) // the reduction applies to an alternative cost too (CR 601.2f)
		if i > 0 {
			// gorge's potential walk marks a second warp row of the same
			// card ambiguous with the first (rules potential_plan.go: same
			// kind, object and mode, AltCostIndex > 0) and leaves it
			// unpriced: it is a priority option only while the floating
			// pool alone pays it, the real-pool walk's offer
			pa := s.poolAvailPlus(p, nil)
			if s.rbAny(p) {
				s.restrAvail(k, p, c, s.restrOriginOfObj(o), &pa)
			}
			if !pa.affords(&wc, pipMaskOf(&wc), 0) {
				continue
			}
		}
		if !av.affords(&wc, pipMaskOf(&wc), 0) && (!s.rbAny(p) && !k.restrSrc || !s.restrFallback(k, p, c, s.restrOriginOfObj(o), &wc, av)) {
			continue
		}
		if !s.altAddCastable(k, c, p, o, &wc, av) {
			continue
		}
		d.add(MakeAction(AkCast, uint8(o), i, castWarp))
	}
}

// offerWarpRecasts offers casting p's warp-exiled cards from exile on a
// later turn (gorge rules legal_walk_alt.go exileCastsWalk, CR 702.185a):
// the normal mana cost, the card's own timing, and only while the DurWarp
// record is live (same object, still exiled).
func (s *State) offerWarpRecasts(k *prep, d *Decision, p uint8, sorc bool) {
	pl := &s.pl[p]
	for i := 0; i < int(pl.exileN); i++ {
		o := pl.exile[i]
		c := s.objs[o].card
		pc := &k.cards[c]
		if !s.warpRecastRecord(p, o) {
			continue
		}
		if !pc.instant && !sorc && !s.castFlash(k, p, c) {
			continue
		}
		if s.castHeldOut(o) || k.gen.anyCantCast && s.castForbidden(k, p, c, o) {
			continue
		}
		if !s.spellTargetsFeasible(k, c, p) || !s.spellCostsFeasible(k, c, p, o) {
			continue
		}
		// gorge's walk offer (legal_walk_alt.go exileCastsWalk offerCastable)
		// runs the real-pool walk: the recast is an option only when the
		// floating pool alone pays it, exactly like the face routes
		// (faces.go FeatPoolPays). The potential walk's version is not a
		// priority option.
		if !s.FeatPoolPays(p, o, 0) {
			continue
		}
		d.add(MakeAction(AkCast, uint8(o), 0, castWarpRecast))
	}
}

// warpRecastRecord reports whether p holds a live DurWarp record for o on
// a turn strictly after the exile's (gorge warpRecastAvailable's TurnChange
// count).
func (s *State) warpRecastRecord(p uint8, o Obj) bool {
	ob := &s.objs[o]
	for i := 0; i < int(s.g.nmay); i++ {
		r := *s.mayAt(i)
		if r.obj == o && r.ctrl == p && r.ver == ob.ver && r.until == kop.DurWarp && s.turn > r.turn {
			return true
		}
	}
	return false
}

// warpRecastLive reports whether a DurWarp record still names the exiled
// card (cleanupGen drops a stale one: the card was cast, moved or is a new
// object).
func (s *State) warpRecastLive(r mayPlay) bool {
	ob := &s.objs[r.obj]
	return ob.ver == r.ver && ob.zone == kop.ZoneExile
}

// refreshWarpRecast points the record at the card after a reversed recast
// put it back in exile: the cast never happened, so the permission stays
// (CR 733.1; gorge's abortCast leaves the exile move as it was).
func (s *State) refreshWarpRecast(o Obj) {
	ob := &s.objs[o]
	for i := 0; i < int(s.g.nmay); i++ {
		r := s.mayAt(i)
		if r.obj == o && r.ctrl == ob.owner() && r.until == kop.DurWarp {
			r.ver = ob.ver
		}
	}
}

// registerWarpExile arms the delayed exile of a permanent cast for its
// warp cost (CR 702.185a), for the beginning of the next end step.
func (s *State) registerWarpExile(o Obj, ctrl uint8) {
	if s.ndly >= MaxDelayed {
		s.capFull(StallDelayed)
		return
	}
	s.dly[s.ndly] = delayed{act: uint8(kop.OpWarpExile), ctrl: ctrl, obj: o, ver: s.objs[o].ver, creator: NoObj}
	s.ndly++
}

// warpRecastGrant gives the exiled card's owner the recast permission
// (CR 702.185a: the owner may cast it, not the spell's controller).
func (s *State) warpRecastGrant(o Obj) {
	ob := &s.objs[o]
	if s.mayFull() {
		s.capFull(StallMayPlay)
		return
	}
	*s.mayAt(int(s.g.nmay)) = mayPlay{obj: o, ver: ob.ver, ctrl: ob.owner(), until: kop.DurWarp, turn: s.turn}
	s.g.nmay++
}

// warpedPerm: battlefield object o was cast for its warp cost (gorge
// CastSa Spell.Warp: the permanent's cast ability was the warp one). Its
// mark is the warp exile it armed as it resolved (registerWarpExile): the
// delayed record, the trigger it queued at the end step, or that trigger
// on the stack, each naming this incarnation; once the exile resolves the
// card is no longer this permanent (a flickered one is a new object).
func (s *State) warpedPerm(o Obj) bool {
	v := s.objs[o].ver
	for i := 0; i < int(s.ndly); i++ {
		if d := &s.dly[i]; d.obj == o && d.ver == v && kop.Op(d.act) == kop.OpWarpExile {
			return true
		}
	}
	for i := 0; i < int(s.npend); i++ {
		if pt := &s.pend[i]; pt.ab == abDelayed && pt.obj == o && pt.srcVer == v && kop.Op(pt.trigAmt) == kop.OpWarpExile {
			return true
		}
	}
	for i := 0; i < int(s.sp); i++ {
		if e := &s.stack[i]; e.kind == seTrigger && e.ab == abDelayed && e.obj == o && e.srcVer == v && kop.Op(e.trigAmt) == kop.OpWarpExile {
			return true
		}
	}
	return false
}
