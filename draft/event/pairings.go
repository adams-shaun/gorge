package event

import "github.com/adams-shaun/gorge/draft/limited/draft"

// Pairings returns event round r's pairs for the hosted 4-seat schedule,
// the circle-method table eventhost drives: seat 0 fixed, each pair ordered
// lower seat first — R1 0–3, 1–2; R2 0–2, 1–3; R3 0–1, 2–3.
func Pairings(round int) [][2]int {
	return RoundRobinPairings(draft.Seats, round)
}

// RoundRobinPairings pairs n seats (even) for round r (1-based) with the
// circle method: seat 0 is fixed and the other n-1 seats rotate one place
// right each round; position i is paired with position n-1-i. It returns
// nil for an odd n or a round outside 1..n-1.
func RoundRobinPairings(n, round int) [][2]int {
	if n%2 != 0 || round < 1 || round >= n {
		return nil
	}
	tail := make([]int, n-1)
	for i := range tail {
		tail[i] = i + 1
	}
	shift := (round - 1) % (n - 1)
	rot := make([]int, n-1)
	for i := range rot {
		rot[(i+shift)%(n-1)] = tail[i]
	}
	out := make([][2]int, 0, n/2)
	for i := 0; i < n/2; i++ {
		a, b := 0, 0
		if i == 0 {
			b = rot[n-2]
		} else {
			a, b = rot[i-1], rot[n-2-i]
		}
		if a > b {
			a, b = b, a
		}
		out = append(out, [2]int{a, b})
	}
	return out
}
