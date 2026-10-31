package draft

import (
	"errors"
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited"
)

// mkPacksN builds a seats×rounds pack set as slices: seat s's round-r pack
// is tagged "r<r>s<s>", with n cards per pack.
func mkPacksN(seats, rounds, n int) [][][]limited.CardRef {
	packs := make([][][]limited.CardRef, seats)
	for s := 0; s < seats; s++ {
		packs[s] = make([][]limited.CardRef, rounds)
		for r := 0; r < rounds; r++ {
			packs[s][r] = mkPack("r"+string(rune('1'+r))+"s"+string(rune('0'+s)), n)
		}
	}
	return packs
}

// driveAll picks seat by seat over p.Seats(), skipping seats with nothing
// on offer, until the pod is done.
func driveAll(t *testing.T, p *Pod) {
	t.Helper()
	for !p.Done() {
		moved := false
		for s := 0; s < p.Seats(); s++ {
			pickNo, id := lexPicker(p, s)
			if id == "" {
				continue
			}
			if err := p.Pick(s, pickNo, id); err != nil {
				t.Fatalf("pick seat %d: %v", s, err)
			}
			moved = true
		}
		if !moved {
			t.Fatal("no seat could pick but the pod is not done")
		}
	}
}

func TestNewPodEightSeatPassDirection(t *testing.T) {
	packs := mkPacksN(8, 3, 14)
	p, err := NewPod(17, packs)
	if err != nil {
		t.Fatalf("NewPod: %v", err)
	}
	if p.Seats() != 8 || p.Rounds() != 3 {
		t.Fatalf("pod = %d seats × %d rounds, want 8×3", p.Seats(), p.Rounds())
	}
	driveAll(t, p)
	if !p.Done() {
		t.Fatal("pod not done")
	}
	// With the lexically-first picker, the k-th smallest ID of the pack
	// opened by seat s goes to the k-th seat the pack visits: rounds 1 and
	// 3 pass left, round 2 passes right in an 8-seat pod. Rounds 1 and 3
	// make seat 0's pack next seen by seat 1; round 2, by seat 7.
	for s := 0; s < 8; s++ {
		for r := 0; r < 3; r++ {
			offset := 1
			if r == 1 {
				offset = 7
			}
			ids := make([]string, 0, len(packs[s][r]))
			for _, c := range packs[s][r] {
				ids = append(ids, c.ID)
			}
			sort.Strings(ids)
			for k, id := range ids {
				want := (s + k*offset) % 8
				got := -1
				for seat := 0; seat < 8; seat++ {
					for _, pid := range poolIDs(p, seat) {
						if pid == id {
							got = seat
						}
					}
				}
				if got != want {
					t.Fatalf("round %d pack of seat %d: card %s (visit %d) pooled at seat %d, want seat %d", r+1, s, id, k+1, got, want)
				}
			}
		}
	}
}

func TestNewPodEightSeatConservesCards(t *testing.T) {
	packs := mkPacksN(8, 3, 14)
	p, err := NewPod(23, packs)
	if err != nil {
		t.Fatalf("NewPod: %v", err)
	}
	driveAll(t, p)
	if !p.Done() {
		t.Fatal("pod not done after every pack was emptied")
	}
	owner := map[string]int{}
	for s := 0; s < 8; s++ {
		pool := p.Pool(s)
		if len(pool) != 42 {
			t.Fatalf("seat %d pool = %d cards, want 42", s, len(pool))
		}
		for _, c := range pool {
			if prev, dup := owner[c.ID]; dup {
				t.Errorf("card %s pooled at seats %d and %d", c.ID, prev, s)
			}
			owner[c.ID] = s
		}
	}
	if len(owner) != 8*3*14 {
		t.Fatalf("%d distinct cards pooled, want %d", len(owner), 8*3*14)
	}
	if got := p.Record(); len(got) != 8*3*14 {
		t.Fatalf("record has %d picks, want %d", len(got), 8*3*14)
	}
}

func TestNewPodEightSeatInterleavingIndependence(t *testing.T) {
	packs := mkPacksN(8, 3, 14)
	a, err := NewPod(11, packs)
	if err != nil {
		t.Fatalf("NewPod: %v", err)
	}
	driveAll(t, a)

	b, err := NewPod(11, packs)
	if err != nil {
		t.Fatalf("NewPod: %v", err)
	}
	for !b.Done() {
		// Longest queue wins; ties break by seat number. No map, no shuffle.
		best, bestLen := -1, -1
		for s := 0; s < b.Seats(); s++ {
			if l := b.QueueLen(s); l > bestLen {
				best, bestLen = s, l
			}
		}
		pickNo, id := lexPicker(b, best)
		if id == "" {
			t.Fatalf("longest queue seat %d had no pack but the pod is not done", best)
		}
		if err := b.Pick(best, pickNo, id); err != nil {
			t.Fatalf("pick seat %d: %v", best, err)
		}
	}

	for s := 0; s < 8; s++ {
		pa, pb := poolIDs(a, s), poolIDs(b, s)
		sort.Strings(pa)
		sort.Strings(pb)
		if len(pa) != len(pb) {
			t.Fatalf("seat %d: pool lens %d vs %d", s, len(pa), len(pb))
		}
		for i := range pa {
			if pa[i] != pb[i] {
				t.Fatalf("seat %d: pool[%d] = %s vs %s", s, i, pa[i], pb[i])
			}
		}
	}
	ra, rb := a.Record(), b.Record()
	if len(ra) != len(rb) {
		t.Fatalf("records %d vs %d picks", len(ra), len(rb))
	}
	less := func(x, y Picked) bool {
		if x.Seat != y.Seat {
			return x.Seat < y.Seat
		}
		if x.Round != y.Round {
			return x.Round < y.Round
		}
		return x.PickNo < y.PickNo
	}
	sort.Slice(ra, func(i, j int) bool { return less(ra[i], ra[j]) })
	sort.Slice(rb, func(i, j int) bool { return less(rb[i], rb[j]) })
	for i := range ra {
		if ra[i] != rb[i] {
			t.Fatalf("record[%d] = %+v vs %+v", i, ra[i], rb[i])
		}
	}
}

func TestNewPodMatchesNewForFourByThree(t *testing.T) {
	arrayed := mkPacks(14)
	sliced := mkPacksN(4, 3, 14)
	// Precondition: the two pack sets are the same cards.
	for s := 0; s < 4; s++ {
		for r := 0; r < 3; r++ {
			if len(arrayed[s][r]) != len(sliced[s][r]) {
				t.Fatalf("pack [%d][%d] length %d vs %d", s, r, len(arrayed[s][r]), len(sliced[s][r]))
			}
			for i := range arrayed[s][r] {
				if arrayed[s][r][i].ID != sliced[s][r][i].ID {
					t.Fatalf("pack [%d][%d][%d] = %s vs %s", s, r, i, arrayed[s][r][i].ID, sliced[s][r][i].ID)
				}
			}
		}
	}
	a := New(29, arrayed)
	driveRoundRobin(t, a)
	b, err := NewPod(29, sliced)
	if err != nil {
		t.Fatalf("NewPod: %v", err)
	}
	driveAll(t, b)

	if a.Seats() != b.Seats() || a.Rounds() != b.Rounds() {
		t.Fatalf("shape %d×%d vs %d×%d", a.Seats(), a.Rounds(), b.Seats(), b.Rounds())
	}
	ra, rb := a.Record(), b.Record()
	if len(ra) != len(rb) {
		t.Fatalf("records %d vs %d picks", len(ra), len(rb))
	}
	for i := range ra {
		if ra[i] != rb[i] {
			t.Fatalf("record[%d] = %+v vs %+v", i, ra[i], rb[i])
		}
	}
	for s := 0; s < 4; s++ {
		pa, pb := poolIDs(a, s), poolIDs(b, s)
		if len(pa) != len(pb) {
			t.Fatalf("seat %d: pool lens %d vs %d", s, len(pa), len(pb))
		}
		for i := range pa {
			if pa[i] != pb[i] {
				t.Fatalf("seat %d: pool[%d] = %s vs %s", s, i, pa[i], pb[i])
			}
		}
	}
}

func TestNewPodRejectsBadShape(t *testing.T) {
	if _, err := NewPod(1, mkPacksN(1, 3, 14)); !errors.Is(err, ErrPodShape) {
		t.Errorf("1 seat: err = %v, want ErrPodShape", err)
	}
	if _, err := NewPod(1, mkPacksN(9, 3, 14)); !errors.Is(err, ErrPodShape) {
		t.Errorf("9 seats: err = %v, want ErrPodShape", err)
	}
	if _, err := NewPod(1, mkPacksN(4, 0, 14)); !errors.Is(err, ErrPodShape) {
		t.Errorf("0 rounds: err = %v, want ErrPodShape", err)
	}
	ragged := mkPacksN(4, 3, 14)
	ragged[2] = ragged[2][:2]
	if _, err := NewPod(1, ragged); !errors.Is(err, ErrPodShape) {
		t.Errorf("ragged rounds: err = %v, want ErrPodShape", err)
	}
	// Positive control: the MinSeats shape builds.
	if _, err := NewPod(1, mkPacksN(MinSeats, 3, 14)); err != nil {
		t.Errorf("MinSeats pod: %v", err)
	}
}

func TestNewPodTwoSeatsOneRound(t *testing.T) {
	packs := mkPacksN(2, 1, 14)
	p, err := NewPod(2, packs)
	if err != nil {
		t.Fatalf("NewPod: %v", err)
	}
	if p.Seats() != 2 || p.Rounds() != 1 {
		t.Fatalf("pod = %d seats × %d rounds, want 2×1", p.Seats(), p.Rounds())
	}
	// Precondition: both seats open their own round-1 packs.
	if p.QueueLen(1) != 1 {
		t.Fatalf("seat 1 queue = %d, want its own round-1 pack", p.QueueLen(1))
	}
	if v, ok := p.Next(1); !ok || v.Pack[0].ID != "r1s1-a" {
		t.Fatalf("seat 1 view ok=%v head=%v, want its own round-1 pack", ok, v.Pack)
	}
	if err := p.Pick(0, 1, "r1s0-a"); err != nil {
		t.Fatalf("seat 0 pick: %v", err)
	}
	// The odd round passes left, to (0+1)%2 — seat 1 now holds its own
	// pack and seat 0's remainder.
	if p.QueueLen(1) != 2 {
		t.Fatalf("seat 1 queue = %d, want 2 (own pack + seat 0's passed remainder)", p.QueueLen(1))
	}
	// Seat 1 picks its own pack head first; the remainder passes back to
	// seat 0, and the passed pack alternates seats thereafter.
	if err := p.Pick(1, 1, "r1s1-a"); err != nil {
		t.Fatalf("seat 1 pick: %v", err)
	}
	if p.QueueLen(0) != 1 {
		t.Fatalf("seat 0 queue = %d, want 1 (seat 1's passed remainder)", p.QueueLen(0))
	}
	v, ok := p.Next(1)
	if !ok || len(v.Pack) != 13 || v.Pack[0].ID != "r1s0-b" {
		t.Fatalf("seat 1 view ok=%v pack len=%d head=%v, want seat 0's 13-card remainder", ok, len(v.Pack), v.Pack)
	}
	driveAll(t, p)
	if !p.Done() {
		t.Fatal("2×1 pod not done after every pack was emptied")
	}
	owner := map[string]int{}
	for s := 0; s < 2; s++ {
		if got := len(p.Pool(s)); got != 14 {
			t.Fatalf("seat %d pool = %d cards, want 14", s, got)
		}
		for _, c := range p.Pool(s) {
			if _, dup := owner[c.ID]; dup {
				t.Errorf("card %s pooled twice", c.ID)
			}
			owner[c.ID] = s
		}
	}
	if len(owner) != 28 {
		t.Fatalf("%d distinct cards pooled, want 28", len(owner))
	}
}

func TestPodOutOfRangeSeat(t *testing.T) {
	p := New(4, mkPacks(2))
	// Positive control: seat 0 is in range and has a pack on offer.
	if v, ok := p.Next(0); !ok || len(v.Pack) != 2 {
		t.Fatalf("positive control: seat 0 view ok=%v pack=%d, want a 2-card pack", ok, len(v.Pack))
	}
	if _, ok := p.Next(-1); ok {
		t.Error("Next(-1) reported a pack")
	}
	if _, ok := p.Next(8); ok {
		t.Error("Next(8) reported a pack on a 4-seat pod")
	}
	if err := p.Pick(9, 1, "x"); err != ErrNotYourTurn {
		t.Errorf("Pick(9, …) = %v, want ErrNotYourTurn", err)
	}
	if err := p.Pick(-1, 1, "x"); err != ErrNotYourTurn {
		t.Errorf("Pick(-1, …) = %v, want ErrNotYourTurn", err)
	}
	if got := p.Pool(9); got != nil {
		t.Errorf("Pool(9) = %v, want nil", got)
	}
	if got := p.QueueLen(-1); got != 0 {
		t.Errorf("QueueLen(-1) = %d, want 0", got)
	}
	// A negative pick number on a real seat is still the ordinary stale
	// rule, not a panic.
	if err := p.Pick(0, -1, "x"); err != ErrStale {
		t.Errorf("Pick(0, -1, …) = %v, want ErrStale", err)
	}
}
