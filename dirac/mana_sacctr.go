package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A counter-scaled sacrifice source (Shrine of Boundless Growth: "{T},
// Sacrifice this: Add {C} for each charge counter on it") is a census gap
// like a storage battery: gorge's planner does not price a dynamic amount, so
// its prefix search activates the source and the play is priced from the
// floating mana (CR 605.1a, 106.4). A plain cast reached only that way is one
// the autopay surface drops; an alternative cast (Kolaghan Skirmisher's dash)
// and an activation stay. The kernel lists it among the counter-cost gaps
// (scriptCtr): the gaps walk counts its live counters (availCtr) and the
// chain's step taps it, adds that much mana and sacrifices it (ctrGapDo). It
// is no grant-search candidate and no ward or unless window pick, as before.

// sacCtrManaAb reports whether mana ability a is that shape: {T} and a
// sacrifice of the source, no mana, one fixed colour, as many as the counters
// of one kind on the source.
func sacCtrManaAb(a *kop.Ability) bool {
	return a != nil && a.Kind == kop.AbMana && a.Cost.Flags == kop.CostTap|kop.CostSacSelf &&
		a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0 && a.AFlags == kop.AfManaDynamic &&
		a.ManaV.K == kop.VCountersSelf && a.ManaColors != 0 && a.ManaColors&(a.ManaColors-1) == 0 &&
		a.ManaRestrict == 0 && a.ManaRestrictAb == 0 && !a.ManaEach && !a.ManaDeferred &&
		a.ManaRider == kop.ManaRiderNone && len(a.Prog) == 0 && a.Trig.Cond == 0
}

// sacCtrUsable reports whether the counter-scaled sacrifice source at sl can
// be activated now: it holds at least one counter of the ability's kind, and
// is untapped, not summoning sick, its abilities kept, and sacrificable
// (CR 602.2b). A source with no counter could pay the cost, but its amount
// resolves to zero, so gorge's census drops it (payment_window_yield.go's
// manaAbilityYieldsNothing: it can contribute to no payment and no prefix
// activates it). Listing it would sacrifice it for nothing, a sacrifice
// gorge never makes.
func (s *State) sacCtrUsable(k *prep, sl Slot, a *kop.Ability) bool {
	pm := &s.perms[sl]
	return pm.ctrN(s, int(a.ManaV.N)) > 0 &&
		pm.flags&pfTapped == 0 && pm.ovr.flags&(ovLoseAll|ovLoseAllEOT) == 0 && !s.activationSick(k, sl) && !s.sacBlocked(k, sl)
}

// sacCtrUnits is the mana the source at sl adds now: its counters of the
// ability's kind.
func (s *State) sacCtrUnits(sl Slot, a *kop.Ability) int {
	n := s.perms[sl].ctrN(s, int(a.ManaV.N))
	if n > 255 {
		s.capFull(StallDynMana)
		n = 255
	}
	return n
}

// sacCtrGap activates the counter-scaled sacrifice source at sl for p: the
// {T}, then the mana its counters count, then the sacrifice (gorge pays the
// cost first and the amount reads the source's last known counters; the pool
// is the same either way). It always runs the activation's trigger passes
// (manaTapped, fireSac): the shape is no grant-search candidate
// (mana_grantedoffers.go), so ctrGapDo's fire false fork-side branch never
// reaches it and there is no deferred grantApply pass to leave them to.
func (s *State) sacCtrGap(k *prep, p uint8, sl Slot, a *kop.Ability) {
	pm := &s.perms[sl]
	if pm.flags&pfTapped != 0 {
		return
	}
	n := s.sacCtrUnits(sl, a)
	pm.flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	col := bits.TrailingZeros8(uint8(a.ManaColors) & 0x3f)
	if n > 0 {
		s.poolAdd(SeatIx(p), col, n)
		s.artTag(k, p, sl, col)
	}
	o := pm.obj
	s.fireSac(k, sl)
	s.leave(k, sl, kop.ZoneGraveyard, false)
	if s.g.on && s.objs[o].of&ofToken != 0 && s.objs[o].zone == kop.ZoneGraveyard {
		s.detach(o) // a token ceases at once, as gorge pays
	}
}
