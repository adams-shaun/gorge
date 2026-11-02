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

// targetMV is Targeted$CardManaCost (gorge evalRefProperty over the
// targeted object, whose id survives its moves): the mana value of the
// chosen target's card -- the copied card while it is on the battlefield
// (its derived characteristics), the printed card elsewhere (a copy effect
// ends as it leaves); 0 for an illegal target. objMV adds the announced X
// when the target is a spell on the stack (CR 202.3e).
//
//go:noinline
func (s *State) targetMV(k *prep, r kop.Ref, ctx evalCtx) int {
	o := s.refObjCtx(r, ctx)
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
func (s *State) rememberedMV(k *prep) int {
	n := 0
	for _, o := range s.g.rem[:s.g.nrem] {
		if s.faceDownAt(k, o) {
			continue // CR 708.2: a face-down permanent or spell has no mana cost
		}
		ob := &s.objs[o]
		if ob.zone == kop.ZoneBattlefield {
			n += s.objMV(k, s.cardAt(ob.slot), o) // static_cda.go
			continue
		}
		n += s.objMV(k, ob.card, o) // static_cda.go
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
	if s.cpxNoCost(k, sl) {
		return 0
	}
	return int(k.cards[s.cardAt(sl)].total)
}
