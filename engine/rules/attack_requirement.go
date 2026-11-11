package rules

import (
	"maps"
	"slices"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules/combat"
	"github.com/adams-shaun/gorge/engine/state"
)

// stat:AttackRequirement is the printed S:Mode$ AttackRequirement static (War's
// Toll, Ekundu Cyclops, Magnetic Web, Viashino Bey): "if a creature [scope]
// attacks, all creatures [scope] attack if able". The reader is
// combat.AttackRequirementClauses; this file narrows the clauses to what the
// attackers decision offers and publishes them on the decision, where
// decision.AttackRequirementsSatisfied is the one rule.
func init() { effects.RegisterNonAPI("stat:AttackRequirement") }

// attackBounds are the decision's own limits a closure must fit within.
type attackBounds struct {
	max       int
	maxSum    int
	payerLife int32
}

// attackRequirementGroups narrows each AttackRequirement clause to the creatures
// opts offers an attack for (a creature with no offered pair is not "able", CR
// 508.1d, and is exempt) and keeps a clause only when its minimal satisfying
// declaration -- the required creatures, every member and one trigger -- fits
// bounds. It returns the kept closures and the creatures that can never attack
// legally (the triggers of a clause that cannot be completed), which the caller
// removes from the offer list so the decision stays answerable. A clause is
// OMITTED (unenforced) rather than enforced when a per-defender cap is in force
// (it can make the closure infeasible in ways this check does not model) or when
// a required creature is itself a trigger.
func attackRequirementGroups(clauses []combat.AttackRequirementClause, opts []decision.Option,
	required map[state.ObjID]bool, bounds attackBounds, capped bool) ([]decision.AttackRequirementGroup, map[state.ObjID]bool) {
	if len(clauses) == 0 || capped {
		return nil, nil
	}
	cheapest := make(map[state.ObjID]*decision.Option, len(opts)) // lookup only, never ranged
	for i := range opts {
		o := &opts[i]
		if c, ok := cheapest[o.Obj]; !ok || o.Value < c.Value {
			cheapest[o.Obj] = o
		}
	}
	var groups []decision.AttackRequirementGroup
	var barred map[state.ObjID]bool
	for _, cl := range clauses {
		g := decision.AttackRequirementGroup{
			Triggers: slices.DeleteFunc(slices.Clone(cl.Triggers), func(id state.ObjID) bool { return cheapest[id] == nil }),
			Members:  slices.DeleteFunc(slices.Clone(cl.Members), func(id state.ObjID) bool { return cheapest[id] == nil }),
		}
		if len(g.Triggers) == 0 || len(g.Members) == 0 {
			continue
		}
		if attackClauseFits(g, cheapest, required, bounds) {
			groups = append(groups, g)
			continue
		}
		if slices.ContainsFunc(g.Triggers, func(id state.ObjID) bool { return required[id] }) {
			continue
		}
		if barred == nil {
			barred = make(map[state.ObjID]bool)
		}
		for _, t := range g.Triggers {
			barred[t] = true
		}
	}
	return groups, barred
}

// attackClauseFits reports whether the cheapest declaration satisfying g -- the
// required creatures plus every member, plus the cheapest trigger when none of
// those already is one -- fits the Max ceiling, the Value budget and the life
// charge bound.
func attackClauseFits(g decision.AttackRequirementGroup, cheapest map[state.ObjID]*decision.Option,
	required map[state.ObjID]bool, b attackBounds) bool {
	var objs []state.ObjID
	add := func(id state.ObjID) {
		if cheapest[id] != nil && !slices.Contains(objs, id) {
			objs = append(objs, id)
		}
	}
	for _, m := range g.Members {
		add(m)
	}
	for _, o := range slices.Sorted(maps.Keys(required)) {
		add(o)
	}
	if !slices.ContainsFunc(objs, func(id state.ObjID) bool { return slices.Contains(g.Triggers, id) }) {
		best := g.Triggers[0]
		for _, t := range g.Triggers {
			if cheapest[t].Value < cheapest[best].Value {
				best = t
			}
		}
		add(best)
	}
	sum, life := 0, int32(0)
	for _, id := range objs {
		o := cheapest[id]
		sum += o.Value
		life += int32(o.CostLife) + int32(o.CostPhyrexian)*decision.PhyrexianLife
	}
	return len(objs) <= b.max && (b.maxSum <= 0 || sum <= b.maxSum) && (b.payerLife <= 0 || life <= b.payerLife)
}

// attackRequirementOffers is askAttackers' AttackRequirement step: the closures
// to publish and opts with the creatures that can never attack legally removed
// (options reindexed). With no AttackRequirement static it returns opts as is.
func attackRequirementOffers(b combat.Board, opts []decision.Option, mustAtt map[state.ObjID]bool, maxOpts, maxSum int,
	payerLife int32) ([]decision.AttackRequirementGroup, []decision.Option) {
	clauses := combat.AttackRequirementClauses(b)
	if len(clauses) == 0 {
		return nil, opts
	}
	// A per-defender cap (an option Group, the carrier of every GroupLimits entry too)
	// can make a closure infeasible in ways attackClauseFits does not model.
	capped := slices.ContainsFunc(opts, func(o decision.Option) bool { return o.Group != "" })
	groups, barred := attackRequirementGroups(clauses, opts, mustAtt, attackBounds{max: maxOpts, maxSum: maxSum, payerLife: payerLife}, capped)
	if len(barred) > 0 {
		opts = slices.DeleteFunc(opts, func(o decision.Option) bool { return barred[o.Obj] })
		for i := range opts {
			opts[i].Index = i
		}
	}
	return groups, opts
}
