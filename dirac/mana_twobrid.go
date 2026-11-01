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
// pip, and whether the table also reduces or raises spell costs (gorge
// composes a reduction on the unresolved cost at the offer gate, leftover
// dropped, but on the resolved faces at the announcement and the charge; the
// kernel's buildCost adjusts before it resolves, so that mix stalls).
func tableTwobrid(t *Table) (any, mod bool) {
	for i := range t.Cards {
		if t.Cards[i].Cost.Tb != 0 {
			any = true
			break
		}
	}
	mod = any && (tableHasStatic(t, kop.StatReduceCost) || tableHasStatic(t, kop.StatIncreaseCost))
	return
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
		if anyColor {
			if a.affordsAnyColorPlain(&r, extra) { // every pip resolved: no hybrid left
				return true
			}
		} else if a.affordsPlain(&r, pipMaskOf(&r), extra) {
			return true
		}
	}
	return false
}

// pipOptionsTb poses twobrid pip pipN (gorge ManaAsk): the colour face when
// the cost resolved with it is affordable over the later pips, then the
// generic face when that one is; the colour alone when neither is.
func (s *State) pipOptionsTb(d *Decision, cost kop.ManaCost, av *avail) {
	e := &s.bld().e
	col := uint8(cost.TbCol(0))
	second := uint8(twobridGeneric)
	if cost.Phyrexian() {
		// a Phyrexian pip's second face: two life (gorge pay_life), from
		// the life the announced life faces leave
		second = phyrexianLife
		av.life -= int16(2 * s.phyLifeAnnounced())
	}
	for face := 0; face < 2; face++ {
		r := cost.ResolveTb(face == 1)
		if e.flags&sfAnyMana != 0 && e.kind == seSpell {
			r = anyTypePips(r)
		}
		if face == 1 && second == phyrexianLife && av.life < 2 {
			continue // two life it cannot pay (CR 119.4)
		}
		if av.affords(&r, pipMaskOf(&r), r.XGeneric(int(e.x))) {
			if face == 1 {
				d.add(MakeAction(AkChoose, 0, 0, second))
			} else {
				d.add(MakeAction(AkChoose, 0, 0, col))
			}
		}
	}
	if d.N == 0 {
		d.add(MakeAction(AkChoose, 0, 0, col))
	}
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
