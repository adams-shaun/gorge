package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A reflected TapsForMana extra over a source that makes a choice of colours
// (Roxanne, Starfall Savant over her Meteorite token's {T}: Add one mana of
// any color): the extra is one more mana of the colour the tap made (gorge
// pay.PaymentPlanReflectedMana, folded per planned production, CR 106.12a), so
// the tap is one source making n+1 mana of one colour of its choice, which the
// planner already prices (pcard.manaOne). Such a source's view is prebuilt per
// card (prepManaTapRefl) and chosen per slot (tapReflFolds); a source that is
// not that shape (a Treasure's sacrifice, a painful, restricted or
// {C}-or-colour one) keeps stalling (mana_tapextra.go srcExtraTapped).

// tableHasTapRefl: a card has a reflected StatTapExtra static.
func tableHasTapRefl(t *Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if a := &t.Cards[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatTapExtra && a.Static.StFlags&kop.StTapReflect != 0 {
				return true
			}
		}
	}
	return false
}

// tapReflShape: pc is one plain {T} ability making n mana of one of several
// colours, no {C} beside them.
func tapReflShape(pc *pcard) bool {
	m := srcMask(pc)
	return pc.manaAb >= 0 && pc.manaN > 0 && pc.manaN < 255 && m&(1<<kop.C) == 0 && bits.OnesCount8(m) >= 2 &&
		pc.manaTap && !pc.manaSac && !pc.manaSacOnly && !pc.manaChosen && !pc.manaDyn && !pc.manaMulti && !pc.reflFold &&
		pc.restrU == 0 && pc.rankCol == 0 && pc.painCol == 0 && !pc.painOnly && pc.wideN == 0 && pc.manaXv == [kop.NumManaKinds]uint8{}
}

// prepManaTapRefl builds card id's folded view when the table has a reflected
// extra (prepareGen, once per card, after the card's own views): the plain
// view with one more mana, all of one colour. A card with a dynamic or gated
// view of its own is left to stall.
func prepManaTapRefl(g *genPrep, p *prep, id CardID, pc *pcard) {
	if !g.anyTapRefl || g.has(id, gfManaVar) || !tapReflShape(pc) {
		return
	}
	g.smallFor(id).reflVar = int16(len(g.varPC))
	v := *pc
	v.manaN++
	v.manaOne = true
	v.reflFold = true
	g.varPC = append(g.varPC, v)
	p.manaOne = true
}

// tapReflVar is the varPC index of card c's folded view.
func (g *genPrep) tapReflVar(c CardID) int16 {
	if r := g.small(c); r != nil {
		return r.reflVar
	}
	return -1
}

// tapReflFolds reports whether exactly one reflected StatTapExtra of the
// controller's, of one mana, reaches a tap of sl: its extra is the folded
// view's one more mana. Two of them (or a larger amount) leave the plain
// view, which stalls.
func (s *State) tapReflFolds(k *prep, sl Slot) bool {
	ctl := s.perms[sl].ctrl
	n := 0
	s.eachStatic(k, func(st statSrc) bool {
		a := &st.a.Static
		if a.StFlags&kop.StTapReflect == 0 || !s.tapExtraReaches(k, st, sl, ctl) {
			return true
		}
		n += int(a.B)
		return true
	})
	return n == 1
}
