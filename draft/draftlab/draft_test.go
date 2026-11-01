package draftlab

import (
	"encoding/json"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// newDrafters resolves n fresh drafter instances of one agent: every seat
// gets its own state, the way a live pod would.
func newDrafters(t *testing.T, l *Lab, name string, n int) []NamedDrafter {
	t.Helper()
	out := make([]NamedDrafter, n)
	for i := range out {
		out[i] = testDrafter(t, l, name)
	}
	return out
}

// checkDraft asserts a finished draft's invariants: pools pick-complete,
// per-seat draft IDs well-formed, log carries the picks. packSize is one
// pack's card count (a seat receives exactly `rounds` packs, so its pool
// is rounds*packSize cards and the pod's picks are seats*rounds*packSize).
func checkDraft(t *testing.T, res DraftResult, packs [][][]limited.CardRef, seats, rounds int) {
	t.Helper()
	if len(res.Pools) != seats {
		t.Fatalf("%d pools, want %d", len(res.Pools), seats)
	}
	packSize := len(packs[0][0])
	for s := range res.Pools {
		if got, want := len(res.Pools[s]), rounds*packSize; got != want {
			t.Errorf("seat %d pool has %d cards, want %d", s, got, want)
		}
		if got, want := res.DraftIDs[s], draftID(res.Seed, s); got != want {
			t.Errorf("seat %d draft id = %q, want %q", s, got, want)
		}
		if len(res.Drafters[s].Name) == 0 {
			t.Errorf("seat %d drafter ref is empty", s)
		}
	}
	entries, err := event.ParseLog(res.Log)
	if err != nil {
		t.Fatalf("ParseLog: %v", err)
	}
	picked := 0
	for _, en := range entries {
		if en.Kind == event.KindPicked {
			picked++
		}
	}
	if want := seats * rounds * packSize; picked != want {
		t.Errorf("log has %d Picked entries, want %d", picked, want)
	}
}

func draftID(seed uint64, seat int) string {
	return itoa(uint64(seed)) + "-s" + itoa(uint64(seat))
}

func itoa(n uint64) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// sameRefs compares two card slices element-wise.
func sameRefs(a, b []limited.CardRef) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// sameDraft compares two results byte-wise on everything a caller sees.
func sameDraft(a, b DraftResult) bool {
	if a.Set != b.Set || a.Seed != b.Seed || len(a.Pools) != len(b.Pools) {
		return false
	}
	if string(a.Log) != string(b.Log) {
		return false
	}
	for s := range a.Pools {
		if !sameRefs(a.Pools[s], b.Pools[s]) {
			return false
		}
		if a.Drafters[s] != b.Drafters[s] || a.DraftIDs[s] != b.DraftIDs[s] {
			return false
		}
	}
	return true
}

func TestDraftEightSeatDeterministic(t *testing.T) {
	l := testLab(t)
	packs, err := l.OpenPod("FDN", setdata.Arena, 42, 8, 3)
	if err != nil {
		t.Fatalf("OpenPod: %v", err)
	}
	drafters := newDrafters(t, l, "rating-colour", 8)
	res1, err := l.Draft(packs, drafters, 7)
	if err != nil {
		t.Fatalf("Draft: %v", err)
	}
	checkDraft(t, res1, packs, 8, 3)
	if res1.Set != "FDN" {
		t.Errorf("Set = %q, want FDN", res1.Set)
	}

	// Same inputs, fresh drafter instances, same seed: identical result.
	res2, err := l.Draft(packs, newDrafters(t, l, "rating-colour", 8), 7)
	if err != nil {
		t.Fatalf("Draft again: %v", err)
	}
	if !sameDraft(res1, res2) {
		t.Error("two same-seed drafts disagree")
	}

	// A different seed diverges somewhere.
	res3, err := l.Draft(packs, newDrafters(t, l, "rating-colour", 8), 8)
	if err != nil {
		t.Fatalf("Draft seed+1: %v", err)
	}
	if sameDraft(res1, res3) {
		t.Error("seed+1 draft is identical to the seed draft")
	}

	// The log replays on its own: the EventCreated seed rebuilds every
	// pick rng, so replay is drafter-free and byte-faithful.
	e, err := event.ReplayLog(res1.Log)
	if err != nil {
		t.Fatalf("ReplayLog: %v", err)
	}
	if e.Phase() != event.PhaseBuild {
		t.Errorf("replayed event is in phase %s, want BUILD", e.Phase())
	}
	// The event's own pools are the result's pools (e.Pool returns a copy;
	// the result's slices must match it).
	for s := range res1.Pools {
		if !sameRefs(e.Pool(s), res1.Pools[s]) {
			t.Errorf("seat %d pool disagrees with the event's", s)
		}
	}
}

func TestDraftConcurrentCallsAgree(t *testing.T) {
	l := testLab(t)
	packs, err := l.OpenPod("FDN", setdata.Arena, 42, 8, 3)
	if err != nil {
		t.Fatalf("OpenPod: %v", err)
	}
	const runs = 8
	results := make([]DraftResult, runs)
	errs := make([]error, runs)
	done := make(chan int, runs)
	for i := 0; i < runs; i++ {
		go func(i int) {
			// Fresh drafter instances per goroutine: the agents' own rule.
			drafters := newDrafters(t, l, "rating-colour", 8)
			results[i], errs[i] = l.Draft(packs, drafters, 7)
			done <- i
		}(i)
	}
	for i := 0; i < runs; i++ {
		<-done
	}
	for i, err := range errs {
		if err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}
	for i := 1; i < runs; i++ {
		if !sameDraft(results[0], results[i]) {
			t.Errorf("concurrent run %d disagrees with run 0", i)
		}
	}
}

func TestDraftRejectsBadInput(t *testing.T) {
	l := testLab(t)
	pod4, err := l.OpenPod("FDN", setdata.Arena, 42, 4, 3)
	if err != nil {
		t.Fatal(err)
	}
	drafters1 := newDrafters(t, l, "rating-colour", 1)

	// A seat count that is not 4 or 8 is refused before any event work.
	pod5, err := l.OpenPod("FDN", setdata.Arena, 43, 5, 3)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.Draft(pod5, newDrafters(t, l, "rating-colour", 5), 7); err == nil || !containsAll(err.Error(), "5 seats") {
		t.Errorf("5-seat draft err = %v, want a seat-count refusal", err)
	}

	// Three drafters for four seats.
	if _, err := l.Draft(pod4, newDrafters(t, l, "rating-colour", 3), 7); err == nil || !containsAll(err.Error(), "3 drafters for 4 seats") {
		t.Errorf("3-drafter draft err = %v, want a drafter-count refusal", err)
	}

	// One drafter name for every seat is legal.
	if _, err := l.Draft(pod4, drafters1, 7); err != nil {
		t.Errorf("1-drafter draft: %v", err)
	}

	// Packs from separate OpenPacks calls carry duplicate IDs; the
	// PackOpened apply must surface it with the OpenPod hint.
	p1, err := l.OpenPacks("FDN", setdata.Arena, 99, 3)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := l.OpenPacks("FDN", setdata.Arena, 99, 3)
	if err != nil {
		t.Fatal(err)
	}
	dupes := make([][][]limited.CardRef, 4)
	for s := 0; s < 4; s++ {
		dupes[s] = [][]limited.CardRef{p1[0], p2[0], p1[0]}
	}
	_, err = l.Draft(dupes, drafters1, 7)
	if err == nil || !containsAll(err.Error(), "already used in another pack", "one OpenPod call") {
		t.Errorf("duplicate-ID packs err = %v, want the duplicate-ID refusal with the OpenPod hint", err)
	}

	// A mixed-set pods is refused: one card relabelled to another set.
	mixed, err := l.OpenPod("FDN", setdata.Arena, 44, 4, 3)
	if err != nil {
		t.Fatal(err)
	}
	mixed[0][0][0].Set = "ZZZ"
	if _, err := l.Draft(mixed, drafters1, 7); err == nil || !containsAll(err.Error(), "mixed sets") {
		t.Errorf("mixed-set draft err = %v, want the mixed-set refusal", err)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !containsSub(s, sub) {
			return false
		}
	}
	return true
}

func containsSub(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
