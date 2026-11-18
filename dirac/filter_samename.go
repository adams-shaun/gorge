package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// sameNameT0 is PredSameNameT0 for the permanent at sl: the resolving
// ability's target 0 itself, or another permanent with its name (gorge
// Maelstrom Pulse's TargetedCard.Self or sharesNameWith Targeted; a
// face-down permanent has no name, CR 708.2). other excludes the target
// itself (PredN 1: Targeted.sameName+Other, Echoing Decay).
func (s *State) sameNameT0(sl Slot, other bool) bool {
	if !s.res.active {
		return false
	}
	e := &s.res.e
	t := e.tgt[0]
	if t == NoTarget || isPlayerOrNone(t) {
		return false
	}
	ob := &s.objs[t]
	if ob.zone != kop.ZoneBattlefield || ob.ver != e.tver[0] {
		return false
	}
	if ob.slot == sl {
		return !other
	}
	n := s.nameCardAt(ob.slot)
	return n != NoCard && n == s.nameCardAt(sl)
}

// predSameNameCard is the card matcher's sharesNameWith arm: Remembered
// reads the remembered set, Self the reading source's current name (Marvin,
// Murderous Mimic) with PredN 1 as the ordinary ! negation.
func (s *State) predSameNameCard(fl *kop.Filter, c CardID, o Obj, fd bool, ctx *evalCtx) bool {
	if fl.Pred == kop.PredSameNameRem {
		return s.sharesNameRem(o, c, fd)
	}
	cx := s.ctxCur()
	if ctx != nil {
		cx = *ctx
	}
	return s.sharesNameSelf(cx.srcSl, cx.src, NoSlot, c, o, fd, fl.PredN == 1)
}

// sharesNameSelf is PredSameNameSelf (gorge sharesNameWith Self): the
// candidate's current name equals the reading source's current name. The
// source is a battlefield permanent at src, or its object srcObj when it has
// no slot (a source still on the stack); the candidate is a permanent at sl,
// or the card c at object o. The name identity is sharesNameRem's: a
// battlefield permanent reads its layer-3 name (nameCardAt), anything else
// its card. A face-down candidate has no name (CR 708.2); neg is the
// ordinary ! negation.
func (s *State) sharesNameSelf(src Slot, srcObj Obj, sl Slot, c CardID, o Obj, fd, neg bool) bool {
	sn := NoCard
	if src != NoSlot {
		sn = s.nameCardAt(src)
	} else if srcObj != NoObj {
		sn = s.objName(srcObj)
	}
	cn := s.nameKey(c)
	if sl != NoSlot {
		cn = s.nameCardAt(sl)
	} else if o != NoObj {
		cn = s.objName(o)
	}
	eq := !fd && cn != NoCard && sn != NoCard && cn == sn
	if neg {
		return !eq
	}
	return eq
}

// objName is o's current name card: its layer-3 name (nameCardAt) on the
// battlefield, else its card.
func (s *State) objName(o Obj) CardID {
	if s.objs[o].zone == kop.ZoneBattlefield {
		return s.nameCardAt(s.objs[o].slot)
	}
	return s.nameKey(s.objs[o].card)
}

// sameNameT0Card is PredSameNameT0 for the card c at object o off the
// battlefield (gorge's "TargetedCard.Self,Card.NotDefinedTargeted+
// sharesNameWith Targeted" over a graveyard: Rat King, Verminister): the
// candidate is target 0 itself, or another card with its name. The target's
// legality is the one resolveTop took as the resolution started (res.bad):
// the sweep matches card by card as it moves them, and gorge's referent
// keeps naming the target after the sweep has moved it (an ObjID survives
// the zone change), so a card after the target in the graveyard still
// matches. A graveyard card is face up; fd only refuses a face-down
// candidate's name (CR 708.2). The Other form (predN != 0) reads the
// battlefield only, so it never matches here.
func (s *State) sameNameT0Card(predN int16, c CardID, o Obj, fd bool) bool {
	if predN != 0 || !s.res.active || o == NoObj || s.res.bad&1 != 0 { // seat-ok: a resolution flag bit
		return false
	}
	t := s.res.e.tgt[0]
	if t == NoTarget || isPlayerOrNone(t) {
		return false
	}
	if Obj(t) == o {
		return true
	}
	return !fd && s.nameKey(s.objs[t].card) == s.nameKey(c)
}
