package rules

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestCastWindowReachableDeadStatesChangeNoAnswer: remembering the states the
// cast window's eligibility walk has exhausted must never change what it
// answers. Random sets of sources (free and fee-paying, one or two
// alternatives, one or two mana) are asked random costs both ways: with the
// memory from the first node and with none.
func TestCastWindowReachableDeadStatesChangeNoAnswer(t *testing.T) {
	t.Parallel()
	b := equipWindowGame(t, 722, cwFreeColorlessSource)
	e := b.engine
	rng := rand.New(rand.NewPCG(7, 11))
	symbols := []byte("WUBRGC")
	yes, no := 0, 0
	for trial := 0; trial < 1500; trial++ {
		n := 2 + rng.IntN(5)
		units := make([]windowManaUnit, 0, n)
		for i := 0; i < n; i++ {
			u := windowManaUnit{ID: state.ObjID(1000 + i)}
			for a := 0; a < 1+rng.IntN(2); a++ {
				var counts [6]int32
				counts[rng.IntN(6)] = 1
				alt := windowManaAlt{Counts: counts, Amt: int32(1 + rng.IntN(2))}
				if rng.IntN(3) == 0 {
					alt.CostGeneric = int32(1 + rng.IntN(2))
				}
				u.Alts = append(u.Alts, alt)
			}
			units = append(units, u)
		}
		cost := ""
		for i, k := 0, 1+rng.IntN(6); i < k; i++ {
			if i > 0 {
				cost += " "
			}
			if rng.IntN(3) == 0 {
				cost += "1"
			} else {
				cost += string(symbols[rng.IntN(6)])
			}
		}
		var pool state.Mana
		pool[rng.IntN(6)] = int32(rng.IntN(2))
		with := e.castWindowReachableAfter(0, 0, ParseCost(cost), pool, state.Mana{}, [7]state.Mana{}, 20, nil, pipRider{}, units)
		without := e.castWindowReachableAfter(-1, 0, ParseCost(cost), pool, state.Mana{}, [7]state.Mana{}, 20, nil, pipRider{}, units)
		if with != without {
			t.Fatalf("trial %d cost %q: the optimised walk answers %v, the plain walk %v; units %+v", trial, cost, with, without, units)
		}
		if with {
			yes++
		} else {
			no++
		}
	}
	if yes < 40 || no < 40 {
		t.Fatalf("the trials must cover both answers: %d reachable, %d not", yes, no)
	}
}
