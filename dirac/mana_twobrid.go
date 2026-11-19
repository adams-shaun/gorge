package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Monocoloured hybrid mana ({2/W}, CR 107.4e). A twobrid pip is
// kop.ManaCost.Tb, and Hyb[0] is kop.HybTwobrid while any pip is unresolved,
// so every flexible-pip gate that tests Hyb[0] reaches its cold branch, which
// dispatches here. The caster announces each pip's face before targets
// (CR 601.2b; gorge pay.AnnouncePip): its colour (mode 0..4), then two
// generic (mode 6). The answers ride build.pipN / build.pipC[0] (bit i set:
// pip i paid generic). The mana value counts two per pip (CR 202.3f;
// kop.ManaCost.MV).

// twobridGeneric is the announcement mode of a twobrid pip paid as two
// generic mana (the colour faces are the colour indexes 0..4).
const twobridGeneric = 6

// tableTwobrid reports whether some card's printed cost carries a twobrid
// pip. A cost reduction or increase composes after the announced faces
// (foldTb), as gorge composes the modifiers on the faces an announcement
// resolved, so no table is stalled for carrying both.
func tableTwobrid(t *Table) bool {
	for i := range t.Cards {
		if t.Cards[i].Cost.Tb != 0 {
			return true
		}
	}
	return false
}

// foldTb folds the faces announced so far into the printed cost c of the
// spell being built, so the cost statics that follow compose on the resolved
// faces (gorge composes an announced face into the cost before the
// modifiers, CR 601.2b then 601.2f; the unresolved pips stay for the pips
// not yet announced). done reports c carried twobrid pips, resolved or not.
func (s *State) foldTb(k *prep, c kop.ManaCost) (kop.ManaCost, bool) {
	if c.Hyb[0] != kop.HybTwobrid {
		return c, false
	}
	return resolveTbBuild(c, s.bld().pipN, s.bld().pipC[0]), true
}

// adjustPlain is adjustCostGen when gen, else adjustCost.
func (s *State) adjustPlain(gen bool, k *prep, p uint8, c CardID, o Obj, base kop.ManaCost) kop.ManaCost {
	if gen {
		return s.adjustCostGen(k, p, c, o, base)
	}
	return s.adjustCost(k, p, c, o, base)
}

// adjustFit is adjustCost for an offer gate. A twobrid cost composes its
// statics on the announced faces (gorge FeasibleAny enumerates the faces
// where the composition could depend on them, and a face-independent one
// gives the same answer), so it returns the adjusted cost of the first face
// assignment av affords, and the adjusted unresolved cost (which av then
// refuses too: a face only lowers it) when none is.
func (s *State) adjustFit(k *prep, av *avail, p uint8, c CardID, o Obj, base kop.ManaCost, gen bool) kop.ManaCost {
	if base.Hyb[0] != kop.HybTwobrid {
		return s.adjustPlain(gen, k, p, c, o, base)
	}
	n := base.TbN()
	phy := base.Phyrexian()
	for m := 0; m < 1<<n; m++ {
		if phy && 2*bits.OnesCount(uint(m)) > int(av.life) {
			continue // the two-life faces must be payable (CR 119.4)
		}
		r := base
		for i := 0; i < n; i++ {
			r = r.ResolveTb(m>>i&1 != 0)
		}
		r = s.adjustPlain(gen, k, p, c, o, r)
		// the face's delve credit (gorge FeasibleAny composes then takes it at
		// the leaf, CR 702.66a): the resolved faces' generic, so a face the
		// credit pays is found and returned with the credit in it -- the
		// caller skips its outer min(generic, graveyard) credit then
		// (avail.tbLeafCredited)
		if cr := min(int(r.Generic), av.delve); cr > 0 {
			r.Generic -= uint8(cr)
		}
		if av.affordsPlain(&r, pipMaskOf(&r), 0) {
			return r
		}
	}
	return s.adjustPlain(gen, k, p, c, o, base)
}

// affordsTb: some face assignment of cost's twobrid pips is affordable
// (gorge pay.ResolveManaWith expands each pip to {colour, two generic}).
func (a *avail) affordsTb(cost *kop.ManaCost, extra int, anyColor bool) bool {
	n := cost.TbN()
	phy := cost.Phyrexian()
	for m := 0; m < 1<<n; m++ {
		if phy && 2*bits.OnesCount(uint(m)) > int(a.life) {
			continue // the two-life faces must be payable (CR 119.4)
		}
		r := *cost
		for i := 0; i < n; i++ {
			r = r.ResolveTb(m>>i&1 != 0)
		}
		// the leaf's delve credit (gorge FeasibleAny composes then takes it at
		// the leaf, CR 702.66a): the resolved faces' generic first, then the
		// {X} part's remainder (gorge delveAsk's order)
		ex := extra
		if a.delve > 0 {
			cr := min(int(r.Generic)+ex, a.delve)
			g := min(int(r.Generic), cr)
			r.Generic -= uint8(g)
			ex -= cr - g
		}
		if anyColor {
			if a.affordsAnyColorPlain(&r, ex) { // every pip resolved: no hybrid left
				return true
			}
		} else if a.affordsPlain(&r, pipMaskOf(&r), ex) {
			return true
		}
	}
	return false
}

// pipOptionsTb poses twobrid pip pipN (gorge ManaAsk): the colour face when
// some assignment of the later pips makes the cost payable with it, then the
// generic face when one does; the colour alone when neither does. A face's
// cost is the printed cost with every pip's face folded in, the cost statics
// composed on it (gorge announceFeasible walks the later pips through
// FeasibleAny, which composes on the resolved faces).
func (s *State) pipOptionsTb(k *prep, d *Decision, cost kop.ManaCost, av *avail) {
	e := &s.bld().e
	// gorge announceFeasible prices the pip menus with the cast's potential
	// delve while the ask is deferred past them (cast_subcounter.go's CR
	// 601.2b defer): the graveyard's size, taken off each resolved leaf's
	// generic -- the picks do not exist yet, so nothing was announced.
	grave := 0
	if k.gen != nil && k.gen.anyDelve {
		grave = s.tbDelveCredit(k, e.ctrl, e.card, e.obj, s.castFromHandBy(e, e.ctrl))
	}
	col := uint8(cost.TbCol(0))
	second := uint8(twobridGeneric)
	phy := cost.Phyrexian()
	life := int(av.life)
	if phy {
		// a Phyrexian pip's second face: two life (gorge pay_life), from
		// the life the announced life faces leave
		second = phyrexianLife
		life -= 2 * s.phyLifeAnnounced()
	}
	rem := cost.TbN() - 1
	for face := 0; face < 2; face++ {
		for m := 0; m < 1<<rem; m++ {
			if phy && 2*(face+bits.OnesCount(uint(m))) > life {
				continue // the two-life faces must be payable (CR 119.4)
			}
			r := s.buildCostFaces(k, face, m, rem)
			// a face the mana window can reach through a filter or a converting source is
			// offered as gorge's castWindowReachable offers it (Jeskai Devotee's {1} for {R})
			r = r.AtX(int(e.x))
			x := 0
			if grave > 0 {
				cr := min(int(r.Generic)+x, grave)
				g := min(int(r.Generic), cr)
				r.Generic -= uint8(g)
				x -= cr - g
			}
			if !s.faceReachable(k, e.ctrl, av, &r, x) {
				continue
			}
			if face == 1 {
				d.add(MakeAction(AkChoose, 0, 0, second))
			} else {
				d.add(MakeAction(AkChoose, 0, 0, col))
			}
			break
		}
	}
	if d.N == 0 {
		d.add(MakeAction(AkChoose, 0, 0, col))
	}
}

// buildCostFaces is the build's cost with the next twobrid pip announced as
// face (1: generic) and the rem pips after it as the bits of later (bit set:
// generic), so no pip is left; the answers it would record, and nothing kept.
func (s *State) buildCostFaces(k *prep, face, later, rem int) kop.ManaCost {
	b := s.bld()
	n, faces := b.pipN, b.pipC[0]
	b.pipC[0] |= uint8(face)<<n | uint8(later)<<(n+1)
	b.pipN += uint8(1 + rem)
	c := s.buildCost(k)
	b.pipN, b.pipC[0] = n, faces
	return c
}

// buildIsTb reports whether the cast being built still announces twobrid
// pips (its unresolved cost carries the marker).
func (s *State) buildIsTb(k *prep) bool {
	if k.gen == nil {
		return false
	}
	if k.phyMana && s.bld().e.kind == seAbility {
		return s.buildCost0(k).Phyrexian() // an activation's Phyrexian pips
	}
	return k.gen.anyTwobrid && s.bld().e.kind == seSpell && s.buildCost0(k).Hyb[0] == kop.HybTwobrid
}

// answerPipTb records the face announced for the next twobrid pip.
func (s *State) answerPipTb(mode uint8) {
	if mode == twobridGeneric || mode == phyrexianLife {
		s.bld().pipC[0] |= 1 << s.bld().pipN
	}
	s.bld().pipN++
}

// resolveTbBuild replays the announced faces onto c.
func resolveTbBuild(c kop.ManaCost, pipN, faces uint8) kop.ManaCost {
	for i := 0; i < int(pipN) && c.Hyb[0] == kop.HybTwobrid; i++ {
		c = c.ResolveTb(faces>>i&1 != 0)
	}
	return c
}

// tbDemand is the twobrid pips of cost c in colour col (gorge CostPips: a
// twobrid pip demands its colour once).
func tbDemand(c *kop.ManaCost, col int) int {
	n := 0
	for i, nt := 0, c.TbN(); i < nt; i++ {
		if c.TbCol(i) == col {
			n++
		}
	}
	return n
}

// tbDevotion is the twobrid pips of c whose colour meets mask (CR 700.5).
func tbDevotion(c *kop.ManaCost, mask kop.ColorMask) int {
	n := 0
	for i, nt := 0, c.TbN(); i < nt; i++ {
		if mask&(1<<uint(c.TbCol(i))) != 0 {
			n++
		}
	}
	return n
}
