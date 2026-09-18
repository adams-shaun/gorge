// Package searchprobe contains the experimental history-conditioned search
// harness. It is not used by production seats or hosts.
package searchprobe

import (
	"fmt"
	"math"

	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/state"
)

type placement struct {
	Index int
	Card  state.ObjID
}
type boundedRandom interface{ IntN(int) int }

func samplePermutation(cards []state.ObjID, fixed []placement, r boundedRandom) ([]state.ObjID, float64, error) {
	n := len(cards)
	pool := make(map[state.ObjID]bool, n)
	for _, id := range cards {
		if id == 0 || pool[id] {
			return nil, 0, fmt.Errorf("invalid or duplicate physical card %d", id)
		}
		pool[id] = true
	}
	out := make([]state.ObjID, n)
	used := make(map[state.ObjID]bool, len(fixed))
	for _, p := range fixed {
		if p.Index < 0 || p.Index >= n || out[p.Index] != 0 || !pool[p.Card] || used[p.Card] {
			return nil, 0, fmt.Errorf("contradictory placement %+v", p)
		}
		out[p.Index] = p.Card
		used[p.Card] = true
	}
	remaining := make([]state.ObjID, 0, n-len(fixed))
	for _, id := range cards {
		if !used[id] {
			remaining = append(remaining, id)
		}
	}
	if len(remaining) > 1 && r == nil {
		return nil, 0, fmt.Errorf("missing permutation randomness")
	}
	for i := len(remaining) - 1; i > 0; i-- {
		j := r.IntN(i + 1)
		remaining[i], remaining[j] = remaining[j], remaining[i]
	}
	j := 0
	for i := range out {
		if out[i] == 0 {
			out[i] = remaining[j]
			j++
		}
	}
	// Each fixed physical position contributes 1/(n-i). This log product is
	// numerically safer than subtracting nearly equal log-factorials.
	logWeight := 0.0
	for i := range fixed {
		logWeight -= math.Log(float64(n - i))
	}
	return out, logWeight, nil
}
func permutationTape(before, after []state.ObjID) ([]rules.ChanceDraw, error) {
	if len(before) != len(after) {
		return nil, fmt.Errorf("permutation lengths differ")
	}
	current := append([]state.ObjID(nil), before...)
	seen := make(map[state.ObjID]bool, len(before))
	for _, id := range before {
		if id == 0 || seen[id] {
			return nil, fmt.Errorf("invalid or duplicate physical card %d", id)
		}
		seen[id] = true
	}
	for _, id := range after {
		if !seen[id] {
			return nil, fmt.Errorf("target is not a permutation")
		}
		delete(seen, id)
	}
	var tape []rules.ChanceDraw
	for i := len(current) - 1; i > 0; i-- {
		j := 0
		for current[j] != after[i] {
			j++
		}
		tape = append(tape, rules.ChanceDraw{Bound: i + 1, Value: j})
		current[i], current[j] = current[j], current[i]
	}
	return tape, nil
}
