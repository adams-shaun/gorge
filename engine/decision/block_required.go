package decision

import (
	"encoding/binary"
	"slices"

	"github.com/adams-shaun/gorge/engine/state"
)

// blockSearchBudget bounds one required-block search's nodes, about a second of
// work. A board past it answers with the best legal team found, which may obey
// fewer requirements than CR 509.1c's maximum, rather than hanging the game. A
// declaration obeying more is still accepted.
const blockSearchBudget = 1 << 20

// blocker is one unit of the required-block search: a creature and the pairs it
// may declare.
type blocker struct {
	id       state.ObjID
	opts     []int
	required bool
}

// blockRequiredCore finds the largest legal team of required blockers (CR
// 509.1c). Unlike a pairwise matching, a block of a Min$ N attacker is legal
// only when the rest of the team can join it. The same result supplies the
// engine's quota and the bot's repair; no separate board-side feasibility
// estimate can disagree with it. Called only on decisions with MustBlock.
//
// A requirement is either BLOCKER-oriented (Option.BlockMust/Required: a
// particular creature must block) or ATTACKER-oriented (Option.AttackMust: a
// particular attacker must receive at least one blocker, "CARDNAME must be
// blocked if able"). The team maximizes the TOTAL number of satisfied
// requirements, so a declaration that satisfies one of each always beats one
// that satisfies only the other; ties take the shortest team.
func (d *Decision) blockRequiredCore() []int {
	return d.blockRequiredSearch(blockSearchBudget)
}

func (d *Decision) blockRequiredSearch(budget int) []int {
	// No requirement of either orientation: no candidate blocker, so the
	// search below returns nil -- skip building its tables.
	if !d.hasRequiredBlocks() {
		return nil
	}
	var blocks []blocker
	pos := make(map[state.ObjID]int)
	minAttackers := make(map[state.ObjID]bool)
	// atkReq is the set of attackers carrying an attacker-oriented
	// requirement. Each is satisfied once by ANY chosen option naming it.
	atkReq := make(map[state.ObjID]bool)
	// atkReq2 is the set of attackers carrying the two-blocker requirement
	// (AttackMustTwo). Each is satisfied once, by the SECOND chosen option
	// naming it, which the per-attacker count below makes exact.
	atkReq2 := make(map[state.ObjID]bool)
	for _, o := range d.Options {
		if o.AttackMust {
			atkReq[o.Attacker] = true
		}
		if o.AttackMustTwo {
			atkReq2[o.Attacker] = true
		}
		if (o.BlockMust || o.Required || o.AttackMust || o.AttackMustTwo) && o.MinBlockers > 1 {
			minAttackers[o.Attacker] = true
		}
	}
	for i, o := range d.Options {
		p, ok := pos[o.Obj]
		if !ok {
			p = len(blocks)
			pos[o.Obj] = p
			blocks = append(blocks, blocker{id: o.Obj})
		}
		blocks[p].required = blocks[p].required || o.BlockMust || o.Required
		blocks[p].opts = append(blocks[p].opts, i)
	}
	// Required creatures first; BlockMustAll options are separate required
	// blocker-attacker pairs, allowing one explicitly permitted blocker to
	// satisfy several defined attacker duties. Ordinary MustBlock options
	// remain one unit per blocker. Other blockers are helpers only for a
	// multi-blocker minimum or an attacker-oriented requirement.
	var candidates, helpers []blocker
	for _, b := range blocks {
		var ordinary blocker
		ordinary.id = b.id
		var allOrdinary []int
		for _, i := range b.opts {
			o := d.Options[i]
			if o.BlockMustAll {
				candidates = append(candidates, blocker{id: b.id, opts: []int{i}, required: true})
				continue
			}
			allOrdinary = append(allOrdinary, i)
			if o.BlockMust || o.Required {
				ordinary.required = true
			}
			if minAttackers[o.Attacker] || atkReq[o.Attacker] || atkReq2[o.Attacker] {
				ordinary.opts = append(ordinary.opts, i)
			}
		}
		if ordinary.required {
			// A required blocker searches all its offered pairs (another
			// pair may serve a Min$ team), but only pairs that obey its
			// requirement count toward the maximum (CR 509.1c).
			ordinary.opts = allOrdinary
			candidates = append(candidates, ordinary)
		} else if len(ordinary.opts) != 0 {
			helpers = append(helpers, ordinary)
		}
	}
	requiredCount := len(candidates)
	candidates = append(candidates, helpers...)
	counts := make(map[state.ObjID]int)
	satAtk := make(map[state.ObjID]bool)
	satTwo := 0
	lifeBound := d.PayerLifeBound()
	var chosen, best []int
	bestRequired := -1
	bestLength := int(^uint(0) >> 1)
	// Where only the blockers per attacker can matter later, a node reached a
	// second time holds nothing the first visit didn't try, and the first best
	// team found stays the answer. Teams of a Min$ attacker reach most nodes
	// many times over. Attackers offered identically are interchangeable, so a
	// node is the same whichever of them holds which count.
	var classes [][]state.ObjID
	var seen map[string]bool
	var key []byte
	var tally []uint64
	if d.blockSearchMemoizable(lifeBound) {
		seen = make(map[string]bool)
		classes = interchangeableAttackers(d, candidates)
	}
	// Each attacker's team minimum, for the bound below.
	var attackers []state.ObjID
	var teamNeeds []int
	teamMin := make(map[state.ObjID]int)
	if len(minAttackers) > 0 {
		for _, o := range d.Options {
			m, ok := teamMin[o.Attacker]
			if !ok {
				attackers = append(attackers, o.Attacker)
				m = o.MinBlockers
			}
			teamMin[o.Attacker] = min(m, o.MinBlockers)
		}
		teamNeeds = make([]int, 0, len(attackers))
	}
	nodes := 0
	var search func(int, int, int, int32)
	search = func(at, satisfied, spent int, spentLife int32) {
		if bestRequired == requiredCount+len(atkReq)+len(atkReq2) {
			return
		}
		if nodes++; nodes > budget {
			return
		}
		if seen != nil && at < len(candidates) {
			key = binary.AppendUvarint(key[:0], uint64(at))
			key = binary.AppendUvarint(key, uint64(satisfied))
			for _, class := range classes {
				tally = tally[:0]
				for _, a := range class {
					n := uint64(counts[a]) << 1
					if satAtk[a] {
						n |= 1
					}
					tally = append(tally, n)
				}
				slices.Sort(tally)
				for _, n := range tally {
					key = binary.AppendUvarint(key, n)
				}
			}
			if seen[string(key)] {
				return
			}
			seen[string(key)] = true
		}
		// Prune what can't beat the best team: more satisfied units, or as
		// many with a shorter team. Each candidate left picks one pair, worth
		// at most one required blocker and one required attacker, so this
		// bound holds and an answer found first stays the answer. Without the
		// cap on attackers, more must-be-blocked attackers than blockers
		// enumerate every assignment.
		left := max(requiredCount-at, 0)
		// Each chosen option obeys at most one attacker unit: the first
		// option naming an attacker its AttackMust unit, the second its
		// AttackMustTwo unit.
		twoLeft := len(atkReq2) - satTwo
		unsat := min(len(atkReq)-len(satAtk)+twoLeft, len(candidates)-at)
		if len(minAttackers) > 0 {
			served, ok := teamsWithin(attackers, teamMin, counts, atkReq, satAtk, len(candidates)-at, teamNeeds)
			if !ok {
				return
			}
			unsat = min(unsat, served+twoLeft)
		}
		need := bestRequired - satisfied
		if need > left+unsat {
			return
		}
		if need == left+unsat && len(chosen)+max((need+1)/2, need-left) >= bestLength {
			return
		}
		if at == len(candidates) {
			for _, ci := range chosen {
				o := d.Options[ci]
				if o.MinBlockers > counts[o.Attacker] {
					return
				}
			}
			// CR 506.5: a team that declares a can't-block-alone creature by
			// itself is no legal declaration, so it cannot be the maximum.
			if !d.AloneLegal(chosen) {
				return
			}
			if satisfied > bestRequired || (satisfied == bestRequired && len(chosen) < bestLength) {
				bestRequired, bestLength = satisfied, len(chosen)
				best = append([]int(nil), chosen...)
			}
			return
		}
		b := candidates[at]
		try := func(ci int) {
			o := d.Options[ci]
			if len(chosen) >= d.maxChoices() || (d.HasBudget() && spent+o.Value > d.MaxSum) {
				return
			}
			// The search must publish a declaration accepted by Validate:
			// ordinary pairs are exclusive per blocker, while BlockAllDefined
			// permits multiple pairs only up to the explicit group cap.
			groupN := 0
			if !d.BlockPairAdmits(chosen, ci) {
				return
			}
			for _, old := range chosen {
				if o.Group != "" && d.Options[old].Group == o.Group {
					groupN++
				}
			}
			if o.BlockMustAll && o.Group != "" && groupN >= d.GroupCapFor(o.Group) {
				return
			}
			// The combined non-mana LIFE charge bound is enforced INSIDE the
			// team search, not by pruning a finished team afterwards: a team
			// member dropped after the fact can break a Min$ team minimum (the
			// remaining members no longer form a legal declaration). One
			// option priced through the shared chargeLifeCost reader, so this
			// bound and the attack path's quota price a pip identically.
			cost := o.chargeLifeCost()
			if lifeBound >= 0 && spentLife+cost > lifeBound {
				return
			}
			if o.MaxBlockers > 0 && counts[o.Attacker] >= o.MaxBlockers {
				return
			}
			counts[o.Attacker]++
			chosen = append(chosen, ci)
			add := 0
			// Only a pair that obeys the blocker's requirement counts (CR 509.1c).
			if b.required && (o.BlockMust || o.Required) {
				add = 1
			}
			// An attacker-oriented requirement is satisfied by the FIRST
			// chosen option naming that attacker; later blockers of the same
			// attacker add nothing. Only the option that SET the flag clears
			// it on backtrack -- a second blocker of the same attacker must
			// not delete the first one's satisfaction.
			setAtk := o.AttackMust && !satAtk[o.Attacker]
			if setAtk {
				satAtk[o.Attacker] = true
				add++
			}
			// The two-blocker unit is obeyed exactly when this option is the
			// second naming its attacker; backtracking the count undoes it.
			setTwo := atkReq2[o.Attacker] && counts[o.Attacker] == 2
			if setTwo {
				satTwo++
				add++
			}
			search(at+1, satisfied+add, spent+o.Value, spentLife+cost)
			if setTwo {
				satTwo--
			}
			if setAtk {
				delete(satAtk, o.Attacker)
			}
			chosen = chosen[:len(chosen)-1]
			counts[o.Attacker]--
		}
		if b.required {
			// Prefer a legal singleton over a multi-blocker team when both
			// discharge the same requirement; this also keeps a Min$ attacker
			// available to an actual team if another required blocker needs it.
			for _, ci := range b.opts {
				if d.Options[ci].MinBlockers <= 1 {
					try(ci)
				}
			}
			for _, ci := range b.opts {
				if d.Options[ci].MinBlockers > 1 {
					try(ci)
				}
			}
			search(at+1, satisfied, spent, spentLife)
		} else {
			search(at+1, satisfied, spent, spentLife)
			for _, ci := range b.opts {
				try(ci)
			}
		}
	}
	search(0, 0, 0, 0)
	return best
}

// teamsWithin bounds how many unsatisfied required attackers free blockers can
// still serve, after first completing every team already begun, and reports false
// when they can't complete those.
func teamsWithin(attackers []state.ObjID, teamMin, counts map[state.ObjID]int, atkReq, satAtk map[state.ObjID]bool, free int, needs []int) (int, bool) {
	needs = needs[:0]
	served := 0
	for _, a := range attackers {
		c, m := counts[a], teamMin[a]
		if c > 0 && c < m {
			if free -= m - c; free < 0 {
				return 0, false
			}
		}
		if !atkReq[a] || satAtk[a] {
			continue
		}
		if c > 0 {
			served++
		} else {
			needs = append(needs, max(m, 1))
		}
	}
	slices.Sort(needs)
	for _, n := range needs {
		if free -= n; free < 0 {
			break
		}
		served++
	}
	return served, true
}

// interchangeableAttackers partitions the attackers by what the candidates offer
// against each: within a class, swapping two attackers maps every team onto one
// satisfying as many requirements with as many blockers.
func interchangeableAttackers(d *Decision, candidates []blocker) [][]state.ObjID {
	offers := make(map[state.ObjID][]byte)
	for at, b := range candidates {
		for _, ci := range b.opts {
			o := &d.Options[ci]
			sig := binary.AppendUvarint(offers[o.Attacker], uint64(at))
			sig = binary.AppendUvarint(sig, uint64(o.MinBlockers))
			sig = binary.AppendUvarint(sig, uint64(o.MaxBlockers))
			var flags uint64
			for bit, set := range [...]bool{o.AttackMust, o.BlockMust, o.Required, o.AttackMustTwo} {
				if set {
					flags |= 1 << bit
				}
			}
			offers[o.Attacker] = binary.AppendUvarint(sig, flags)
		}
	}
	var classes [][]state.ObjID
	class := make(map[string]int)
	placed := make(map[state.ObjID]bool)
	for _, o := range d.Options {
		if placed[o.Attacker] {
			continue
		}
		placed[o.Attacker] = true
		sig := string(offers[o.Attacker])
		i, ok := class[sig]
		if !ok {
			i = len(classes)
			class[sig] = i
			classes = append(classes, nil)
		}
		classes[i] = append(classes[i], o.Attacker)
	}
	return classes
}

// blockSearchMemoizable reports that what's still legal in the team search reads
// only the blockers per attacker, not which ones: no budget, life charge, shared
// group or can't-block-alone creature.
func (d *Decision) blockSearchMemoizable(lifeBound int32) bool {
	if d.HasBudget() || lifeBound >= 0 {
		return false
	}
	for i := range d.Options {
		if o := &d.Options[i]; o.BlockMustAll || o.Group != "" || o.AloneRestricted {
			return false
		}
	}
	return true
}

func (d *Decision) hasRequiredBlocks() bool {
	for _, o := range d.Options {
		if o.BlockMust || o.Required || o.AttackMust || o.AttackMustTwo {
			return true
		}
	}
	return false
}

// blockRequirementsSatisfied counts the CR 509.1c requirement units the
// chosen options satisfy: one per required BLOCKER chosen (BlockMust/Required)
// plus one per required ATTACKER that at least one chosen option blocks
// (AttackMust), plus one per two-blocker attacker that at least two chosen
// options block (AttackMustTwo). This is the ONE counter blockRequiredQuota reads on the
// solver's team and RequiredChosen reads on a submitted answer, so the engine's
// declaration check and the client's repair can never disagree about what
// "the maximum" means. Out-of-range indices are ignored.
func (d *Decision) blockRequirementsSatisfied(choices []int) int {
	// Map-free (it runs per candidate answer in search): a unit counts at the
	// first choice that satisfies it, found by scanning the choices before.
	n := 0
	for k, c := range choices {
		if c < 0 || c >= len(d.Options) {
			continue
		}
		o := &d.Options[c]
		blockUnit, allPairUnit, atkUnit := !o.BlockMustAll && (o.BlockMust || o.Required), o.BlockMustAll, o.AttackMust
		for _, p := range choices[:k] {
			if !blockUnit && !allPairUnit && !atkUnit {
				break
			}
			if p < 0 || p >= len(d.Options) {
				continue
			}
			q := &d.Options[p]
			if blockUnit && !q.BlockMustAll && (q.BlockMust || q.Required) && q.Obj == o.Obj {
				blockUnit = false
			}
			if allPairUnit && q.BlockMustAll && q.Obj == o.Obj && q.Attacker == o.Attacker {
				allPairUnit = false
			}
			if atkUnit && q.AttackMust && q.Attacker == o.Attacker {
				atkUnit = false
			}
		}
		if blockUnit || allPairUnit {
			n++
		}
		if atkUnit {
			n++
		}
		if o.AttackMustTwo && namedBefore(d, choices[:k], o.Attacker) == 1 {
			n++
		}
	}
	return n
}

// namedBefore counts the in-range choices naming attacker, the map-free read
// blockRequirementsSatisfied makes for the two-blocker unit.
func namedBefore(d *Decision, choices []int, attacker state.ObjID) int {
	n := 0
	for _, p := range choices {
		if p >= 0 && p < len(d.Options) && d.Options[p].Attacker == attacker {
			n++
		}
	}
	return n
}

func (d *Decision) blockRequiredQuota() int {
	return d.blockRequirementsSatisfied(d.blockRequiredCoreChargeFeasible())
}

// blockRequiredCoreChargeFeasible is the legal maximum team over all MustBlock
// candidates, with the combined non-mana LIFE charge (CostLife plus each
// Phyrexian pip at two life, the shared chargeLifeCost price) bounded INSIDE
// the team search. The charge bound is a whole-declaration property the
// per-pair option list cannot express, and it is solved WITH the team minima
// rather than by pruning a finished team: filtering a complete team pair by
// pair can drop a member a Min$ attacker needed and return a set that is not
// a legal declaration at all, which Submit rejects. blockRequiredCore already
// enforces the bound (and the mana budget) on every candidate, so its result
// is charge-feasible by construction; a charge-free decision (no published
// PayerLife) skips the check and is byte-identical. ONE derivation, so the
// quota and the repair cannot disagree.
func (d *Decision) blockRequiredCoreChargeFeasible() []int {
	return d.blockRequiredCore()
}

// BlockPairAdmits is the shared incremental KBlockers multi-block rule:
// two pairs may share a blocker when both are marked BlockAllDefined, or when
// neither is and the blocker's Group carries a raised cap (CanBlockAmount$ /
// CanBlockAny$, published by the rules layer through GroupLimits from
// combat.BlockCapacity). GroupCapFor remains the separate upper bound on how
// many such pairs fit.
func (d *Decision) BlockPairAdmits(chosen []int, ci int) bool {
	if d.Kind != KBlockers || ci < 0 || ci >= len(d.Options) {
		return true
	}
	o := d.Options[ci]
	for _, old := range chosen {
		if old < 0 || old >= len(d.Options) {
			continue
		}
		q := d.Options[old]
		if q.Obj == o.Obj && (!q.BlockMustAll || !o.BlockMustAll) &&
			(q.BlockMustAll || o.BlockMustAll || d.GroupCapFor(o.Group) < 2) {
			return false
		}
	}
	return true
}

func (d *Decision) blockAnswerLegal(choices []int) bool {
	if len(choices) > d.maxChoices() || !d.blockCountLegal(choices) {
		return false
	}
	for i, ci := range choices {
		if !d.Repeatable {
			for _, old := range choices[:i] {
				if old == ci {
					return false
				}
			}
		}
		if !d.BlockPairAdmits(choices[:i], ci) {
			return false
		}
	}
	sum := 0
	groups := make(map[string]int)
	for _, ci := range choices {
		o := d.Options[ci]
		if o.Group != "" && !d.GroupAdmits(groups, o.Group) {
			return false
		}
		groups[o.Group]++
		sum += o.Value
	}
	return !d.HasBudget() || sum <= d.MaxSum
}

// BlockRequiredTeam is the legal maximum team over all MustBlock candidates.
// Rules uses it when constructing the offer to highlight one feasible team.
func (d *Decision) BlockRequiredTeam() []int {
	return d.blockRequiredCoreChargeFeasible()
}

// blockCountLegal checks published per-attacker team minima and maxima on an
// already selected answer. The engine still checks its live board bounds.
func (d *Decision) blockCountLegal(choices []int) bool {
	counts := make(map[state.ObjID]int)
	for _, ci := range choices {
		if ci < 0 || ci >= len(d.Options) {
			return false
		}
		counts[d.Options[ci].Attacker]++
	}
	for _, ci := range choices {
		o := d.Options[ci]
		if counts[o.Attacker] < o.MinBlockers || (o.MaxBlockers > 0 && counts[o.Attacker] > o.MaxBlockers) {
			return false
		}
	}
	return true
}
