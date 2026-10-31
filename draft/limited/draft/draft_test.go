package draft

import (
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited"
)

// mkPack builds a pack of n cards whose IDs are "<tag>-<i>", all sharing one
// printing set so a PickView's Set is derivable.
func mkPack(tag string, n int) []limited.CardRef {
	pack := make([]limited.CardRef, n)
	for i := range pack {
		pack[i] = limited.CardRef{
			ID:     tag + "-" + string(rune('a'+i)),
			Name:   tag,
			Set:    "FDN",
			Number: "1",
			Rarity: limited.Common,
		}
	}
	return pack
}

// mkPacks builds a full [seat][round] pack set: seat s's round-r pack is
// tagged "r<r>s<s>", with n cards per pack.
func mkPacks(n int) [4][3][]limited.CardRef {
	var packs [4][3][]limited.CardRef
	for s := 0; s < 4; s++ {
		for r := 0; r < 3; r++ {
			packs[s][r] = mkPack("r"+string(rune('1'+r))+"s"+string(rune('0'+s)), n)
		}
	}
	return packs
}

// lexPicker takes the lexically first ID still on offer at seat's queue head.
func lexPicker(p *Pod, seat int) (int, string) {
	v, ok := p.Next(seat)
	if !ok {
		return 0, ""
	}
	ids := make([]string, 0, len(v.Pack))
	for _, c := range v.Pack {
		ids = append(ids, c.ID)
	}
	sort.Strings(ids)
	return v.PickNo, ids[0]
}

// driveRoundRobin picks seat by seat, skipping seats with nothing on offer,
// until the pod is done.
func driveRoundRobin(t *testing.T, p *Pod) {
	t.Helper()
	for !p.Done() {
		moved := false
		for s := 0; s < 4; s++ {
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

// drive picks exactly n picks, always the first seat in seat order that has
// a pack on offer, so tests control interleavings precisely.
func drive(t *testing.T, p *Pod, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		picked := false
		for s := 0; s < 4 && !picked; s++ {
			pickNo, id := lexPicker(p, s)
			if id == "" {
				continue
			}
			if err := p.Pick(s, pickNo, id); err != nil {
				t.Fatalf("pick seat %d: %v", s, err)
			}
			picked = true
		}
		if !picked {
			t.Fatalf("no seat could pick for pick %d of %d (round %d)", i+1, n, p.round)
		}
	}
}

func poolIDs(p *Pod, seat int) []string {
	ids := make([]string, 0, len(p.Pool(seat)))
	for _, c := range p.Pool(seat) {
		ids = append(ids, c.ID)
	}
	return ids
}

func TestPodFullDraftConservesCards(t *testing.T) {
	packs := mkPacks(14)
	p := New(7, packs)
	driveRoundRobin(t, p)
	if !p.Done() {
		t.Fatal("pod not done after every pack was emptied")
	}
	owner := map[string]int{}
	for s := 0; s < 4; s++ {
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
	if len(owner) != 4*3*14 {
		t.Fatalf("%d distinct cards pooled, want %d", len(owner), 4*3*14)
	}
	rec := p.Record()
	if len(rec) != 4*3*14 {
		t.Fatalf("record has %d picks, want %d", len(rec), 4*3*14)
	}
}

func TestPodPassDirectionByPoolMembership(t *testing.T) {
	// With the lexically-first picker, the k-th smallest ID of the pack
	// opened by seat s goes to the k-th seat the pack visits: rounds 1 and
	// 3 pass left (s, s+1, s+2, s+3 mod 4), round 2 passes right
	// (s, s+3, s+2, s+1 mod 4). A 14-card pack visits each seat repeatedly,
	// so every k from 1 to 14 pins a direction step.
	packs := mkPacks(14)
	p := New(11, packs)
	driveRoundRobin(t, p)
	if !p.Done() {
		t.Fatal("pod not done")
	}
	for s := 0; s < 4; s++ {
		for r := 0; r < 3; r++ {
			offset := 1
			if r == 1 {
				offset = 3
			}
			ids := make([]string, 0, len(packs[s][r]))
			for _, c := range packs[s][r] {
				ids = append(ids, c.ID)
			}
			sort.Strings(ids)
			for k, id := range ids {
				want := (s + k*offset) % 4
				got := -1
				for seat := 0; seat < 4; seat++ {
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

func TestPodRoundTwoPackNextSeenBySeatThree(t *testing.T) {
	// Round 1 packs of one card (a round is four picks), seat 3's round-2
	// pack of one card, everything else two cards. After seat 0 picks from
	// its round-2 pack, the remainder passes right (0 → 3) and — seat 3
	// having emptied its own single-card pack — lands at seat 3's queue
	// head.
	packs := mkPacks(2)
	packs[0][0] = mkPack("r1s0", 1)
	packs[1][0] = mkPack("r1s1", 1)
	packs[2][0] = mkPack("r1s2", 1)
	packs[3][0] = mkPack("r1s3", 1)
	packs[3][1] = mkPack("r2s3", 1)
	packs[0][2] = mkPack("r3s0", 1)
	packs[1][2] = mkPack("r3s1", 1)
	packs[2][2] = mkPack("r3s2", 1)
	packs[3][2] = mkPack("r3s3", 1)
	p := New(3, packs)

	drive(t, p, 4)
	if p.round != 2 {
		t.Fatalf("round = %d, want 2 after round 1's four picks", p.round)
	}
	// Precondition: seat 0's queue head is its two-card round-2 pack.
	v0, ok := p.Next(0)
	if !ok || len(v0.Pack) != 2 || v0.Pack[0].ID != "r2s0-a" || v0.Pack[1].ID != "r2s0-b" {
		t.Fatalf("seat 0 queue head ok=%v pack=%v, want [r2s0-a r2s0-b]", ok, v0.Pack)
	}
	keep := v0.Pack[0].ID
	wantRest := v0.Pack[1].ID
	if err := p.Pick(0, v0.PickNo, keep); err != nil {
		t.Fatalf("pick: %v", err)
	}
	if p.QueueLen(3) != 2 {
		t.Fatalf("seat 3 queue = %d packs, want 2 (own + the right-passed remainder)", p.QueueLen(3))
	}
	// Empty seat 3's own one-card pack first.
	v3, ok := p.Next(3)
	if !ok || v3.Pack[0].ID != "r2s3-a" {
		t.Fatalf("seat 3 queue head = %v, want its own r2s3 pack", v3.Pack)
	}
	if err := p.Pick(3, v3.PickNo, v3.Pack[0].ID); err != nil {
		t.Fatalf("seat 3 pick: %v", err)
	}
	// Now the passed pack is at the head: seat 3 next sees seat 0's pack.
	v3b, ok := p.Next(3)
	if !ok {
		t.Fatal("seat 3 has no pack; the right-passed remainder never arrived")
	}
	if len(v3b.Pack) != 1 || v3b.Pack[0].ID != wantRest || v3b.Pack[0].ID != "r2s0-b" {
		t.Fatalf("seat 3 queue head = %v, want the [%s] remainder of seat 0's round-2 pack", v3b.Pack, wantRest)
	}
	if v3b.Round != 2 {
		t.Fatalf("seen round = %d, want 2", v3b.Round)
	}
}

func TestPodRoundBarrierOpensNextRoundTogether(t *testing.T) {
	// With one-card packs a round ends after exactly four picks; while the
	// last seat still owes its pick no new pack may be queued anywhere.
	p := New(5, mkPacks(1))
	drive(t, p, 3)
	// Seats 0-2 emptied their one-card packs; seat 3 still owes its pick,
	// so its own pack is on offer but no new round may have opened.
	for s := 0; s < 3; s++ {
		if _, ok := p.Next(s); ok {
			t.Fatalf("seat %d was offered a pack before seat 3 emptied round 1", s)
		}
	}
	if v, ok := p.Next(3); !ok || v.Round != 1 {
		t.Fatalf("seat 3 view ok=%v round=%d, want its own round-1 pack still on offer", ok, v.Round)
	}
	if p.Done() {
		t.Fatal("pod reported done with rounds left")
	}
	if p.round != 1 {
		t.Fatalf("round = %d, want 1 while seat 3 still owes its pick", p.round)
	}
	drive(t, p, 1)
	if p.round != 2 {
		t.Fatalf("round = %d, want 2 after the barrier opened", p.round)
	}
}

func TestPodInterleavingIndependence(t *testing.T) {
	// The same packs, the same lexically-first picker, two interleavings:
	// seat-round-robin vs. always-the-longest-queue. Pools (as sorted ID
	// sets) and the sorted pick record must come out identical.
	packs := mkPacks(14)
	a := New(11, packs)
	driveRoundRobin(t, a)

	b := New(11, packs)
	for !b.Done() {
		// Longest queue wins; ties break by seat number. No map, no shuffle.
		best, bestLen := -1, -1
		for s := 0; s < 4; s++ {
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

	for s := 0; s < 4; s++ {
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

func TestPodPickErrors(t *testing.T) {
	packs := mkPacks(2)
	p := New(9, packs)

	// Precondition: seat 0's round-1 pack is on offer with two cards.
	v, ok := p.Next(0)
	if !ok || len(v.Pack) != 2 {
		t.Fatalf("seat 0 queue head: ok=%v len=%d, want a 2-card pack", ok, len(v.Pack))
	}
	otherID := v.Pack[1].ID

	// A wrong pick number is stale while the pack is on offer.
	if err := p.Pick(0, 3, v.Pack[0].ID); err != ErrStale {
		t.Errorf("Pick pickNo=3 = %v, want ErrStale", err)
	}
	// A card from a different pack is not in this pack.
	if err := p.Pick(0, 1, "r1s1-a"); err != ErrNotInPack {
		t.Errorf("Pick foreign card = %v, want ErrNotInPack", err)
	}
	if got := len(p.Pool(0)); got != 0 {
		t.Errorf("failed picks pooled %d cards, want 0", got)
	}
	// A valid pick empties seat 0's queue until a pack is passed back.
	if err := p.Pick(0, 1, otherID); err != nil {
		t.Fatalf("pick: %v", err)
	}
	if p.QueueLen(0) != 0 {
		t.Fatalf("seat 0 queue = %d, want 0 after its 2-card pack was passed", p.QueueLen(0))
	}
	if err := p.Pick(0, 2, "r1s0-a"); err != ErrNotYourTurn {
		t.Errorf("empty-queue pick = %v, want ErrNotYourTurn", err)
	}
}

func TestPodPickViewCopiesAreDefensive(t *testing.T) {
	p := New(4, mkPacks(2))
	v, ok := p.Next(0)
	if !ok || len(v.Pack) != 2 || len(v.Seen) != 0 {
		t.Fatalf("precondition: seat 0 view ok=%v pack=%d seen=%d, want a 2-card pack, no seen", ok, len(v.Pack), len(v.Seen))
	}
	if v.Set != "FDN" || v.PickNo != 1 || v.Round != 1 {
		t.Fatalf("view = set %q round %d pick %d, want FDN/1/1", v.Set, v.Round, v.PickNo)
	}
	// Mutating the view's slices must not reach the pod.
	v.Pack[0].ID = "tampered"
	if err := p.Pick(0, v.PickNo, "r1s0-a"); err != nil {
		t.Fatalf("pick after view tamper: %v", err)
	}
	if got := p.Pool(0)[0].ID; got != "r1s0-a" {
		t.Fatalf("pool got %q, want r1s0-a (a view mutation reached the pod)", got)
	}
	// Seat 0's remainder passed left to seat 1 behind its own pack; seat 1
	// picks its own pack once, and the next view at seat 1 is seat 0's
	// remainder with seat 1's history attached.
	if err := p.Pick(1, 1, "r1s1-a"); err != nil {
		t.Fatalf("seat 1 pick: %v", err)
	}
	v2, ok := p.Next(1)
	if !ok || v2.PickNo != 2 {
		t.Fatalf("second view ok=%v pickNo=%d, want ok with pickNo 2", ok, v2.PickNo)
	}
	if len(v2.Pack) != 1 || v2.Pack[0].ID != "r1s0-b" {
		t.Fatalf("second view pack = %v, want seat 0's unmutated remainder [r1s0-b]", v2.Pack)
	}
	if len(v2.Seen) != 1 || len(v2.Seen[0]) != 2 || v2.Seen[0][0].ID != "r1s1-a" {
		t.Fatalf("seen = %v, want seat 1's own pack as first seen, unmutated", v2.Seen)
	}
	if len(v2.Picks) != 1 || v2.Picks[0].ID != "r1s1-a" {
		t.Fatalf("view picks = %v, want the one picked card", v2.Picks)
	}
}

// TestPickViewSeatsIsPodSize: every view a 4- and an 8-seat pod hands out
// carries the pod size, from the first pick to the last.
func TestPickViewSeatsIsPodSize(t *testing.T) {
	for _, n := range []int{4, 8} {
		p, err := NewPod(1, mkPacksN(n, 3, n+2))
		if err != nil {
			t.Fatal(err)
		}
		views := 0
		for !p.Done() {
			for s := 0; s < n; s++ {
				v, ok := p.Next(s)
				if !ok {
					continue
				}
				if v.Seats != n {
					t.Fatalf("n=%d seat %d round %d pick %d: Seats = %d", n, s, v.Round, v.PickNo, v.Seats)
				}
				views++
				if err := p.Pick(s, v.PickNo, v.Pack[0].ID); err != nil {
					t.Fatal(err)
				}
			}
		}
		if views != n*3*(n+2) {
			t.Fatalf("n=%d: checked %d views", n, views)
		}
	}
}
