package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// lowerDynSacGap is gorge's potential prefix script for an activation the
// floating pool cannot pay and only a dynamic {T}, sacrifice source funds
// (Lotus Blossom's "{T}, Sacrifice Lotus Blossom: Add X mana of any one
// colour", X its petal counters): gorge's planner cannot price an
// evaluated-amount source whose cost sacrifices it (the probe needs a
// TapOnly shape), so the potential walk offers the play as a prefix script
// and runScript activates the source -- tap, sacrifice, float -- BEFORE the
// play is pushed, then the play pays the rest from the floating pool
// (tail-74 pair 1, decision 477). The kernel's census prices the source for
// the offer (avail.dynSac) and its planner taps it at the mana step, after
// targets, so the state at the target ask diverged: the Lotus still on the
// battlefield with its counters, the pool empty. Float here, before the
// build's push, with gorge's script colour rule (fundOrder/needFirst), and
// leave the remainder to the mana step's own plan as gorge's payCast does.
func (s *State) lowerDynSacGap(k *prep) {
	b := s.bld()
	e := &b.e
	if e.kind != seAbility || k.gen == nil {
		return
	}
	ab := s.buildAbility(k)
	if ab == nil || ab.Cost.Flags&^(kop.CostSorcery|kop.CostOncePerTurn|kop.CostOncePerGame) != 0 {
		return // a cost with a tap, sacrifice or counter part pays it in its own build stage
	}
	m := ab.Cost.Mana
	if m.X != 0 || m.XRed != 0 || m.XPip != 0 || m.Hyb[0] != 0 || m.Tb != 0 || m.Snow != 0 {
		return // a cost whose composition is not final at the build's start
	}
	p := e.ctrl
	cost := s.buildCost(k)
	pips := pipMaskOf(&cost)
	var av avail
	s.availForMode(k, p, &av, slotMask{}, false)
	if av.affords(&cost, pips, 0) {
		return // gorge's planner prices it: the planned route taps after the push
	}
	av.dynSac = true
	s.availForMode(k, p, &av, slotMask{}, false)
	if !av.affords(&cost, pips, 0) {
		return // the prefix search finds nothing either
	}
	var order [MaxPerm]Slot
	n := s.battlefieldOrder(p, s.ctrlM[p], order[:], 0)
	for _, sl := range order[:n] {
		if !s.dynSacFloatable(k, p, sl) {
			continue
		}
		s.dynSacFloat(k, p, sl, grantNeedColours(cost))
		av.dynSac = false
		s.availForMode(k, p, &av, slotMask{}, false)
		if av.affords(&cost, pips, 0) {
			return
		}
	}
}

// dynSacFloatable mirrors the census walk's gate (availForMode) for a
// dynamic {T}, sacrifice source: the plain census prices the source in the
// dynSac census (a.dynSac) only through this gate, so a source that passes it
// is a unit the prefix script funds.
func (s *State) dynSacFloatable(k *prep, p uint8, sl Slot) bool {
	pc := s.manaPCGen(k, sl)
	if !(pc.manaDyn && pc.manaSac) {
		return false
	}
	if pc.manaAb < 0 && s.perms[sl].ovr.flags&ovCopy == 0 {
		return false
	}
	if s.g.noGrant && s.g.availX&availPlain != 0 && s.manaGranted(k, sl) {
		return false // grantCap/grantPrefix's search, not another gap's
	}
	if !s.canTapForManaPC(k, sl, pc) {
		return false
	}
	if !s.deferLive(k) || s.g.availX&(availWalk|availHyp) != 0 {
		return true
	}
	return !s.scriptTapsDefer(k, p).has(sl) || !s.planDeferred(k, p, sl)
}

// dynSacFloat activates the dynamic {T}, sacrifice source at sl into the
// pool: tap, float its whole production in the script's colour, then
// sacrifice it, as gorge's runScript does and the funding path (fundSac)
// mirrors. Its colour answer is the script ask's (fundOrder): the source's
// printed colour order, the colours it does not name after in W U B R G C
// order, the play's needed colours moved first.
func (s *State) dynSacFloat(k *prep, p uint8, sl Slot, need [kop.NumManaKinds]bool) {
	pc := s.manaPCGen(k, sl)
	mask := s.srcMaskAt(pc, sl)
	var ord [kop.NumManaKinds]uint8
	n := 0
	var seen uint8
	if cd := &k.t.Cards[s.cardAt(sl)]; pc.manaAb >= 0 && int(pc.manaAb) < len(cd.Abilities) {
		var pr [5]uint8
		m := manaColourOrder(&cd.Abilities[pc.manaAb], &pr)
		for _, c := range pr[:m] {
			if mask&(1<<c) != 0 {
				ord[n] = c
				seen |= 1 << c
				n++
			}
		}
	}
	for m := mask &^ seen; m != 0 && n < len(ord); m &= m - 1 {
		ord[n] = uint8(bits.TrailingZeros8(m))
		n++
	}
	needFirst(ord[:n], need)
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	s.poolAdd(p, int(ord[0]), int(pc.manaN))
	s.addExtraCol(k, p, sl, pc, int(ord[0]))
	s.artTag(k, p, sl, int(ord[0]))
	s.sacFilterSource(k, sl)
}
