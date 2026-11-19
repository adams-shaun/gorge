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
	if !k.sacMana || cost.X != 0 || !s.anySacSource(k, p) {
		return true
	}
	ab := spellAb(k, c)
	if ab == 255 || tspec(k, c, ab, 0) == nil {
		return true
	}
	tmp := s.fork()
	defer s.unfork()
	n0 := tmp.npend
	var pk planner
	pk.try = true
	tmp.tapForPK(k, p, cost, 0, o, false, false, &pk)
	if !pk.found || !plannerConsumesSac(&pk) {
		return true
	}
	if tmp.spellTargetsFeasible(k, c, p) || s.announcedTargetKept(k, tmp, c, p) || tmp.witnessEndsGameWon(k, p, c, n0) {
		return true
	}
	return s.witnessKeepingPips(k, p, o, cost)
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
	if cost.Hyb[0] == 0 || cost.Hyb[0] == kop.HybTwobrid {
		return false // one colouring: the replan is the same plan again (a twobrid cost is announced before the payment)
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
			return true
		}
	}
	return false
}
