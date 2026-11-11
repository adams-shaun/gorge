package decision

import "github.com/adams-shaun/gorge/engine/state"

// AttackRequirementGroup is one AttackRequirement closure on a KAttackers
// decision: if the declaration includes any creature in Triggers it must
// include every creature in Members (both already narrowed to creatures the
// decision offers an attack for, so "if able" is built in).
type AttackRequirementGroup struct {
	Triggers []state.ObjID
	Members  []state.ObjID
}

// AttackRequirementsSatisfied reports whether the options choices names obey
// every AttackRequirement closure. It is the ONE rule: Validate rejects an
// answer that fails it, FitRequired's repair builds toward it, and the engine's
// validateAttackDeclarationChosen asks it too, so a repaired answer can never
// be one the engine rejects. Out-of-range indices are ignored.
func (d *Decision) AttackRequirementsSatisfied(choices []int) bool {
	for i := range d.AttackRequirements {
		if !d.attackGroupSatisfied(&d.AttackRequirements[i], choices) {
			return false
		}
	}
	return true
}

func (d *Decision) attackGroupSatisfied(g *AttackRequirementGroup, choices []int) bool {
	if !d.groupTriggered(g, choices) {
		return true
	}
	for _, m := range g.Members {
		if !d.objChosen(m, choices) {
			return false
		}
	}
	return true
}

// groupTriggered reports whether some chosen option's Obj is a Triggers member.
func (d *Decision) groupTriggered(g *AttackRequirementGroup, choices []int) bool {
	for _, c := range choices {
		if c < 0 || c >= len(d.Options) {
			continue
		}
		for _, t := range g.Triggers {
			if d.Options[c].Obj == t {
				return true
			}
		}
	}
	return false
}

func (d *Decision) objChosen(obj state.ObjID, choices []int) bool {
	for _, c := range choices {
		if c >= 0 && c < len(d.Options) && d.Options[c].Obj == obj {
			return true
		}
	}
	return false
}

// repairAttackRequirements makes out obey every AttackRequirement closure: for a
// triggered closure it adds the cheapest admissible option of each missing
// member (Max, the budgets, the life charge, the per-Group caps and the
// one-pair-per-creature rule all still hold). When a member cannot be added and
// drop is set, it removes every chosen option whose Obj triggers that closure
// instead -- the empty declaration is always legal, so the repair never loops
// and never returns an answer Validate rejects for this rule. With drop unset
// it only adds (the pre-fold pass over the required core).
func (d *Decision) repairAttackRequirements(out []int, drop bool) []int {
	for range 2 * (len(d.AttackRequirements) + 1) * (len(out) + len(d.Options) + 1) {
		changed := false
		for i := range d.AttackRequirements {
			g := &d.AttackRequirements[i]
			if d.attackGroupSatisfied(g, out) {
				continue
			}
			next, ok := d.fillMembers(g, out)
			if ok {
				out, changed = next, true
				continue
			}
			if drop {
				out, changed = d.dropTriggers(g, out), true
			}
		}
		if !changed {
			break
		}
	}
	return out
}

// fillMembers adds the cheapest admissible option of every member of g not yet
// in out, or reports false (out unchanged) when some member has none.
func (d *Decision) fillMembers(g *AttackRequirementGroup, out []int) ([]int, bool) {
	next := append([]int(nil), out...)
	for _, m := range g.Members {
		if d.objChosen(m, next) {
			continue
		}
		best := -1
		for c := range d.Options {
			o := &d.Options[c]
			if o.Obj != m || (best >= 0 && o.Value >= d.Options[best].Value) || !d.requirementAdmits(next, c) {
				continue
			}
			best = c
		}
		if best < 0 {
			return out, false
		}
		next = append(next, best)
	}
	return next, true
}

// requirementAdmits reports whether option c can join out without breaking the
// decision's own bounds: Max, the Value budgets, the life charge, the tap pool
// and the per-Group caps. One pair per creature is the caller's (objChosen).
func (d *Decision) requirementAdmits(out []int, c int) bool {
	if len(out) >= d.maxChoices() {
		return false
	}
	with := append(append(make([]int, 0, len(out)+1), out...), c)
	sum, sum2 := 0, 0
	for _, i := range with {
		sum += d.Options[i].Value
		sum2 += d.Options[i].Value2
	}
	return d.BudgetsFit(sum, sum2) && d.ChargeOptionsFit(with) && !d.groupCapExceeded(with)
}

// dropTriggers removes every chosen option whose Obj triggers g.
func (d *Decision) dropTriggers(g *AttackRequirementGroup, out []int) []int {
	kept := out[:0:0]
	for _, c := range out {
		trig := false
		for _, t := range g.Triggers {
			if c >= 0 && c < len(d.Options) && d.Options[c].Obj == t {
				trig = true
			}
		}
		if !trig {
			kept = append(kept, c)
		}
	}
	return kept
}
