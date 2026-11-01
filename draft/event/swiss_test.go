package event_test

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
)

// testSwissSalt mirrors event's unexported swissSalt: the round-1 pin
// below recomputes the seeded permutation with it, so a silent salt change
// fails the test.
const testSwissSalt = 0x5A1550

// swissRoundOneOrder is the round-1 seat order SwissPairings must produce
// for a seed: every seat has 0 points, so the order is the seeded
// permutation's ranks ascending.
func swissRoundOneOrder(seed uint64, n int) []int {
	rng := rand.New(rand.NewPCG(seed, testSwissSalt<<32|uint64(1)))
	key := rng.Perm(n)
	order := make([]int, n)
	for s := range order {
		order[s] = s
	}
	sort.SliceStable(order, func(i, j int) bool { return key[order[i]] < key[order[j]] })
	return order
}

// TestSwissRoundOneIsSeededPermutation pins round 1 at 8 seats with no
// results: the pairs are the seeded order's adjacent seats, the same seed
// gives the same pairs twice, and seeds 1 and 2 differ.
func TestSwissRoundOneIsSeededPermutation(t *testing.T) {
	order := swissRoundOneOrder(7, 8)
	want := make([][2]int, 0, 4)
	for i := 0; i < 8; i += 2 {
		a, b := order[i], order[i+1]
		if a > b {
			a, b = b, a
		}
		want = append(want, [2]int{a, b})
	}
	got, err := event.SwissPairings(8, 1, nil, 7)
	if err != nil {
		t.Fatalf("SwissPairings: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("round 1 has %d pairs, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("round 1 pair %d is %v, want %v (order %v)", i, got[i], want[i], order)
		}
	}
	again, err := event.SwissPairings(8, 1, nil, 7)
	if err != nil {
		t.Fatalf("SwissPairings again: %v", err)
	}
	for i := range got {
		if got[i] != again[i] {
			t.Errorf("seed 7 is not pure: pair %d is %v then %v", i, got[i], again[i])
		}
	}
	one, err := event.SwissPairings(8, 1, nil, 1)
	if err != nil {
		t.Fatalf("SwissPairings seed 1: %v", err)
	}
	two, err := event.SwissPairings(8, 1, nil, 2)
	if err != nil {
		t.Fatalf("SwissPairings seed 2: %v", err)
	}
	same := true
	for i := range one {
		if one[i] != two[i] {
			same = false
			break
		}
	}
	if same {
		t.Errorf("seeds 1 and 2 produced identical round-1 pairs %v", one)
	}
}

// TestSwissPairsByPointsAndAvoidsRematches drives three 8-seat rounds
// through SwissPairings with synthetic results (the lower seat of every
// pair always wins): every round is a perfect matching, no pair repeats,
// and round 2 pairs only seats of equal points.
func TestSwissPairsByPointsAndAvoidsRematches(t *testing.T) {
	seed := uint64(11)
	var results []event.SeriesResult
	met := map[[2]int]bool{}
	for round := 1; round <= 3; round++ {
		// points holds the pre-round match points, for the round-2 pin.
		points := map[int]int{}
		for _, r := range results {
			if r.Score[0] > r.Score[1] {
				points[r.A] += 3
			} else {
				points[r.B] += 3
			}
		}
		pairs, err := event.SwissPairings(8, round, results, seed)
		if err != nil {
			t.Fatalf("round %d: SwissPairings: %v", round, err)
		}
		if len(pairs) != 4 {
			t.Fatalf("round %d has %d pairs, want 4", round, len(pairs))
		}
		covered := map[int]bool{}
		for _, p := range pairs {
			if p[0] >= p[1] {
				t.Errorf("round %d pair %v is not lower seat first", round, p)
			}
			if covered[p[0]] || covered[p[1]] {
				t.Errorf("round %d is not a perfect matching: %v", round, pairs)
			}
			covered[p[0]], covered[p[1]] = true, true
			if met[p] {
				t.Errorf("round %d repeats pair %v", round, p)
			}
			met[p] = true
			if round == 2 && points[p[0]] != points[p[1]] {
				t.Errorf("round 2 pair %v joins seats with %d and %d points, want equal", p, points[p[0]], points[p[1]])
			}
			results = append(results, event.SeriesResult{Round: round, A: p[0], B: p[1], Score: [2]int{1, 0}})
		}
		if len(covered) != 8 {
			t.Fatalf("round %d covers %d seats, want 8", round, len(covered))
		}
	}
}

// TestSwissFallsBackToRematchWhenForced pairs 4 seats over a full round
// robin (all six pairs played, the lower seat always winning): round 4 has
// no rematch-free pairing left and still returns a perfect matching, the
// adjacent seats of the points order (0-1, 2-3).
func TestSwissFallsBackToRematchWhenForced(t *testing.T) {
	var results []event.SeriesResult
	for r := 1; r <= 3; r++ {
		pairs := event.RoundRobinPairings(4, r)
		if len(pairs) != 2 {
			t.Fatalf("round-robin round %d has %d pairs, want 2", r, len(pairs))
		}
		for _, p := range pairs {
			results = append(results, event.SeriesResult{Round: r, A: p[0], B: p[1], Score: [2]int{1, 0}})
		}
	}
	if len(results) != 6 {
		t.Fatalf("setup produced %d results, want all 6 pairs", len(results))
	}
	got, err := event.SwissPairings(4, 4, results, 3)
	if err != nil {
		t.Fatalf("SwissPairings: %v", err)
	}
	want := [][2]int{{0, 1}, {2, 3}}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("round 4 pair %d is %v, want %v", i, got[i], want[i])
		}
	}
}

// TestSwissRejectsBadInput names the pure function's four input refusals.
func TestSwissRejectsBadInput(t *testing.T) {
	badSeat := []event.SeriesResult{{Round: 1, A: 0, B: 9, Score: [2]int{1, 0}}}
	selfPair := []event.SeriesResult{{Round: 1, A: 3, B: 3, Score: [2]int{1, 0}}}
	cases := []struct {
		name    string
		n, rnd  int
		results []event.SeriesResult
	}{
		{"5 seats", 5, 1, nil},
		{"0 seats", 0, 1, nil},
		{"round 0", 8, 0, nil},
		{"seat 9", 8, 1, badSeat},
		{"A == B", 8, 1, selfPair},
	}
	for _, tc := range cases {
		if _, err := event.SwissPairings(tc.n, tc.rnd, tc.results, 1); err == nil {
			t.Errorf("%s: SwissPairings accepted bad input", tc.name)
		}
	}
}

// TestSwissEventLifecycleEightSeats drives an 8-seat swiss draft event
// through all 3 default rounds: after readying the event stays in BUILD
// awaiting round 1's pairing, the driver logs a RoundPaired per round from
// SuggestedPairings and plays every pending series (lower seat wins), and
// the event reaches COMPLETE after 12 series with a log that replays
// byte-identically.
func TestSwissEventLifecycleEightSeats(t *testing.T) {
	e := newEightSeatDraft(t, event.Entry{Structure: "swiss"})
	if e.Rounds() != 3 {
		t.Fatalf("rounds is %d, want 3 (ceil(log2 8))", e.Rounds())
	}
	saveAndReadyAll(t, e)
	if e.Phase() != event.PhaseBuild {
		t.Fatalf("phase after readying is %s, want BUILD (awaiting pairing)", e.Phase())
	}
	if r, ok := e.AwaitingPairing(); !ok || r != 1 {
		t.Fatalf("AwaitingPairing after readying is (%d, %v), want (1, true)", r, ok)
	}
	if ps := e.PendingSeries(); len(ps) != 0 {
		t.Fatalf("PendingSeries before round 1 lists %d series, want none", len(ps))
	}
	for round := 1; round <= e.Rounds(); round++ {
		if r, ok := e.AwaitingPairing(); !ok || r != round {
			t.Fatalf("round %d: AwaitingPairing is (%d, %v), want (%d, true)", round, r, ok, round)
		}
		pairs, err := e.SuggestedPairings()
		if err != nil {
			t.Fatalf("round %d: SuggestedPairings: %v", round, err)
		}
		mustApply(t, e, event.Entry{Kind: event.KindRoundPaired, Round: round, Pairs: pairs})
		if e.Phase() != event.RoundPhase(round) {
			t.Fatalf("phase after round %d pairing is %s, want ROUND%d", round, e.Phase(), round)
		}
		got := e.CurrentPairings()
		if len(got) != len(pairs) {
			t.Fatalf("round %d pairings are %v, want %v", round, got, pairs)
		}
		for i := range pairs {
			if got[i] != pairs[i] {
				t.Errorf("round %d pairing %d is %v, want %v", round, i, got[i], pairs[i])
			}
		}
		for _, p := range e.PendingSeries() {
			if p.Round != round {
				t.Fatalf("pending series %+v is not round %d", p, round)
			}
			playBo1(t, e, p.Round, p.A, p.B, p.A)
		}
	}
	if _, ok := e.AwaitingPairing(); ok {
		t.Fatalf("AwaitingPairing after the last round, want false (awaiting EventEnded)")
	}
	mustApply(t, e, event.Entry{Kind: event.KindEventEnded})
	if !e.Done() {
		t.Fatalf("event ended in phase %s, want COMPLETE", e.Phase())
	}
	series := 0
	for _, en := range e.Log() {
		if en.Kind == event.KindSeriesEnded {
			series++
		}
	}
	if series != 12 {
		t.Fatalf("the log holds %d ended series, want 12 (3 rounds × 4)", series)
	}
	st := e.Standings()
	if len(st) != 8 {
		t.Fatalf("standings has %d rows, want 8", len(st))
	}
	if st[0].Seat != 0 || st[0].Points != 9 {
		t.Fatalf("standings head is %+v, want seat 0 with 9 points (3 wins)", st[0])
	}
	replayMatches(t, e)
}

// TestRoundPairedValidation checks the entry's shape refusals: wrong round,
// a seat twice, a>b, too few pairs, and a RoundPaired in a round-robin
// event. Every refusal changes nothing. A valid but non-suggested matching
// is accepted (the shape-only rule).
func TestRoundPairedValidation(t *testing.T) {
	e := newEightSeatDraft(t, event.Entry{Structure: "swiss"})
	saveAndReadyAll(t, e)
	base := len(e.Log())
	suggested, err := e.SuggestedPairings()
	if err != nil {
		t.Fatalf("SuggestedPairings: %v", err)
	}
	if _, ok := e.AwaitingPairing(); !ok {
		t.Fatal("precondition: the event is not awaiting round 1's pairing")
	}
	twice := [][2]int{{0, 1}, {0, 2}, {3, 4}, {5, 6}}
	reversed := [][2]int{{1, 0}, {2, 3}, {4, 5}, {6, 7}}
	three := [][2]int{{0, 1}, {2, 3}, {4, 5}}
	bad := []struct {
		name string
		en   event.Entry
	}{
		{"wrong round", event.Entry{Kind: event.KindRoundPaired, Round: 2, Pairs: suggested}},
		{"seat twice", event.Entry{Kind: event.KindRoundPaired, Round: 1, Pairs: twice}},
		{"a > b", event.Entry{Kind: event.KindRoundPaired, Round: 1, Pairs: reversed}},
		{"3 pairs at 8 seats", event.Entry{Kind: event.KindRoundPaired, Round: 1, Pairs: three}},
		{"nil pairs", event.Entry{Kind: event.KindRoundPaired, Round: 1}},
	}
	for _, tc := range bad {
		if err := e.Apply(tc.en); err == nil {
			t.Errorf("%s: RoundPaired accepted", tc.name)
		}
		if len(e.Log()) != base {
			t.Errorf("%s: refusal logged an entry", tc.name)
		}
	}

	// A valid, non-suggested perfect matching: the suggested pairs' first
	// members crossed with the third and fourth's, so no pair of it is a
	// suggested pair. The shape-only rule accepts it.
	first := []int{}
	second := []int{}
	for _, p := range suggested {
		first = append(first, p[0])
		second = append(second, p[1])
	}
	alt := [][2]int{
		norm(first[0], first[2]), norm(first[1], first[3]),
		norm(second[0], second[2]), norm(second[1], second[3]),
	}
	for _, p := range alt {
		for _, q := range suggested {
			if p == q {
				t.Fatalf("alternative matching %v contains suggested pair %v", alt, q)
			}
		}
	}
	mustApply(t, e, event.Entry{Kind: event.KindRoundPaired, Round: 1, Pairs: alt})
	if e.Phase() != event.RoundPhase(1) {
		t.Fatalf("phase after the accepted pairing is %s, want ROUND1", e.Phase())
	}
	if got := e.CurrentPairings(); len(got) != len(alt) {
		t.Fatalf("pairings hold %d pairs, want %d", len(got), len(alt))
	}

	// A round-robin event never takes a RoundPaired.
	rr := newEightSeatDraft(t, event.Entry{})
	saveAndReadyAll(t, rr)
	baseRR := len(rr.Log())
	if err := rr.Apply(event.Entry{Kind: event.KindRoundPaired, Round: 1, Pairs: [][2]int{{0, 3}, {1, 2}, {4, 5}, {6, 7}}}); err == nil {
		t.Errorf("RoundPaired in a round-robin event accepted")
	}
	if len(rr.Log()) != baseRR {
		t.Errorf("round-robin refusal logged an entry")
	}
}

// norm orders a pair lower seat first (the test-side copy of the shape rule).
func norm(a, b int) [2]int {
	if a > b {
		a, b = b, a
	}
	return [2]int{a, b}
}

// TestSwissFourSeatDefaultRounds pins a 4-seat swiss event's default round
// count: ceil(log2 4) == 2.
func TestSwissFourSeatDefaultRounds(t *testing.T) {
	e := event.New()
	mustApply(t, e, event.Entry{Kind: event.KindEventCreated, Set: "TST", Seats: testSeats(), Structure: "swiss"})
	if got := e.Rounds(); got != 2 {
		t.Errorf("4-seat swiss rounds is %d, want 2", got)
	}
	if e.Structure() != event.StructureSwiss {
		t.Errorf("structure is %q, want swiss", e.Structure())
	}
}

// TestRoundPairedLogRedactionNothingHidden pins the log-level fact: a
// RoundPaired entry survives LogFor unredacted for every viewer (pairings
// are public) and round-trips through MarshalLog/ParseLog.
func TestRoundPairedLogRedactionNothingHidden(t *testing.T) {
	e := newEightSeatDraft(t, event.Entry{Structure: "swiss"})
	saveAndReadyAll(t, e)
	pairs, err := e.SuggestedPairings()
	if err != nil {
		t.Fatalf("SuggestedPairings: %v", err)
	}
	mustApply(t, e, event.Entry{Kind: event.KindRoundPaired, Round: 1, Pairs: pairs})
	for _, viewer := range []int{0, 3, 8} {
		for _, en := range e.LogFor(viewer) {
			if en.Kind != event.KindRoundPaired {
				continue
			}
			if fmt.Sprint(en.Pairs) != fmt.Sprint(pairs) {
				t.Errorf("viewer %d sees pairs %v, want %v", viewer, en.Pairs, pairs)
			}
		}
	}
}
