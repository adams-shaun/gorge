package event

import (
	"fmt"
	"math/rand/v2"
	"sort"
)

// swissSalt tags the per-round tie-break stream: each round r's pairing
// draws its permutation from PCG(seed, swissSalt<<32|r), so two events with
// the same seed tie-break the same way and one event's rounds differ.
const swissSalt = 0x5A1550

// SwissPairings pairs n seats for the given round from the completed series
// results (any order). Seats are ordered by match points (3 per series won,
// 1 per drawn series) descending, then by a seeded per-round tie-break key;
// the first seat in order is paired with the first later seat it has not
// met, backtracking until every seat is paired. When no rematch-free
// pairing exists, adjacent seats in order are paired. Pairs are lower seat
// first, in table order (the top bracket first). Pure: the same inputs give
// the same pairs.
func SwissPairings(n, round int, results []SeriesResult, seed uint64) ([][2]int, error) {
	if n < 2 {
		return nil, fmt.Errorf("event: SwissPairings: %d seats, want at least 2", n)
	}
	if n%2 != 0 {
		return nil, fmt.Errorf("event: SwissPairings: %d seats, want an even count", n)
	}
	if round < 1 {
		return nil, fmt.Errorf("event: SwissPairings: round %d < 1", round)
	}
	points := make([]int, n)
	met := make([][]bool, n)
	for i := range met {
		met[i] = make([]bool, n)
	}
	for _, r := range results {
		if r.A < 0 || r.A >= n || r.B < 0 || r.B >= n || r.A == r.B {
			return nil, fmt.Errorf("event: SwissPairings: result names seats %d–%d, want two distinct seats in 0..%d", r.A, r.B, n-1)
		}
		switch {
		case r.Score[0] > r.Score[1]:
			points[r.A] += 3
		case r.Score[1] > r.Score[0]:
			points[r.B] += 3
		default:
			points[r.A]++
			points[r.B]++
		}
		met[r.A][r.B] = true
		met[r.B][r.A] = true
	}
	rng := rand.New(rand.NewPCG(seed, swissSalt<<32|uint64(round)))
	key := rng.Perm(n)
	order := make([]int, n)
	for s := range order {
		order[s] = s
	}
	sort.SliceStable(order, func(i, j int) bool {
		if points[order[i]] != points[order[j]] {
			return points[order[i]] > points[order[j]]
		}
		return key[order[i]] < key[order[j]]
	})
	pairs := make([][2]int, 0, n/2)
	paired := make([]bool, n)
	var search func(u int) bool
	search = func(u int) bool {
		for u < n && paired[order[u]] {
			u++
		}
		if u == n {
			return true
		}
		uu := order[u]
		for v := u + 1; v < n; v++ {
			vv := order[v]
			if paired[vv] || met[uu][vv] {
				continue
			}
			paired[uu], paired[vv] = true, true
			pairs = append(pairs, [2]int{uu, vv})
			if search(u + 1) {
				return true
			}
			pairs = pairs[:len(pairs)-1]
			paired[uu], paired[vv] = false, false
		}
		return false
	}
	if !search(0) {
		pairs = pairs[:0]
		for i := 0; i < n; i += 2 {
			pairs = append(pairs, [2]int{order[i], order[i+1]})
		}
	}
	for i := range pairs {
		pairs[i] = normPair(pairs[i][0], pairs[i][1])
	}
	return pairs, nil
}

// normPair orders a pair lower seat first.
func normPair(a, b int) [2]int {
	if a > b {
		a, b = b, a
	}
	return [2]int{a, b}
}
