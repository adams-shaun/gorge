package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// gorge's cast ledger (rules spellsCastThisTurnMatching),
// bounded, in a cold array.
//
// gorge counts the PutOnStack events since the last TurnChange (CR 601.2a:
// the push, not the payment) and tests each pushed object's spec against
// the object as it is NOW, wherever it lives: its current controller and
// characteristics (a spell cast from an opponent's card stops counting for
// YouCtrl once it leaves the stack, a stolen permanent spell stops counting
// for its caster). The kernel's per-caster tallies (histSpells, isCast)
// cannot answer that, so each player's pushes are listed here:
// castPushes(p) holds them in push order, and hist[p][histCastN] counts them.
//
// The ring is recorded only for tables with an ExThisTurnCastSpec
// expression, a Tf4NthFiltered trigger or a Paradigm card. A push past
// castRingN by one player in one turn stalls with StallCastRing rather than
// truncating.
//
// CR 733.1: gorge's abortCast leaves the reversed cast's PutOnStack in the
// append-only log, but its readers pair the SpellCastReversed with the cast it
// undid (events.CastReversals) and count no cast. unpushSpell marks the ring
// entry reversed (castReversedPush) and every count skips it; the entry keeps
// its slot, for stackedThisTurn reads the log's PutOnStack, which stays.

// castRingN is the pushes per player per turn the ring records.
const castRingN = 12

// castPush is one push: the object as Obj+1 and its castFrom* origin and
// castTreasurePush and castWarpPush bits.
type castPush struct {
	obj  uint8
	prov uint8
}

// castPush.prov: bits 0-1 the zone the push came from.
const (
	castFromOther uint8 = iota
	castFromHand
	castFromGrave
	castFromExile
	castFromMask uint8 = 3

	castTreasurePush uint8 = 4 // its payment spent mana from a Treasure (CastSpecTreasure)

	// castAdventurePush (16; castWarpPush is 8): the push was an Adventure
	// card's Adventure half, which CastSpecAsCast reads as that half although
	// the card now shows its creature face from exile.
	castAdventurePush uint8 = 16

	// castReversedPush: the cast was reversed (CR 733.1), and no count reads it.
	castReversedPush uint8 = 32
)

// castPushes is player p's pushes this turn. A table with no reader has no ring.
func (s *State) castPushes(p uint8) []castPush {
	n := s.g.hist[p][histCastN]
	if n == 0 {
		return nil
	}
	return s.cold.castRing[int(p)*castRingN:][:n]
}

// noteReversed marks caster's latest push of o reversed (CR 733.1), the cast
// gorge's events.CastReversals pairs with the SpellCastReversed.
func (s *State) noteReversed(o Obj, caster uint8) {
	ring := s.castPushes(caster)
	for i := len(ring) - 1; i >= 0; i-- {
		if ring[i].obj == uint8(o)+1 && ring[i].prov&castReversedPush == 0 {
			ring[i].prov |= castReversedPush
			return
		}
	}
}

// noteCast appends the push of spell o by caster (pushBuild, CR 601.2a).
// from is the push's origin zone (gorge PutOnStack.From): the from-hand
// bit (wasCastFromYourHand[ByYou]) and the push's origin code
// (wasCastFromExile).
//
//go:noinline
func (s *State) noteCast(o Obj, caster uint8, from kop.Zone) {
	n := s.g.hist[caster][histCastN]
	if n >= castRingN {
		s.capFull(StallCastRing)
		return
	}
	s.bld().castN = n + 1
	code := castFromOther
	switch from {
	case kop.ZoneHand:
		code = castFromHand
	case kop.ZoneGraveyard:
		code = castFromGrave
	case kop.ZoneExile:
		code = castFromExile
	}
	if s.bld().e.mode&modeWarped != 0 {
		code |= castWarpPush // value_void.go: the push is a warp cast
	}
	s.cold.castRing[int(caster)*castRingN+int(n)] = castPush{obj: uint8(o) + 1, prov: code}
	s.g.hist[caster][histCastN] = n + 1
}

// Recycling (zones.go objReferenced): only an ofToken object outside every
// zone is recycled, and the one object pushed through pushBuild that is not
// a card is a prepared copy (face_prepare.go), an ofCopy object that
// newToken never recycles (a token cannot be cast; a spell copy is a stack
// entry copy_gen.go pushes without a cast), so no ring entry can name a
// recycled object and the ring needs no castRingReferenced guard beside
// enteredReferenced.

// thisTurnCastSpec evaluates Expr ExThisTurnCastSpec (gorge
// spellsCastThisTurnMatching):
//
//   - CastSpecYou (the spec contains "You", gorge's substring test): only
//     the evaluating controller's pushes; otherwise both players';
//   - CastSpecExclude (!CastSaSource): every push of the source object is
//     dropped;
//   - CastSpecHand (wasCastFromYourHand[ByYou]): only pushes from the
//     caster's hand (gorge castProvenanceAdmits on THIS cast);
//   - CastSpecNotHand (!wasCastFromYourHand[ByYou]): only the pushes from
//     anywhere else, the complement of the same from-hand bit;
//   - CastSpecExile (wasCastFromExile): only pushes from exile (gorge
//     castOriginAdmits on THIS cast's ledger view, any caster);
//   - CastSpecGrave (wasCastFromGraveyard): only pushes from a graveyard
//     (a resolved flashback spell is a new object but its cast still counts,
//     CR 400.7);
//   - while buildCost composes the in-flight spell's reductions after its
//     push (bflCostComp), that push is hidden (gorge costCompositionEvent:
//     Gigastorm Titan's "another spell" sees prior casts only);
//   - F is matched against the object where it lives now.
//
//go:noinline
func (s *State) thisTurnCastSpec(k *prep, x *kop.Expr, ctx evalCtx) int {
	if DebugCastRingLive {
		return s.thisTurnCastSpecTally(x, ctx)
	}
	hide := NoObj
	hideP := uint8(2)
	if s.bld().bfl&(bflCostComp|bflPushed) == bflCostComp|bflPushed && s.bld().e.kind == seSpell && !DebugReduceGatesNoFirst {
		hide, hideP = s.bld().e.obj, s.bld().e.ctrl
	}
	n := 0
	var tm kop.TypeMask // CastSpecTypes: the matching objects' card-type union
	for p := uint8(0); p < MaxSeats; p++ {
		if x.N&kop.CastSpecYou != 0 && p != ctx.ctrl {
			continue
		}
		ring := s.castPushes(p)
		if p == hideP && len(ring) > 0 && Obj(ring[len(ring)-1].obj-1) == hide {
			ring = ring[:len(ring)-1] // the in-flight cast's own (latest) push
		}
		for _, r := range ring {
			if r.prov&castReversedPush != 0 {
				continue
			}
			o := Obj(r.obj - 1)
			if x.N&kop.CastSpecExclude != 0 && o == ctx.src {
				continue
			}
			if x.N&kop.CastSpecHand != 0 && r.prov&castFromMask != castFromHand {
				continue
			}
			if x.N&kop.CastSpecNotHand != 0 && r.prov&castFromMask == castFromHand {
				continue
			}
			if x.N&kop.CastSpecTreasure != 0 && r.prov&castTreasurePush == 0 {
				continue
			}
			if x.N&kop.CastSpecExile != 0 && r.prov&castFromMask != castFromExile {
				continue
			}
			if x.N&kop.CastSpecGrave != 0 && r.prov&castFromMask != castFromGrave {
				continue
			}
			if x.N&(kop.CastSpecCtrlTrig|kop.CastSpecTypes) != 0 {
				if s.castRingCountHeadsMatch(k, x, o, ctx, &tm) && x.N&kop.CastSpecTypes == 0 {
					n++
				}
				continue
			}
			if x.N&kop.CastSpecAsCast != 0 && r.prov&castAdventurePush != 0 {
				if s.castRingMatchAdventure(k, x.F, o, ctx) {
					n++
				}
				continue
			}
			if s.castRingMatch(k, x.F, o, ctx) {
				n++
			}
		}
	}
	if x.N&kop.CastSpecTypes != 0 {
		return castTypeCount(tm)
	}
	return n
}

// castPushesMatching counts caster's pushes this turn that match f, read from
// listener l's side (gorge activatorCastCount, ActivatorThisTurnCast$ over a
// ValidCard$ that turns some spells away).
func (s *State) castPushesMatching(k *prep, f kop.FilterID, caster uint8, l trigListener) int {
	n := 0
	for _, r := range s.castPushes(caster) {
		if r.prov&castReversedPush == 0 && s.castRingMatch(k, f, Obj(r.obj-1), evalCtx{src: l.obj, srcSl: l.sl, ctrl: l.ctrl}) {
			n++
		}
	}
	return n
}

// castRingMatch tests f against object o where it lives now (gorge
// matchesSpec on the pushed ObjID): a permanent through the perm matcher
// (current controller, layered characteristics); a spell on the stack as
// its stack entry's card under the entry's controller; anywhere else the
// card under its owner (gorge resets Controller to Owner off the
// battlefield and stack, apply_zone.go). kgen lowers only clauses both
// matchers read exactly (no Self/Other, no battlefield-only flags).
func (s *State) castRingMatch(k *prep, f kop.FilterID, o Obj, ctx evalCtx) bool {
	ob := &s.objs[o]
	switch ob.zone {
	case kop.ZoneBattlefield:
		if ob.slot == NoSlot {
			return false
		}
		return s.matchPermGen(k, f, ob.slot, ctx.ctrl, ctx.srcSl, layerFull)
	case kop.ZoneStack:
		for i := int(s.sp) - 1; i >= 0; i-- {
			if st := &s.stack[i]; st.kind == seSpell && st.obj == o {
				return s.matchCardEntered(k, f, st.card, st.ctrl, ctx.ctrl, o, ctx.src)
			}
		}
		return false
	}
	if s.ceasedCopy(ob) {
		return false
	}
	return s.matchCardEntered(k, f, ob.card, ob.owner(), ctx.ctrl, o, ctx.src)
}

// castRingMatchAdventure is castRingMatch for a push of an Adventure half:
// the card's Adventure face, the characteristics it was cast with (gorge
// MatchesSpecAsFace; Forge's thisTurnCast holds the cast-time LKI copy).
func (s *State) castRingMatchAdventure(k *prep, f kop.FilterID, o Obj, ctx evalCtx) bool {
	ob := &s.objs[o]
	c := ob.card
	if fs := k.t.Cards[c].Faces; len(fs) == 2 {
		c = fs[1]
	}
	return s.matchCardEntered(k, f, c, ob.owner(), ctx.ctrl, o, ctx.src)
}

// DebugCastRingLive is the mutation test's switch: thisTurnCastSpec reads
// the caster's histSpells tally (the payment-time count by caster) instead
// of the ring, the reader kgen refused before. Test-only
// and cold.
var DebugCastRingLive bool

// thisTurnCastSpecTally is the refused tally reader, kept only for the
// mutation test: it ignores the spec's filter and the object's current
// controller, and counts at payment rather than at the push.
func (s *State) thisTurnCastSpecTally(x *kop.Expr, ctx evalCtx) int {
	n := int(s.g.hist[ctx.ctrl][histSpells])
	if x.N&kop.CastSpecYou == 0 && ctx.ctrl < 2 {
		n += int(s.g.hist[1-ctx.ctrl][histSpells])
	}
	return n
}

// thisTurnCast is ExThisTurnCast (gorge spellsCastThisTurnMatching for the
// <Type>.YouCtrl / Card.OppCtrl specs): the spells player Ref cast this turn
// (PutOnStack since TurnChange), every spell (N=0, the bank's histSpells)
// or instants and sorceries (N=1, the isCast tally onCast keeps).
//
//go:noinline
func (s *State) thisTurnCast(k *prep, x *kop.Expr, ctx evalCtx) int {
	p := s.refPlayer(k, x.Ref, ctx)
	if p > 1 {
		return 0
	}
	if x.N == 1 {
		return int(s.g.isCast[p])
	}
	return int(s.g.hist[p][histSpells])
}
