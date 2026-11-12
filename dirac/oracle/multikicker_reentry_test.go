package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Multikicker count ask and the ETB target bound read state a cast leaves
// behind (CR 702.43, kw_multikicker.go). Two shapes the class lockstep's
// basics-only decks cannot reach:
//
//   - a granted Convoke (Chief Engineer: artifact spells you cast have
//     convoke): gorge's count walk folds the creature credit into each
//     candidate (convokeCountCredit), so the kernel's bound must offer the
//     same counts as the floating pool plus the untapped creatures;
//   - a Multikicker permanent that re-enters without being cast (Zombify on a
//     Batroc the legend rule sent to the graveyard) is a new object with no
//     payments: its ETB trigger's target bound reads 0, not the last cast's
//     count, when the permanent leaves before its targets are chosen.

// The granted-Convoke deck floats Dark Ritual's {B}{B}{B} first (gorge's
// pool-only multikicked offer under a credit-bearing cast), so the Chalice's
// count ask has the pool plus the untapped Chief Engineers' credit to bound it.
func TestMultikickerGrantedConvokeLockstep(t *testing.T) {
	t.Parallel()
	d := []cardset.Entry{
		{Name: "Everflowing Chalice", Count: 4}, {Name: "Chief Engineer", Count: 8}, {Name: "Dark Ritual", Count: 8},
		{Name: "Island", Count: 10}, {Name: "Swamp", Count: 10},
	}
	tl := &kmultikickerTally{casts: map[string]int{}, counts: map[int]int{}}
	runMultikicker(t, [2][]cardset.Entry{d, d}, 24, func(seed uint64) Policy {
		return kmultikickerPolicy(seed, tl)
	})
	t.Logf("multikicked casts %v; counts %v", tl.casts, tl.counts)
	if tl.casts["Everflowing Chalice"] == 0 {
		t.Error("no multikicked Chalice cast")
	}
	// the pool's {B}{B}{B} pays one payment; a second is the creature credit's
	two := 0
	for n, c := range tl.counts {
		if n >= 2 {
			two += c
		}
	}
	if two == 0 {
		t.Error("no count of two or more was answered: the creature credit was never part of a count bound")
	}
}

func TestMultikickerReanimatedLockstep(t *testing.T) {
	t.Parallel()
	d := []cardset.Entry{
		{Name: "Batroc the Leaper", Count: 8}, {Name: "Zombify", Count: 6},
		{Name: "Mountain", Count: 13}, {Name: "Swamp", Count: 13},
	}
	tl := &mkClassTally{casts: map[string]int{}, counts: map[int]int{}}
	runMultikicker(t, [2][]cardset.Entry{d, d}, 64, func(seed uint64) Policy {
		return mkClassPolicy(seed, "Batroc the Leaper", false, tl)
	})
	t.Logf("multikicked casts %v; counts %v", tl.casts, tl.counts)
	if tl.casts["Batroc the Leaper"] == 0 {
		t.Error("no multikicked Batroc cast")
	}
}
