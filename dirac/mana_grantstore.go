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

// grantStoreIdleGaps is p's storage batteries holding no counters whose {T} is
// free: gorge's census lists none of them (Amount$ X resolves to zero with no X
// announced, manaAbilityYieldsNothing), but one that holds a granted ability is
// a gap through the grant, and its activation ask lists the printed storage
// ability first: X is 0, so the {T} is paid and no mana is added (CR 107.3,
// 602.2). The search takes that answer before the grant's, then goes on.
func (s *State) grantStoreIdleGaps(k *prep, p uint8) slotMask {
	var m slotMask
	if !k.ctrMana {
		return m
	}
	for b := s.scriptCtrIdle(k, p, true).andNot(s.scriptCtr(k, p)); b.any(); b = b.rest() {
		sl := b.first()
		c := s.cardAt(sl)
		if kop.StoreManaAb(k.ctrAbOf(c)) && !k.ctrSecond(c) {
			m = m.with(sl)
		}
	}
	return m
}

// storeIdleTap pays an idle storage battery's {T} (X = 0): the tap, no counters
// removed and no mana added.
func (s *State) storeIdleTap(k *prep, sl Slot, a *kop.Ability) {
	pm := &s.perms[sl]
	if a.Cost.Flags&kop.CostTap == 0 || pm.flags&pfTapped != 0 {
		return
	}
	pm.flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	s.tapExtraStall(k, sl)
}

// storeBranch activates the battery candidate i (slot sl) on a copy of t: a
// fixed colour and the X ask's answers, the counters it holds first and 0
// last (gorge's search branches every X the ask offers, each one a node: the
// ask's own is charged first). An idle battery (no counters) pays its {T}
// alone and asks nothing. A grant holder's "choose a mana ability" ask comes
// first, one node, the battery's answer before the grants': asked reports it
// charged, so the grant side does not charge it again.
func (g *grantSrch) storeBranch(t *State, i int, sl Slot) (found, asked bool) {
	a := g.k.ctrAbOf(t.cardAt(sl))
	n := t.perms[sl].ctrN(t, int(a.Cost.Ctr))
	if a.Cost.Flags&kop.CostTap != 0 && t.perms[sl].flags&pfTapped != 0 || g.np >= len(g.path) {
		return false, false
	}
	if n <= 0 && a.Cost.Flags&kop.CostTap == 0 {
		return false, false // nothing to pay and nothing made: no step
	}
	if g.ng[i] > 0 {
		asked = true
		if !g.node() { // the "choose a mana ability" ask
			return false, true
		}
	}
	if n <= 0 {
		return g.storeTry(t, i, sl, a, 0), asked
	}
	if !g.node() { // the X ask
		return false, asked
	}
	for x := min(n, 255); x >= 0; x-- {
		if g.storeTry(t, i, sl, a, x) {
			return true, asked
		}
	}
	return false, asked
}

// storeTry takes the X ask's answer x on a copy of t and continues the search
// there.
func (g *grantSrch) storeTry(t *State, i int, sl Slot, a *kop.Ability, x int) bool {
	if g.np >= len(g.path) {
		return false
	}
	u := t.fork()
	u.storeGapX(g.k, g.p, sl, a, x)
	g.path[g.np] = grantStep{i: uint8(i), col: uint8(bits.TrailingZeros8(uint8(a.ManaColors) & 0x3f)), n: uint8(x), gi: stepStore, sl: sl}
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
		if g.path[i].kind == stepAct && g.path[i].gi == stepStore && g.path[i].n > 0 {
			return true
		}
	}
	return false
}

// autopayScriptedCast reports a plain cast a under the autopay surface: gorge's
// priority list drops a plain cast it reaches only by a scripted prefix (the
// oracle's priorityVocabulary), so a prefix that activates a storage battery
// does not add it to the offers, where the full surface lists it and an
// ability the script pays (Iron Suitcase's {3}) is listed either way
// (CR 601.2g-h).
func (s *State) autopayScriptedCast(a Action) bool {
	return s.cfg&cfgGorgeAutopay != 0 && a.Kind() == AkCast && a.Mode() == 0
}
