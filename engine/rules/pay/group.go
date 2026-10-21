package pay

import (
	"slices"

	costvocab "github.com/adams-shaun/gorge/engine/rules/cost"
	"github.com/adams-shaun/gorge/engine/state"
)

// GroupValue is one object's contribution to an AtLeast exile part's
// set-level sum (costvocab.GroupTotal): its mana value for withTotalCMC, or
// the number of mana symbols of the group's colour in its mana cost for
// withTotalManaSymbols (CR 107.4e/f: a hybrid or Phyrexian symbol of that
// colour is one symbol of it). A face-less object contributes 0.
func GroupValue(o *state.Object, g costvocab.GroupTotal) int32 {
	if o == nil || o.Face() == nil {
		return 0
	}
	switch g.Prop {
	case costvocab.GroupCMC:
		return state.ManaValue(nil, o)
	case costvocab.GroupManaSymbols:
		return costvocab.ManaSymbolsOfColor(costvocab.ParseCost(o.Face().ManaCost), g.Color)
	}
	return 0
}

// GroupValues is GroupValue over a candidate list, in list order.
func GroupValues(g *state.Game, ids []state.ObjID, group costvocab.GroupTotal) []int32 {
	out := make([]int32, len(ids))
	for i, id := range ids {
		out[i] = GroupValue(g.Obj(id), group)
	}
	return out
}

// GroupSums returns, ascending, every total some subset of values with at
// least min members reaches -- the announcements a withTotal<Prop>EQX group
// can settle (CR 601.2b: an announced X must be one the payment can pay).
// Values are non-negative, so the totals are bounded by their sum; the walk
// is a subset-sum table over (members capped at min, total).
func GroupSums(values []int32, min int) []int32 {
	if min < 0 {
		min = 0
	}
	total := int32(0)
	for _, v := range values {
		if v > 0 {
			total += v
		}
	}
	width := int(total) + 1
	// reach[k*width+s]: some subset of exactly k members (k == min means
	// "min or more") totals s.
	reach := make([]bool, (min+1)*width)
	reach[0] = true
	for _, v := range values {
		if v < 0 {
			v = 0
		}
		for k := min; k >= 0; k-- {
			for s := width - 1 - int(v); s >= 0; s-- {
				if !reach[k*width+s] {
					continue
				}
				nk := k + 1
				if nk > min {
					nk = min
				}
				reach[nk*width+s+int(v)] = true
			}
		}
	}
	var out []int32
	for s := 0; s < width; s++ {
		if reach[min*width+s] {
			out = append(out, int32(s))
		}
	}
	return out
}

// GroupFeasible reports whether some subset of values with at least min
// members satisfies group against the resolved right-hand side rhs. An
// inactive group needs only min members.
func GroupFeasible(values []int32, min int, group costvocab.GroupTotal, rhs int32) bool {
	if len(values) < min {
		return false
	}
	switch group.Cmp {
	case costvocab.GroupGE:
		// Values are non-negative: taking every candidate maximises the sum.
		total := int32(0)
		for _, v := range values {
			if v > 0 {
				total = costvocab.AddClampedGeneric(total, int64(v))
			}
		}
		return total >= rhs
	case costvocab.GroupEQ:
		return slices.Contains(GroupSums(values, min), rhs)
	}
	return !group.Active()
}
