package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// sacrificedMV is Sacrificed$CardManaCost (gorge c.Sacrificed's last known
// information): the mana value the sacrificed cost object had, the copied card's
// on a copy (CR 707.2), from the entry's LKI record (RefSacrificed); 0 without one,
// and 0 for a face-down permanent (CR 708.2; gorge captures ManaValue before the
// zone change, where FaceDownHidden is 0).
//
//go:noinline
func (s *State) sacrificedMV(ctx evalCtx) int {
	if e := ctx.e; e != nil && e.li != 0 {
		l := s.lk(e.li)
		if l.bits&lkiFaceDown != 0 {
			return 0
		}
		return int(l.mv)
	}
	return 0
}

// triggerLKIMV is TriggeredCard$CardManaCost on a trigger whose entry holds the
// triggering permanent's last-known information: the mana value it had on the
// battlefield, the copied card's on a copy (CR 707.2, 608.2h, 603.10a; gorge
// reads the zone-change snapshot, count_ref_property.go), not the printed card
// it became in the graveyard (a Mockingbird that copied a three-drop reads 3,
// not its own X cost). ok is false when there is no record of o.
func (s *State) triggerLKIMV(r kop.Ref, o Obj, ctx evalCtx) (int, bool) {
	if r != kop.RefTriggerObj || ctx.e == nil || ctx.e.li == 0 {
		return 0, false
	}
	l := s.lk(ctx.e.li)
	if l.obj != o || l.ver != s.objs[o].ver && s.objs[o].zone == kop.ZoneBattlefield {
		return 0, false
	}
	if l.bits&lkiFaceDown != 0 {
		return 0, true
	}
	return int(l.mv), true
}

// targetMV is Targeted$CardManaCost (gorge evalRefProperty over the
// targeted object, whose id survives its moves): the mana value of the
// chosen target's card -- the copied card while it is on the battlefield
// (its derived characteristics), the printed card elsewhere (a copy effect
// ends as it leaves); 0 for an illegal target. objMV adds the announced X
// when the target is a spell on the stack (CR 202.3e).
//
//go:noinline
func (s *State) targetMV(k *prep, r kop.Ref, ctx evalCtx) int {
	return s.objTargetMV(k, s.refObjCtx(r, ctx))
}

// objTargetMV is targetMV's read over the target object o.
func (s *State) objTargetMV(k *prep, o Obj) int {
	if o == NoObj {
		return 0
	}
	ob := &s.objs[o]
	if s.faceDownAt(k, o) {
		return 0 // CR 708.2: a face-down permanent or spell has no mana cost
	}
	if ob.zone == kop.ZoneBattlefield {
		return s.objMV(k, s.cardAt(ob.slot), o) // static_cda.go
	}
	return s.objMV(k, ob.card, o) // static_cda.go
}

// rememberedMV is Remembered$CardManaCost (gorge evalRefProperty): the
// remembered objects' mana values summed, each the copied card's on the
// battlefield and the printed card's elsewhere (targetMV's rule), plus the
// announced X for one that is a spell on the stack (CR 202.3e; objMV).
//
//go:noinline
func (s *State) rememberedMV(k *prep) int { return s.rememberedMVAgg(k, false) }

// rememberedMVAgg is rememberedMV, or with greatest the largest single mana
// value (TriggerRemembered$GreatestCardManaCost; ExMVOf N == AggGreatestMV).
//
//go:noinline
func (s *State) rememberedMVAgg(k *prep, greatest bool) int {
	n := 0
	for _, o := range s.g.rem[:s.g.nrem] {
		if s.faceDownAt(k, o) {
			continue // CR 708.2: a face-down permanent or spell has no mana cost
		}
		ob := &s.objs[o]
		mv := 0
		if ob.zone == kop.ZoneBattlefield {
			mv = s.objMV(k, s.cardAt(ob.slot), o) // static_cda.go
		} else {
			mv = s.objMV(k, ob.card, o) // static_cda.go
		}
		if greatest {
			n = max(n, mv)
		} else {
			n += mv
		}
	}
	return n
}

// The object-level mana-value reader behind every Ref-based MV read
// (findings-t4). Cold: it sits on the exprCold/targetMV/rememberedMV paths only.

// objMV is object o's mana value as gorge's state.ManaValue reads it in the
// common case: the printed total of card c, a split card's halves off the
// stack and battlefield (CR 709.4b), plus the announced X while o is a spell
// on the stack (CR 202.3e). Every object-reference MV read (ExMVOf,
// targetMV, rememberedMV) goes through here, so a new reader cannot forget
// the stack X term.
func (s *State) objMV(k *prep, c CardID, o Obj) int {
	if o != NoObj && s.objs[o].zone == kop.ZoneBattlefield && s.copiesBackFace(k, s.objs[o].slot) {
		return 0 // CR 712.8e
	}
	mv := int(k.cards[c].total)
	if k.gen.anySplit {
		mv = s.splitMV(k, c, o, mv) // face_castface.go
	}
	if o != NoObj {
		if nx := k.t.Cards[c].Cost.X; nx != 0 {
			mv += s.stackXMV(o, nx) // static_cda.go
		}
	}
	return mv
}

// mvPerm is the printed mana value of the permanent at sl, 0 for an embalm
// copy (a copy exception's "with no mana cost").
func (s *State) mvPerm(k *prep, sl Slot) int {
	if s.cpxNoCost(k, sl) || s.copiesBackFace(k, sl) {
		return 0
	}
	return int(k.cards[s.cardAt(sl)].total)
}

// copiesBackFace reports whether the permanent at sl is a copy (a clone's
// copy layer or a token copy) whose visible face is a transforming card's
// back face. Its mana value is 0 (CR 712.8e): the copiable values are the
// back face's, which has no mana cost, where the transformed card itself
// reads its front face's cost (gorge state.ManaValue's CopyFace case).
func (s *State) copiesBackFace(k *prep, sl Slot) bool {
	c := s.cardAt(sl)
	cd := &k.t.Cards[c]
	if cd.FaceMode != kop.FaceTransform || cd.Face == 0 {
		return false
	}
	return s.perms[sl].ovr.flags&ovCopy != 0 || s.objs[s.perms[sl].obj].of&ofCopy != 0
}

// notOwnLibMV is ExNotOwnLibMV: the mana values summed of the library cards
// in the resolution's remembered set that the repeated player does not own
// (gorge Count$ValidLibrary Card.IsRemembered+!RememberedPlayerOwn
// $CardManaCost). Zero outside an OpRepeatEachPlayer (repEnd == 0), as
// ctrlEachPl.
func (s *State) notOwnLibMV(k *prep) int {
	if s.g.repEnd == 0 {
		return 0
	}
	n := 0
	for _, o := range s.g.rem[:s.g.nrem] {
		ob := &s.objs[o]
		if ob.zone != kop.ZoneLibrary || ob.owner() == s.g.eachPl || s.faceDownAt(k, o) {
			continue
		}
		n += s.objMV(k, ob.card, o)
	}
	return n
}
