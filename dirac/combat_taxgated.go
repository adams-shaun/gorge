package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugAtkTaxGatedOne (test only, TestFourSeatAtkTaxGatedPairLockstep): the
// walk counts a gated larger mana alternative (Whisperer of the Wilds' gated
// {G}{G}) as the one {G} the window taps, not as the larger unit count.
var DebugAtkTaxGatedOne bool

// atkTaxGatedPair reports whether card c's two listed mana abilities are a
// gated larger pair of one fixed colour (Whisperer of the Wilds, Fanatic of
// Rhonas: {T}: Add {G}, then Ferocious {T}: Add {G}{G}, gatedLargerPair): both
// plain {T} abilities the window lists (atkTaxManaAb), the second behind an
// IsPresent$ gate (Trig.Cond) and making more. gorge's census lists the
// gate-on source as one unit with both alternatives, so its budget counts the
// larger (attackSourceUnits: the max over a unit's alternatives, CR 605.3a)
// while the window taps the first listed alternative (CR 508.1h). The kernel
// reads the gate through the source's gate-on view (manaGateView, wideN).
func atkTaxGatedPair(c *kop.CardDef, first int) bool {
	if first < 0 || !atkTaxManaAb(&c.Abilities[first]) {
		return false
	}
	second, n := -1, 0
	for j := range c.Abilities {
		if a := &c.Abilities[j]; a.Kind == kop.AbMana && a.AFlags&kop.AfLatent == 0 && !atkTaxUnlisted(a) {
			n++
			if j != first {
				second = j
			}
		}
	}
	if n != 2 || second < 0 {
		return false
	}
	a0, ag := &c.Abilities[first], &c.Abilities[second]
	return second > first && a0.Trig.Cond == 0 && ag.Trig.Cond != 0 && atkTaxManaAb(ag) && a0.AFlags == 0 && ag.AFlags == 0 &&
		a0.ManaColors == ag.ManaColors && a0.ManaColors&^0x3f == 0 && a0.ManaN >= 1 && !a0.ManaEach && !ag.ManaEach && ag.ManaN > a0.ManaN
}

// atkTaxWide is the units the source at sl adds beyond what its window tap
// makes: a gate-on pair view's larger alternative (wideN), 0 for any other.
func (s *State) atkTaxWide(k *prep, sl Slot) int {
	if DebugAtkTaxGatedOne {
		return 0
	}
	if pc := s.manaPCGen(k, sl); s.widePairAb(k, sl, pc) >= 0 && k.gen.flags[s.cardAt(sl)]&gfManaGate != 0 && s.canTapForManaPC(k, sl, pc) {
		return int(pc.wideN)
	}
	return 0
}

// DebugAtkTaxSplitRoot (test only, TestFourSeatAtkTaxCondSplitLockstep): the
// walk reads a Condition-split source's root production ({U}) after the arm's
// gate holds (a land played), not the arm's ({B}).
var DebugAtkTaxSplitRoot bool

// atkTaxSplitAb reports whether mana ability a is a Condition-split source of
// the shape the walk reads through the window's view (splitWindowPC): River of
// Tears' "{T}: Add {U}. If you played a land this turn, add {B} instead", one
// fixed colour at the root and one at the arm. gorge's census prices the
// source by the arm's production while the gate holds
// (condSplitRootSuperseded) and the tap makes it (CR 605.1a, 106.1).
func atkTaxSplitAb(a *kop.Ability) bool {
	return a.SplitGate != kop.SplitGateNone && a.ManaN == 1 && a.SplitCols&^0x3f == 0 && a.ManaColors&^0x3f == 0 &&
		a.SplitCols != 0 && a.SplitCols&(a.SplitCols-1) == 0 && a.ManaColors != 0 && a.ManaColors&(a.ManaColors-1) == 0
}

// atkTaxSplitCard reports whether card c has a Condition-split mana ability
// beside another mana ability: the unit's alternatives are not modelled.
func atkTaxSplitCard(c *kop.CardDef) bool {
	for j := range c.Abilities {
		if a := &c.Abilities[j]; a.Kind == kop.AbMana && a.SplitGate != kop.SplitGateNone {
			return true
		}
	}
	return false
}

// DebugBlkTaxSacDropPair (test only, TestFourSeatBlkTaxSelfSacLockstep): a
// declared blocker the block window sacrifices takes its pair with it, as if
// the declaration had been committed after the payment.
var DebugBlkTaxSacDropPair bool

// blockTombstone marks the attackers the creature at sl blocks as blocked
// before a window sacrifice takes it. gorge's block window pays before it
// commits the declaration (completeBlockPay emits DeclareBlockers last), and a
// declared blocker is a source the window may sacrifice (an Eldrazi Spawn or
// Scion pays for its own block), so the pair is committed with the dead
// creature: the attacker's BlockedBy keeps it, the attacker is blocked and its
// "becomes blocked" triggers fire (events.foldDeclareBlockers; CR 509.1h, 506.4).
// The kernel records the pair as it is picked and the sacrifice releases it; the
// attacker's blocked mark is the tombstone a blocker that left combat keeps
// (pfBlocked), set here as that blocker leaves.
func (s *State) blockTombstone(sl Slot) {
	p := &s.perms[sl]
	if p.flags&pfBlocking == 0 || DebugBlkTaxSacDropPair {
		return
	}
	for _, a := range [2]Slot{p.blocking, s.xblkOf(sl)} {
		if a != NoSlot {
			s.perms[a].flags |= pfBlocked
			s.touchSl(a)
		}
	}
}
