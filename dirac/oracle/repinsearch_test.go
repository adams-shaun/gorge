package oracle

import (
	"testing"
	"time"
)

func TestRepinSearchFindsTheFirstPairThatReaches(t *testing.T) {
	var tried []int
	got := repinSearch(map[int]bool{0: true, 2: true}, 1, 10, time.Minute, func(p int) bool {
		tried = append(tried, p)
		return p == 4
	})
	if len(got) != 1 || got[0] != 4 {
		t.Fatalf("got %v; want [4]", got)
	}
	if want := []int{1, 3, 4}; len(tried) != 3 || tried[0] != want[0] || tried[1] != want[1] || tried[2] != want[2] {
		t.Fatalf("tried %v, want %v: taken pairs are skipped and the order is by index", tried, want)
	}
}

func TestRepinSearchIsBounded(t *testing.T) {
	n := 0
	if got := repinSearch(nil, 1, 5, time.Minute, func(int) bool { n++; return false }); len(got) != 0 || n != 5 {
		t.Fatalf("found=%v after %d plays, want none after 5", got, n)
	}
	n = 0
	// an already-spent wall-clock budget stops the search before the pair bound does
	if got := repinSearch(nil, 1, 1000, -time.Second, func(int) bool { n++; return false }); len(got) != 0 || n >= 1000 {
		t.Fatalf("found=%v after %d plays, want the spent wall-clock to stop it", got, n)
	}
	got := repinSearch(nil, 2, 10, time.Minute, func(p int) bool { return p%3 == 1 })
	if len(got) != 2 || got[0] != 1 || got[1] != 4 {
		t.Fatalf("got %v, want the first two good pairs [1 4]", got)
	}
}
