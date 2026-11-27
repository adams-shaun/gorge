package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A painland funding the filter search (filtSearch.visit's second pass) and
// the grant search (grantSrch.visit's funding loop).
// gorge's prefix search activates the covered source and its mana ask lists
// every ability the source can activate, in ability order, each one's colours
// in printed order: Karplusan Forest's {C}, then its painful {R} or {G}
// (CR 605.3a, 605.3b: the rider is no cost, so the painful ability is as
// activatable as the plain one). Each answer is a node of gorge's potentialScriptNodes
// budget, so a kernel search that funded with {C} alone reached a line gorge's
// ran out of budget before (Dauthi Embrace's {B}{B} beside School of the
// Unseen, Karplusan Forest and five Mountains: gorge leaves it unpriced and
// unoffered).

// painFunds reports whether funding source pc's ask lists a painful ability
// beside its plain one (a pain-only source's sole ability is its plain one).
func painFunds(k *prep, pc *pcard) bool {
	return k.painMana && pc.painCol != 0 && !pc.painOnly
}

// fundMaskAt is the colours funding source sl's ask answers: its census
// production and, for a painland, its painful ability's colours.
func (s *State) fundMaskAt(k *prep, pc *pcard, sl Slot) uint8 {
	m := s.srcMaskAt(pc, sl)
	if painFunds(k, pc) {
		m |= uint8(pc.painCol) & 0x3f
	}
	return m
}

// painFundOrder appends to out, after its first n answers, the painful
// ability's colours of mask in printed order (manaColourOrder), skipping the
// ones seen; it returns the new count and seen set.
func painFundOrder(k *prep, t *State, sl Slot, pc *pcard, mask uint8, out []uint8, n int, seen uint8) (int, uint8) {
	if !painFunds(k, pc) {
		return n, seen
	}
	cd := &k.t.Cards[t.cardAt(sl)]
	plain := 0
	for j := range cd.Abilities {
		a := &cd.Abilities[j]
		if a.Kind != kop.AbMana || a.AFlags&kop.AfLatent != 0 {
			continue
		}
		if plain++; plain < 2 {
			continue // the plain ability, the first (prepPainMana)
		}
		var ord [5]uint8
		for _, c := range ord[:manaColourOrder(a, &ord)] {
			if mask&(1<<c) != 0 && seen&(1<<c) == 0 && n < len(out) {
				out[n] = c
				seen |= 1 << c
				n++
			}
		}
		break
	}
	return n, seen
}

// fundPainBranch taps funding source i (slot sl), a painland (painFunds), for
// each answer of its mana ask in gorge's order (fundColourOrder): the ask is
// one node, the plain {C} first, then the painful colours, whose answer deals
// the rider as the ability resolves (CR 605.3b; tapPain). A painful answer a
// life cost cannot pay is not offered (CR 119.4; painAskOrder).
func (g *grantSrch) fundPainBranch(t *State, i int, sl Slot, pc *pcard) bool {
	mask := t.fundMaskAt(g.k, pc, sl)
	if bits.OnesCount8(mask) > 1 && !g.node() {
		return false
	}
	var order [kop.NumManaKinds]uint8
	n := pc.manaN
	var xpc *pcard
	if g.k.gen.anyTapExtra {
		xpc = t.stepView(g.k, sl, -1)
	}
	for _, c := range order[:fundColourOrder(g.k, t, sl, pc, mask, &order)] {
		pain := uint8(pc.manaCol)&(1<<c) == 0
		if pain && pc.painLife && int(t.pl[SeatIx(g.p)].life) < int(pc.painN) {
			continue
		}
		u := t.fork()
		u.perms[sl].flags |= pfTapped
		u.pl[SeatIx(g.p)].pool[c] += n
		u.artTag(g.k, g.p, sl, int(c))
		xv := u.srcExtraPick(g.k, sl, xpc, int(c))
		u.addXv(SeatIx(g.p), &xv)
		if pain {
			u.painDamage(g.k, sl, SeatIx(g.p), int(pc.painN))
		}
		g.path[g.np] = grantStep{i: uint8(i), col: c, n: n, gi: -1, sl: sl, pain: pain}
		g.np++
		ok := g.next(u, sl)
		t.unfork()
		if ok {
			return true
		}
		g.np--
	}
	return false
}
