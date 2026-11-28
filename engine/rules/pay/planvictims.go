package pay

import (
	"cmp"
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
// fundPlanVictims itself, so another member of a class stands in for a
// witness's unit that is the only victim. A board with fewer free victims than
// the set has such steps (two altars, one creature), or whose only victim is a
// source every member choice spends, then leaves the search to rank the sets it
// can fund: a second other-sacrifice source never removes a way to pay
// (CR 601.2g-h). Adding units only adds steps and takes candidates away, so no
// superset of a refused set is fundable either. choices and classes are the
// tables of the search it gates.
func planVictimGate(e Engine, p state.PlayerID, choices [][]Alt, classes []Class, kept, tapped, gone []state.ObjID) func([]Alt) bool {
	return func(chosen []Alt) bool {
		if !slices.ContainsFunc(chosen, func(a Alt) bool { return a.Consequence.SacrificeOthers }) {
			return true
		}
		plan := decision.PaymentPlan{Activations: make([]decision.PaymentActivation, len(chosen))}
		for i := range chosen {
			plan.Activations[i] = chosen[i].Activation
			plan.Activations[i].Consequence = chosen[i].Consequence.Wire()
		}
		return fundPlanVictims(e, p, &plan, choices, classes, slices.Concat(kept, tapped), gone)
	}
}

// AssignPlanVictims names the victims of every other-sacrifice step of plan,
// in step order, by the fixed ranking, never an object the plan otherwise
// spends (a source, a kept or gone object, an earlier step's victim). It
// returns the index of the first step it cannot fund with distinct victims,
// -1 when every step has its victims. A plan with no such step is untouched.
func AssignPlanVictims(e Engine, p state.PlayerID, plan *decision.PaymentPlan, kept, gone []state.ObjID) int {
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
			return i
		}
		victims := PaymentPlanSacrificeVictims(e, p, pa.Source, part, int(part.N), reserved)
		if victims == nil {
			return i
		}
		pa.Sacrifices = victims
		for _, v := range victims {
			reserved[v] = true
		}
	}
	return -1
}

// fundPlanVictims names plan's victims, first changing which members of a
// class the plan taps when the witness's choice leaves a step none: the search
// fixes how MANY units of each class pay, so any member of the class pays the
// same (CR 601.2g-h). Each swap untaps one victim candidate and taps a member
// that is none (swapPlanMember). It reports false when no member choice funds
// every step; the plan is then not offered. choices and classes are the
// tables of the search that materialized plan.
func fundPlanVictims(e Engine, p state.PlayerID, plan *decision.PaymentPlan, choices [][]Alt, classes []Class, kept, gone []state.ObjID) bool {
	for range len(plan.Activations) + 1 {
		short := AssignPlanVictims(e, p, plan, kept, gone)
		if short < 0 {
			return true
		}
		if !swapPlanMember(e, p, plan, short, choices, classes, kept, gone) {
			break
		}
	}
	return false
}

// swapPlanMember frees one victim for the unfunded step short: among the plan's
// sources that step could sacrifice and whose class has an unused member it
// could not, the victim ranking picks the one to free, and that class's
// lowest-id such member takes its step. The steps return to census order, the
// witness order. It reports false when no source can be exchanged.
func swapPlanMember(e Engine, p state.PlayerID, plan *decision.PaymentPlan, short int, choices [][]Alt, classes []Class, kept, gone []state.ObjID) bool {
	step := plan.Activations[short]
	part, ok := stepSacPart(e, p, PaymentPlanQueryUnits(e, p), step)
	if !ok {
		return false
	}
	cands := ManaSacrificeCandidates(e, p, step.Source, part)
	reserved := planVictimReserve(plan.Activations, kept, gone)
	for _, a := range plan.Activations[:short] {
		for _, v := range a.Sacrifices {
			reserved[v] = true
		}
	}
	position := make(map[state.ObjID]int, len(choices))
	for i, alts := range choices {
		if len(alts) != 0 {
			position[alts[0].Activation.Source] = i
		}
	}
	// spare maps each source that can be freed to the unit replacing it.
	spare := map[state.ObjID]int{}
	for _, a := range plan.Activations {
		v := a.Source
		if !slices.Contains(cands, v) || slices.Contains(kept, v) || slices.Contains(gone, v) {
			continue
		}
		if m, ok := spareMember(plan.Activations, choices, classes, position[v], cands); ok {
			spare[v] = m
		}
	}
	for v := range spare {
		delete(reserved, v)
	}
	freed := PaymentPlanSacrificeVictims(e, p, step.Source, part, 1, reserved)
	if freed == nil {
		return false
	}
	at := slices.IndexFunc(plan.Activations, func(a decision.PaymentActivation) bool { return a.Source == freed[0] })
	old, m := plan.Activations[at], spare[freed[0]]
	alt := slices.IndexFunc(choices[position[freed[0]]], func(x Alt) bool {
		return x.Activation.Ability == old.Ability && x.Activation.Produces == old.Produces
	})
	if alt < 0 {
		return false
	}
	next := choices[m][alt].Activation
	next.Consequence = choices[m][alt].Consequence.Wire()
	plan.Activations[at] = next
	slices.SortStableFunc(plan.Activations, func(x, y decision.PaymentActivation) int {
		return cmp.Compare(position[x.Source], position[y.Source])
	})
	return true
}

// spareMember is the lowest-id unit of the class of the unit at position at
// that no step of acts taps and that is not among cands.
func spareMember(acts []decision.PaymentActivation, choices [][]Alt, classes []Class, at int, cands []state.ObjID) (int, bool) {
	k := slices.IndexFunc(classes, func(c Class) bool { return slices.Contains(c.Members, at) })
	if k < 0 {
		return 0, false
	}
	best, bestID := -1, state.ObjID(0)
	for _, m := range classes[k].Members {
		id := choices[m][0].Activation.Source
		used := slices.ContainsFunc(acts, func(a decision.PaymentActivation) bool { return a.Source == id })
		if !used && !slices.Contains(cands, id) && (best < 0 || id < bestID) {
			best, bestID = m, id
		}
	}
	return best, best >= 0
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
