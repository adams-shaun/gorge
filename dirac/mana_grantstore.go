package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A storage battery (Mercadian Bazaar's "{T}, Remove any number of storage
// counters: Add {R} for each counter removed this way", mana_countercost.go
// storeGap) beside a granted mana ability (Nature's Embrace's "{T}: Add two
// mana of any one color" on a Forest): gorge's census lists neither (a
// SubCounter<X> cost is no planner unit, PaymentAbility excludes the grant),
// and its one prefix search (potential_script.go potentialPrefixScript)
// activates every census gap in battlefield order, the battery among the
// grants. So the grant search (grantSrch) lists the battery as a gap candidate
// too: the enchanted Forest's {B}{B}, a Swamp, two Forests and the Bazaar's
// two counters pay Haunting Hymn's {4}{B}{B} (CR 107.3, 601.2g-h, 605.1a).

// stepStore is the gi of a prefix step that activates a storage battery for
// every counter it holds.
const stepStore int16 = -8

// grantStoreGaps is p's activatable storage batteries the grant search lists:
// a battery with no plain first mana ability beside it (ctrSecond), whose one
// activation asks nothing (its X is the counters it holds, its colour fixed).
func (s *State) grantStoreGaps(k *prep, p uint8) slotMask {
	var m slotMask
	if !k.ctrMana {
		return m
	}
	for b := s.scriptCtr(k, p); b.any(); b = b.rest() {
		sl := b.first()
		c := s.cardAt(sl)
		if kop.StoreManaAb(k.ctrAbOf(c)) && !k.ctrSecond(c) {
			m = m.with(sl)
		}
	}
	return m
}

// storeBranch activates the battery candidate i (slot sl) on a copy of t: a
// fixed colour and an X at the bound, so no ask node (gorge's non-interactive
// SubCounter stage).
func (g *grantSrch) storeBranch(t *State, i int, sl Slot) bool {
	a := g.k.ctrAbOf(t.cardAt(sl))
	n := t.perms[sl].ctrN(t, int(a.Cost.Ctr))
	if n <= 0 || a.Cost.Flags&kop.CostTap != 0 && t.perms[sl].flags&pfTapped != 0 || g.np >= len(g.path) {
		return false
	}
	u := t.fork()
	u.storeGap(g.k, g.p, sl, a)
	g.path[g.np] = grantStep{i: uint8(i), col: uint8(bits.TrailingZeros8(uint8(a.ManaColors) & 0x3f)), n: uint8(min(n, 255)), gi: stepStore, sl: sl}
	g.np++
	var ok bool
	if g.tapsLive {
		ok = g.settle(u) // (the tap may have fired a trigger)
	} else {
		ok = g.visit(u)
	}
	t.unfork()
	if !ok {
		g.np--
	}
	return ok
}

// storeBesideGrant reports whether p has an activatable storage battery while
// a permanent of p's holds a granted mana ability: the grant search is gorge's
// one prefix search over both, so applyPriority runs it before the scripted
// gap prefix, which would activate the battery alone.
func (s *State) storeBesideGrant(k *prep, p uint8) bool {
	if !s.grantStoreGaps(k, p).any() {
		return false
	}
	for m := s.ctrlM[SeatIx(p)].and(s.occ); m.any(); m = m.rest() {
		var gi [maxGrants]int16
		if s.grantIdxs(k, m.first(), &gi) > 0 {
			return true
		}
	}
	return false
}

// storeStep reports whether the found prefix activates a storage battery:
// the offer builder's units hold no battery's counters, so grantCap offers
// the play the prefix pays (CR 601.2g-h).
func (g *grantSrch) storeStep() bool {
	for i := 0; i < g.np; i++ {
		if g.path[i].kind == stepAct && g.path[i].gi == stepStore {
			return true
		}
	}
	return false
}
