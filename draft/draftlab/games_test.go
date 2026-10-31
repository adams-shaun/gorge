package draftlab

import (
	"reflect"
	"testing"
)

// TestGamesWorkerCountIndependent: a 6-matchup batch is identical whether it
// runs at 1 worker or 4, so no result depends on scheduling.
func TestGamesWorkerCountIndependent(t *testing.T) {
	base := testLab(t)
	es := testEntrants(t, base, 4)
	var ms []Matchup
	for i := 0; i < 4; i++ {
		for j := i + 1; j < 4; j++ {
			ms = append(ms, Matchup{A: es[i], B: es[j]})
		}
	}
	if len(ms) != 6 {
		t.Fatalf("built %d matchups, want 6 (precondition)", len(ms))
	}
	l1 := testLabOpts(t, Options{Workers: 1})
	l4 := testLabOpts(t, Options{Workers: 4})
	r1, err := l1.Games(ms, 2024)
	if err != nil {
		t.Fatalf("Games workers=1: %v", err)
	}
	r4, err := l4.Games(ms, 2024)
	if err != nil {
		t.Fatalf("Games workers=4: %v", err)
	}
	if len(r1) != 6 || len(r4) != 6 {
		t.Fatalf("Games returned %d/%d results, want 6 each (precondition)", len(r1), len(r4))
	}
	if !reflect.DeepEqual(r1, r4) {
		t.Errorf("Games differs between workers=1 and workers=4")
	}
}
