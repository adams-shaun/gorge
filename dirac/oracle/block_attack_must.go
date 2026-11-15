package oracle

import (
	"slices"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// blockAttackMustReachable reports whether a blockers answer begun as chosen
// can still obey as many CR 509.1c requirements as gorge's validateBlockers
// demands (decision.RequiredQuota) when every requirement is attacker-oriented
// ("CARDNAME must be blocked if able.", Option.AttackMust: Joraga Invocation):
// one unit per such attacker with at least one blocker. The most units a
// completion can add is a maximum matching of the blockers not yet declared to
// the must-be-blocked attackers not yet blocked, over the valid AttackMust
// pairs (CR 509.1a: one blocker blocks as many attackers as its group cap
// allows, one unless CanBlockAmount$ raised it, so a declared blocker with
// pairs to spare is free for that many more), capped by the picks Max
// leaves. It is a necessary condition, exact where the budgeted completion
// search turns optimistic on a wide board: a second blocker on an attacker
// already blocked, taken from a creature that could have met another
// requirement, falls short of the maximum (the kernel's mbReach is exact
// there). With a blocker-oriented requirement (BlockMust, Required,
// BlockMustAll) in the decision it answers true and leaves it to the search.
func blockAttackMustReachable(fr *frame, chosen []int) bool {
	d := fr.d
	if d.Kind != decision.KBlockers {
		return true
	}
	mb := false
	for i := range d.Options {
		o := &d.Options[i]
		if o.BlockMust || o.Required || o.BlockMustAll {
			return true
		}
		mb = mb || o.AttackMust
	}
	if !mb {
		return true
	}
	quota := d.RequiredQuota()
	have := d.RequiredChosen(chosen)
	if have >= quota {
		return true
	}
	var used, blocked []state.ObjID // used: one entry per chosen pair
	for _, c := range chosen {
		o := optByIndex(d, c)
		used = append(used, o.Obj)
		if o.AttackMust {
			blocked = append(blocked, o.Attacker)
		}
	}
	// The free blocker slots (a blocker's pairs to spare: its group cap less
	// its chosen pairs) and the open must-be-blocked attackers, in option
	// order, and the pairs between them.
	var free, open []state.ObjID
	slots := map[state.ObjID][]int{} // blocker -> its slots in free
	type edge struct{ b, a int }
	var edges []edge
	for i := range d.Options {
		o := &d.Options[i]
		if !fr.valid[i] || !o.AttackMust || slices.Contains(blocked, o.Attacker) {
			continue
		}
		if _, ok := slots[o.Obj]; !ok {
			spare := 1
			if o.Group != "" {
				spare = d.GroupCapFor(o.Group)
			}
			for _, u := range used {
				if u == o.Obj {
					spare--
				}
			}
			slots[o.Obj] = nil
			for ; spare > 0; spare-- {
				slots[o.Obj] = append(slots[o.Obj], len(free))
				free = append(free, o.Obj)
			}
		}
		a := slices.Index(open, o.Attacker)
		if a < 0 {
			a = len(open)
			open = append(open, o.Attacker)
		}
		for _, k := range slots[o.Obj] {
			edges = append(edges, edge{k, a})
		}
	}
	match := make([]int, len(open)) // open attacker -> free blocker, -1 none
	for i := range match {
		match[i] = -1
	}
	var augment func(b int, seen []bool) bool
	augment = func(b int, seen []bool) bool {
		for _, e := range edges {
			if e.b != b || seen[e.a] {
				continue
			}
			seen[e.a] = true
			if match[e.a] < 0 || augment(match[e.a], seen) {
				match[e.a] = b
				return true
			}
		}
		return false
	}
	gain := 0
	for b := range free {
		if augment(b, make([]bool, len(open))) {
			gain++
		}
	}
	if left := d.Max - len(chosen); gain > left {
		gain = left
	}
	return have+gain >= quota
}
