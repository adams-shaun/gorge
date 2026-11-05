package botpolicy

import (
	"slices"
	"testing"
)

// BottomingAlternatives offers MulliganLands' own split with one land more
// and one land fewer put back, then one other spell, never keeping fewer
// than two lands while the hand has them.
func TestBottomingAlternatives(t *testing.T) {
	// Three Plains and four spells of mana values 1..4, two to bottom: the
	// rule keeps three lands (K = 5 keeps at most ceil(5/2)) and bottoms the
	// 4- and 3-drops.
	hand := []Card{plainsCard(), plainsCard(), plainsCard(),
		spellCard(1, "W"), spellCard(2, "1 W"), spellCard(3, "2 W"), spellCard(4, "3 W")}
	for i := range hand[3:] {
		hand[3+i].PrintedName = string(rune('a' + i))
	}
	b := handBoard(MulliganLands, hand...)
	d := bottomDecision(7, 2)
	if own := b.landBottoming(&d); !slices.Equal(own, []int{5, 6}) {
		t.Fatalf("the rule bottoms %v, want the 3- and 4-drops [5 6]", own)
	}
	got := BottomingAlternatives(b, &d)
	// One land more: a Plains and the 4-drop. One land fewer is impossible:
	// the rule puts back no land. The spell swap puts back the 2-drop in
	// place of the 3-drop.
	want := [][]int{{0, 6}, {4, 6}}
	if !slices.EqualFunc(got, want, slices.Equal) {
		t.Fatalf("alternatives %v, want %v", got, want)
	}

	// Two lands and five spells, one to bottom: no alternative may drop a
	// land, so only the spell swap is offered.
	hand = []Card{plainsCard(), plainsCard(),
		spellCard(1, "W"), spellCard(2, "1 W"), spellCard(3, "2 W"), spellCard(4, "3 W"), spellCard(5, "4 W")}
	for i := range hand[2:] {
		hand[2+i].PrintedName = string(rune('a' + i))
	}
	b = handBoard(MulliganLands, hand...)
	d = bottomDecision(7, 1)
	if got := BottomingAlternatives(b, &d); !slices.EqualFunc(got, [][]int{{5}}, slices.Equal) {
		t.Fatalf("two-land alternatives %v, want only the 4-drop in place of the 5-drop [[5]]", got)
	}
}
