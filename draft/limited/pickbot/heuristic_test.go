package pickbot_test

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited"
	pb "github.com/adams-shaun/gorge/draft/limited/pickbot"
)

// TestHeuristicRatingOrdering pins the heuristic's ordering on five named
// FDN cards, WITHOUT the 17Lands file: a mythic bomb over an efficient
// uncommon vanilla, over a removal common, over an evasion common, over a
// vanilla common. The expected shape of each card is asserted first, so a
// corpus drift fails loudly rather than flipping the order silently.
func TestHeuristicRatingOrdering(t *testing.T) {
	reg := corpus(t)
	type named struct {
		name string
		rar  limited.Rarity
		// the facts the score is expected to come from
		creature bool
		evades   int
		removal  bool
	}
	cards := []named{
		{"Bloodthirsty Conqueror", limited.Mythic, true, 0, false},
		{"Savannah Lions", limited.Uncommon, true, 0, false},
		{"Make Your Move", limited.Common, false, 0, true},
		{"Healer's Hawk", limited.Common, true, 1, false},
		{"Hungry Ghoul", limited.Common, true, 0, false},
	}
	for _, c := range cards {
		f := lookupCard(t, reg, c.name).Faces[0]
		if f.IsCreature() != c.creature {
			t.Fatalf("%s creature=%v, want %v", c.name, f.IsCreature(), c.creature)
		}
		if c.removal {
			if !f.IsInstant() && !f.IsSorcery() {
				t.Fatalf("%s is neither instant nor sorcery", c.name)
			}
			o := f.Oracle
			if !containsAny(o, "destroy target", "exile target", "damage to target") {
				t.Fatalf("%s oracle %q does not read as removal", c.name, o)
			}
		}
	}
	score := func(n named) float64 {
		return pb.Heuristic(lookupCard(t, reg, n.name), n.rar)
	}
	// Precondition: the five scores are not all equal.
	scores := make([]float64, len(cards))
	for i, c := range cards {
		scores[i] = score(c)
	}
	if scores[0] == scores[len(scores)-1] {
		t.Fatalf("first and last card both score %g: the ordering would be vacuous", scores[0])
	}
	for i := 0; i+1 < len(cards); i++ {
		if scores[i] <= scores[i+1] {
			t.Errorf("%s scored %g, want strictly above %s's %g", cards[i].name, scores[i], cards[i+1].name, scores[i+1])
		}
	}
}

// TestHeuristicRatingNilCard pins the base scores: a nil card — no facts at
// all — scores its rarity base only, and a basic-land rarity scores 0.
func TestHeuristicRatingNilCard(t *testing.T) {
	if got := pb.Heuristic(nil, limited.Mythic); got != 6 {
		t.Fatalf("Heuristic(nil, mythic) = %g, want 6", got)
	}
	if got := pb.Heuristic(nil, limited.Rare); got != 5 {
		t.Fatalf("Heuristic(nil, rare) = %g, want 5", got)
	}
	if got := pb.Heuristic(nil, limited.Uncommon); got != 2 {
		t.Fatalf("Heuristic(nil, uncommon) = %g, want 2", got)
	}
	if got := pb.Heuristic(nil, limited.Common); got != 0 {
		t.Fatalf("Heuristic(nil, common) = %g, want 0", got)
	}
	if got := pb.Heuristic(nil, limited.Land); got != 0 {
		t.Fatalf("Heuristic(nil, land) = %g, want 0", got)
	}
}

func containsAny(s string, subs ...string) bool {
	low := strings.ToLower(s)
	for _, sub := range subs {
		if strings.Contains(low, sub) {
			return true
		}
	}
	return false
}
