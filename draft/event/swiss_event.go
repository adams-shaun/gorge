package event

import "fmt"

// swissDefaultRounds is a Swiss event's default round count when the
// EventCreated entry names none: ceil(log2 n) — 2 for 4 seats, 3 for 8.
func swissDefaultRounds(n int) int {
	for r := 0; ; r++ {
		if 1<<uint(r) >= n {
			return r
		}
	}
}

// AwaitingPairing reports the Swiss round a RoundPaired entry must open
// next: (1, true) after a Swiss event is built and every seat is ready,
// (r+1, true) once round r's every series has ended and r is not the last
// round. It reports false outside a Swiss event and whenever nothing waits
// for a pairing — a live round, the aftermath of the last round (which
// awaits the EventEnded), or a round-robin event.
func (e *Event) AwaitingPairing() (round int, ok bool) {
	if e.structure != StructureSwiss {
		return 0, false
	}
	if e.phase == PhaseBuild && e.allReady() {
		return 1, true
	}
	if e.inRound() && e.round < e.rounds && e.roundSeriesEnded() {
		return e.round + 1, true
	}
	return 0, false
}

// roundSeriesEnded reports whether every pairing of the current round has
// an ended series.
func (e *Event) roundSeriesEnded() bool {
	for _, p := range e.pairings {
		st := e.series[p]
		if st == nil || !st.ended {
			return false
		}
	}
	return len(e.pairings) > 0
}

// SuggestedPairings is SwissPairings(n, round, e.SeriesResults(), e.Seed())
// for the round AwaitingPairing names: the pairs a driver normally logs as
// its RoundPaired entry. An error when nothing is awaited (the driver must
// not pair) or the pure function rejects the shape.
func (e *Event) SuggestedPairings() ([][2]int, error) {
	r, ok := e.AwaitingPairing()
	if !ok {
		return nil, fmt.Errorf("event: SuggestedPairings: no Swiss round awaits its pairing (phase %s)", e.phase)
	}
	return SwissPairings(e.NumSeats(), r, e.SeriesResults(), e.Seed())
}

// applyRoundPaired opens round r of a Swiss event with the pairs the driver
// chose. It validates the pairs' SHAPE only — event created, structure
// swiss, the round is the one AwaitingPairing names, Pairs is a perfect
// matching of 0..n-1 in lower-seat-first order — and never compares with
// SuggestedPairings: pairings are chosen by the driver and logged so the
// log keeps its meaning after a later change to the pairing algorithm (the
// same reason packs are logged in full). A round-robin or none event
// refuses RoundPaired.
func (e *Event) applyRoundPaired(en *Entry) error {
	if !e.cooked {
		return fmt.Errorf("event: RoundPaired: event not created")
	}
	if e.structure != StructureSwiss {
		return fmt.Errorf("event: RoundPaired: structure %s never takes a RoundPaired entry", e.structure)
	}
	r, ok := e.AwaitingPairing()
	if !ok {
		return fmt.Errorf("event: RoundPaired: illegal in phase %s", e.phase)
	}
	if en.Round != r {
		return fmt.Errorf("event: RoundPaired: round %d, want the awaited round %d", en.Round, r)
	}
	n := e.NumSeats()
	if len(en.Pairs) != n/2 {
		return fmt.Errorf("event: RoundPaired: %d pairs, want %d (one per two of %d seats)", len(en.Pairs), n/2, n)
	}
	seen := make([]bool, n)
	for _, p := range en.Pairs {
		if p[0] < 0 || p[0] >= n || p[1] < 0 || p[1] >= n {
			return fmt.Errorf("event: RoundPaired: pair %d–%d names a seat outside 0..%d", p[0], p[1], n-1)
		}
		if p[0] >= p[1] {
			return fmt.Errorf("event: RoundPaired: pair %d–%d is not lower seat first", p[0], p[1])
		}
		if seen[p[0]] || seen[p[1]] {
			return fmt.Errorf("event: RoundPaired: pair %d–%d repeats a seat (not a perfect matching)", p[0], p[1])
		}
		seen[p[0]], seen[p[1]] = true, true
	}
	e.round = r
	e.pairings = append([][2]int(nil), en.Pairs...)
	e.series = map[[2]int]*seriesState{}
	e.phase = RoundPhase(r)
	return nil
}
