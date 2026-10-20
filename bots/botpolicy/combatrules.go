package botpolicy

import (
	"sort"

	"github.com/adams-shaun/gorge/engine/state"
)

// CombatRules selects the opt-in A7 combat rules. Zero value: every rule off,
// and the attack/block search is byte-identical to the shipped one.
type CombatRules struct {
	// GangBlocks makes the block search also try the cheapest team of two or
	// more still-unassigned blockers that kills an attacker the incumbent
	// left unblocked (the one-blocker-at-a-time climb cannot reach a team
	// whose halves are each a losing chump).
	GangBlocks bool
	// GangPredict makes the sim's predicted reply also double-block each
	// attacker the default rule left unblocked when a team of untapped
	// creatures kills it for less value than it is worth.
	GangPredict bool
	// DoubleStrike models Double Strike in a blocked combat inside the sim
	// (blockCombatDS), not only when unblocked.
	DoubleStrike bool
}

// gangMinTeam: a lone kill is already one climb step (GangBlocks) and is
// BR1's call (GangPredict).
const gangMinTeam = 2

// gangValueSlack: GangPredict takes a team iff lost value + slack < the
// attacker's value. Zero: the evaluator already prices trades.
const gangValueSlack int64 = 0

// gangSeed returns base plus, for each attacker base leaves unblocked, the
// cheapest killing team of two or more still-unassigned blockers. Attackers
// are visited by power desc then id asc; each one with a team gets it, the
// next drawing on what is left. changed is false when none has such a team.
func (b Board) gangSeed(base []int, blkOrder []state.ObjID, choices [][]state.ObjID, atkOrder []state.ObjID) ([]int, bool) {
	seed := append([]int(nil), base...)
	changed := false
	blocked := make(map[state.ObjID]bool, len(atkOrder))
	for k, ai := range seed {
		if ai >= 0 {
			blocked[choices[k][ai]] = true
		}
	}
	order := append([]state.ObjID(nil), atkOrder...)
	sort.SliceStable(order, func(i, j int) bool {
		pi, pj := b.Creatures.Get(order[i]).Power, b.Creatures.Get(order[j]).Power
		if pi != pj {
			return pi > pj
		}
		return order[i] < order[j]
	})
	for _, aid := range order {
		a := b.Creatures.Get(aid)
		if blocked[aid] || a.Power <= 0 || a.remTough() <= 0 {
			continue
		}
		var cands []blkCand
		for k, id := range blkOrder {
			if seed[k] != -1 {
				continue
			}
			for _, ca := range choices[k] {
				if ca == aid {
					cands = append(cands, blkCand{pos: k, id: id, c: b.Creatures.Get(id)})
					break
				}
			}
		}
		team := bestKillingBlock(a, cands)
		if len(team) < gangMinTeam {
			continue
		}
		for _, m := range team {
			for ai, ca := range choices[m.pos] {
				if ca == aid {
					seed[m.pos] = ai
				}
			}
		}
		changed = true
	}
	return seed, changed
}

// gangReinforce adds to the predicted reply out a team block on each attacker
// the default rule left unblocked: the cheapest team of two or more untapped,
// not-yet-used defender creatures that kills it, taken iff the team's lost
// value (plus gangValueSlack) is below the attacker's value. It never removes
// or changes a predicted block.
func (w *simWorld) gangReinforce(defender state.PlayerID, attackers []state.ObjID, out map[state.ObjID][]state.ObjID) {
	used := make(map[state.ObjID]bool)
	for _, bl := range out {
		for _, id := range bl {
			used[id] = true
		}
	}
	order := append([]state.ObjID(nil), attackers...)
	sort.SliceStable(order, func(i, j int) bool {
		pi, pj := w.units[w.index(order[i])].c.Power, w.units[w.index(order[j])].c.Power
		if pi != pj {
			return pi > pj
		}
		return order[i] < order[j]
	})
	for _, aid := range order {
		ai := w.index(aid)
		a := w.units[ai].c
		if len(out[aid]) > 0 || a.Power <= 0 || a.remTough() <= 0 {
			continue
		}
		var cands []blkCand
		for i := range w.units {
			u := &w.units[i]
			if u.dead || u.c.Controller != defender || u.c.Tapped || used[u.id] || !simCanBlock(a, u.c) {
				continue
			}
			cands = append(cands, blkCand{pos: i, id: u.id, c: u.c})
		}
		team := bestKillingBlock(a, cands)
		if len(team) < gangMinTeam {
			continue
		}
		creatures := make([]Creature, len(team))
		for i, m := range team {
			creatures[i] = m.c
		}
		_, dead := blockCombat(a, creatures)
		var lost int64
		for i, m := range team {
			if dead[i] {
				lost += w.units[m.pos].value
			}
		}
		if lost+gangValueSlack >= w.units[ai].value {
			continue
		}
		for _, m := range team {
			out[aid] = append(out[aid], m.id)
			used[m.id] = true
		}
	}
}
