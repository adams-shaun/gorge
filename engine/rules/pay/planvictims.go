package pay

import (
	"slices"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	costvocab "github.com/adams-shaun/gorge/engine/rules/cost"
	"github.com/adams-shaun/gorge/engine/state"
)

// Other-sacrifice witness victims (the Ashnod's Altar family): the planner
// names the permanents each "Sacrifice another permanent: Add" step
// sacrifices (AssignPlanVictims), and validation and the executor re-check
// that the named victims are still legal for their step (PlanVictimsLegal)
// before the step runs. One ranking (PaymentPlanSacrificeVictims) picks them.

// stepSacPart is the single Sac part of an other-sacrifice step's ability,
// re-resolved from the witness step at this state.
func stepSacPart(e Engine, p state.PlayerID, units []WindowUnit, pa decision.PaymentActivation) (costvocab.CostPart, bool) {
	step, ok := PaymentPlanStepAlternative(e, units, pa)
	if !ok || step.Ma == nil || !step.Consequence.SacrificeOthers {
		return costvocab.CostPart{}, false
	}
	cost := e.Eval().ManaAbilityCost(p, pa.Source, step.Ma).Cost
	if len(cost.Sac) != 1 {
		return costvocab.CostPart{}, false
	}
	return cost.Sac[0], true
}

// planVictimReserve is every object a plan already spends: its sources, the
// payment's kept and gone objects, and the victims named so far.
func planVictimReserve(acts []decision.PaymentActivation, extra ...[]state.ObjID) map[state.ObjID]bool {
	reserved := make(map[state.ObjID]bool, len(acts))
	for _, a := range acts {
		reserved[a.Source] = true
	}
	for _, ids := range extra {
		for _, id := range ids {
			reserved[id] = true
		}
	}
	return reserved
}

// planVictimGate is the planner search's Env.Fundable: whether every
// other-sacrifice step among a paying set's units can name its own victim, by
// AssignPlanVictims itself. A board with fewer free victims than the set has
// such steps (two altars, one creature), or whose only victim is a source the
// set spends, then leaves the search to rank the sets it can fund: a second
// other-sacrifice source never removes a way to pay (CR 601.2g-h). Adding
// units only adds steps and takes candidates away, so no superset of a refused
// set is fundable either.
func planVictimGate(e Engine, p state.PlayerID, kept, tapped, gone []state.ObjID) func([]Alt) bool {
	return func(chosen []Alt) bool {
		if !slices.ContainsFunc(chosen, func(a Alt) bool { return a.Consequence.SacrificeOthers }) {
			return true
		}
		plan := decision.PaymentPlan{Activations: make([]decision.PaymentActivation, len(chosen))}
		for i := range chosen {
			plan.Activations[i] = chosen[i].Activation
			plan.Activations[i].Consequence = chosen[i].Consequence.Wire()
		}
		return AssignPlanVictims(e, p, &plan, slices.Concat(kept, tapped), gone)
	}
}

// AssignPlanVictims names the victims of every other-sacrifice step of plan,
// in step order, by the fixed ranking, never an object the plan otherwise
// spends (a source, a kept or gone object, an earlier step's victim). It
// reports false when some step cannot be funded with distinct victims; the
// plan is then not offered. A plan with no such step is untouched.
func AssignPlanVictims(e Engine, p state.PlayerID, plan *decision.PaymentPlan, kept, gone []state.ObjID) bool {
	var reserved map[state.ObjID]bool
	var units []WindowUnit
	for i := range plan.Activations {
		pa := &plan.Activations[i]
		if pa.Consequence == nil || !pa.Consequence.SacrificeOthers {
			continue
		}
		if reserved == nil {
			reserved = planVictimReserve(plan.Activations, kept, gone)
			units = PaymentPlanQueryUnits(e, p)
		}
		part, ok := stepSacPart(e, p, units, *pa)
		if !ok {
			return false
		}
		victims := PaymentPlanSacrificeVictims(e, p, pa.Source, part, int(part.N), reserved)
		if victims == nil {
			return false
		}
		pa.Sacrifices = victims
		for _, v := range victims {
			reserved[v] = true
		}
	}
	return true
}

// PlanVictimsLegal reports whether every other-sacrifice step from index
// next on can still sacrifice exactly the victims the witness names: each
// is a current candidate for its step's Sac part, none is a plan source,
// and no two steps share one. A witness whose victim left, changed or
// became ineligible stops before the step (the manual window follows).
func PlanVictimsLegal(e Engine, p state.PlayerID, units []WindowUnit, acts []decision.PaymentActivation, next int) bool {
	var reserved map[state.ObjID]bool
	for i, pa := range acts {
		if i < next || pa.Consequence == nil || !pa.Consequence.SacrificeOthers {
			continue
		}
		if reserved == nil {
			reserved = planVictimReserve(acts)
		}
		part, ok := stepSacPart(e, p, units, pa)
		if !ok || len(pa.Sacrifices) != int(part.N) {
			return false
		}
		cands := ManaSacrificeCandidates(e, p, pa.Source, part)
		for _, v := range pa.Sacrifices {
			if reserved[v] || v == pa.Source || !slices.Contains(cands, v) {
				return false
			}
			reserved[v] = true
		}
	}
	return true
}

// otherSacFundable reports whether an other-sacrifice alternative of source
// has at least one victim other than the source on the board now, so the
// census never hands the search a step no plan could fund.
func otherSacFundable(e Engine, p state.PlayerID, source state.ObjID, ma *cards.SA) bool {
	cost := e.Eval().ManaAbilityCost(p, source, ma).Cost
	if len(cost.Sac) != 1 {
		return false
	}
	return PaymentPlanSacrificeVictims(e, p, source, cost.Sac[0], int(cost.Sac[0].N), nil) != nil
}
