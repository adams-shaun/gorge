package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// plainSacKeepsTarget: gorge's potential walk refuses a plain cast whose
// witness payment sacrifices every permanent the spell could target (the
// witness lowers the plan before the cast, potentialWitnessReaches), unless
// the target is announced before the mana is paid and some candidate survives
// the plan (witnessAnnouncesBeforePaying, CR 601.2c-h): Double Cleave over the
// one creature, a Catalyst Elemental that sacrifices itself for {R}{R}, is
// "unsupported witness_unreachable". It is the plain-cast form of kickWitness
// and xWitnessEats's target read (a spell with a second target slot is not
// read, announcedTargetKept). When the walk refuses, gorge's
// potentialWitnessKeeping re-plans with planKeepSources, where a plan that
// sacrifices its source is insufficient (potential_plan.go planKeepSources):
// a hybrid cost's planHybridCost then tries every pip colouring, so a
// colouring whose plan pays from tap sources alone is the cast's keeping plan
// and the cast is offered (witnessKeepingPips below). A one-colouring cost's
// replan is the same plan again, so the refusal stands for it.
func (s *State) plainSacKeepsTarget(k *prep, p uint8, c CardID, o Obj, cost *kop.ManaCost) bool {
	if !s.sacWitnessNeedsKeeping(k, p, c, o, cost) {
		return true
	}
	return s.witnessKeepingPips(k, p, o, cost)
}

// sacWitnessNeedsKeeping reports whether the plain witness for the cast of c
// consumes the only permanent the spell could target, and the target does not
// survive the payment: the cast is offered only through the keep-sources
// replan (plainSacKeepsTarget, witnessKeepingPips).
func (s *State) sacWitnessNeedsKeeping(k *prep, p uint8, c CardID, o Obj, cost *kop.ManaCost) bool {
	if !k.sacMana || cost.X != 0 || !s.anySacSource(k, p) {
		return false
	}
	ab := spellAb(k, c)
	if ab == 255 || tspec(k, c, ab, 0) == nil {
		return false
	}
	tmp := s.fork()
	defer s.unfork()
	n0 := tmp.npend
	var pk planner
	pk.try = true
	tmp.tapForPK(k, p, cost, 0, o, false, false, &pk)
	if !pk.found || !plannerConsumesSac(&pk) {
		return false
	}
	return !(tmp.spellTargetsFeasible(k, c, p) || s.announcedTargetKept(k, tmp, c, p) || tmp.witnessEndsGameWon(k, p, c, n0))
}

// lowerWitnessKeepsTarget is the cost lowerWitness's plain lowered cast taps:
// a hybrid cost whose plain witness consumes the spell's only target is
// offered through gorge's potentialWitnessKeeping replan
// (plainSacKeepsTarget), so the lowering taps that keeping colouring, not the
// plain witness's consuming one. Every other cost is returned unchanged.
func (s *State) lowerWitnessKeepsTarget(k *prep, p uint8, excl Obj, cost kop.ManaCost) kop.ManaCost {
	e := s.bld().e
	if cost.Hyb[0] == 0 || cost.Hyb[0] == kop.HybTwobrid || e.kind != seSpell ||
		e.mode != 0 || e.flags&(sfKicked|sfFlashback|sfMayPlay) != 0 {
		return cost
	}
	if r, ok := s.loweringKeepsTarget(k, p, excl, cost); ok {
		return r
	}
	return cost
}

// loweringKeepsTarget is the cost lowerWitness taps for a lowered plain cast
// whose plain witness consumes its only target: gorge's potentialWitnessKeeping
// lower is the same keep-sources plan, so the lowering taps that colouring,
// not the plain witness's (Shattering Blow's {1}{R/W} keeps its Treasure target
// and pays the {W} face from a Plains). ok is false when the plain witness
// needs no replan.
func (s *State) loweringKeepsTarget(k *prep, p uint8, o Obj, cost kop.ManaCost) (kop.ManaCost, bool) {
	if !s.sacWitnessNeedsKeeping(k, p, s.bld().e.card, o, &cost) {
		return cost, false
	}
	return s.witnessKeepingCost(k, p, o, &cost)
}

// witnessKeepingPips is plainSacKeepsTarget's keep-sources replan for a
// hybrid cost: every pip colouring of cost (gorge planHybridCost's order,
// pip 0 the fastest varying) is planned on a dry planner (the plan is
// selected, never tapped, cost_disjointmana.go's shape) and the first
// colouring whose plan consumes nothing is the keeping plan
// (potentialWitnessKeeping, potential_plan.go planKeepSources). The cost is
// resolved before the plan, so tapForPK's own hybrid walk (which stops at the
// first affording resolution) does not hide the later colourings.
func (s *State) witnessKeepingPips(k *prep, p uint8, o Obj, cost *kop.ManaCost) bool {
	_, ok := s.witnessKeepingCost(k, p, o, cost)
	return ok
}

// witnessKeepingCost is witnessKeepingPips with the colouring it finds: the
// first resolution whose dry plan consumes nothing (planKeepSources). The cost
// is resolved before the plan, so tapForPK's own hybrid walk (which stops at
// the first affording resolution) does not hide the later colourings.
func (s *State) witnessKeepingCost(k *prep, p uint8, o Obj, cost *kop.ManaCost) (kop.ManaCost, bool) {
	if cost.Hyb[0] == 0 || cost.Hyb[0] == kop.HybTwobrid {
		return kop.ManaCost{}, false // one colouring: the replan is the same plan again (a twobrid cost is announced before the payment)
	}
	n, f := hybFacesOf(cost)
	for m := 0; m < 1<<n; m++ {
		r := f.resolve(*cost, n, hybWitnessMask(m, n))
		var pk planner
		pk.try = true
		pk.dry = true
		pk.noGrant = true // hybResolveTappable's trial census: no granted source (mana_grantedoffers.go)
		s.tapForPK(k, p, &r, 0, o, false, false, &pk)
		if pk.found && !plannerConsumesSac(&pk) {
			return r, true
		}
	}
	return kop.ManaCost{}, false
}
