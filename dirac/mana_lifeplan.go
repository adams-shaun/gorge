package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A life-only mana ability's planner price (kop.GapLifeManaAb: Treasonous
// Ogre's "Pay 3 life: Add {R}"). gorge's payment planner lists the source as a
// last-resort unit (pay.PlanLastResortCostOK admits a cost of PayLife alone):
// phase 1 plans from the printed sources, and only when none pays does phase 2
// add the paid-life alternative, never a plan whose summed life reaches the
// payer's life (the lethal guard, CR 119.4). A plain cast that plan pays is
// one the autopay surface keeps, so the kernel's scripted-offer drop
// (autopayDropsPlainCast) must leave it: availLife is the printed walk
// (availWalk) with those units added.
const availLife uint16 = 512

// availLifeGaps adds to a the life-only sources' units the planner's phase 2
// may spend together: each one's cost counted against the payer's life, the
// plan's sum staying below it.
func (s *State) availLifeGaps(k *prep, p uint8, a *avail) {
	life := int(s.pl[SeatIx(p)].life)
	pain := 0
	for b := s.lifeGapMask(k, p); b.any(); b = b.rest() {
		sl := b.first()
		ab := &k.t.Cards[s.cardAt(sl)].Abilities[k.gen.gapOther(s.cardAt(sl))]
		if pain+int(ab.Cost.N) >= life {
			continue
		}
		pain += int(ab.Cost.N)
		n := max(ab.ManaN, 1)
		if ab.ManaColors == kop.ColAny {
			a.addUnits(0x1f, n)
			continue
		}
		a.addUnits(1<<bits.TrailingZeros8(uint8(ab.ManaColors)), n)
	}
}

// lifePlanned reports whether the printed sources and p's life-only sources
// pay plain cast a (gorge's phase 2 plan). walk is the walk's lazily built
// options: built says it was.
func (s *State) lifePlanned(k *prep, p uint8, life slotMask, walk *Decision, built *bool, a Action) bool {
	if !life.any() || !s.lifeGapShaped(k, p, Obj(a.Source())) {
		return false // none, or a cast the planner never prices (an {X} cost)
	}
	if !*built {
		*built = true
		s.plainOptions(k, walk, availWalk|availLife)
	}
	return hasOption(walk, a)
}

// lifePlannedPlay reports whether gorge's planner prices play a with p's
// life-only sources on the autopay surface (its phase 2, mana_lifeplan.go),
// so no prefix search runs for it: the verdict is "sufficient" and the plan
// pays the play. A scripted gap that would also pay it (a depletion land's
// counter mana, listed before the life-only unit in battlefield order) is not
// the payment gorge's plan makes. The plain cast the autopay surface keeps is
// already on the plain walk (castGateAffordable); this is the play beside it
// whose own gate reads the plain census alone (an adventure face, a flashback
// or cycling cost, an activation: CR 601.2g, 118.3).
func (s *State) lifePlannedPlay(k *prep, p uint8, a Action) bool {
	if s.cfg&cfgGorgeAutopay == 0 || !s.lifeGapMask(k, p).any() {
		return false
	}
	if a.Kind() == AkCast && !s.lifeGapShaped(k, p, Obj(a.Source())) {
		return false // a cast the planner never prices (an {X} cost)
	}
	var d Decision
	s.plainOptions(k, &d, availLife)
	return hasOption(&d, a)
}
