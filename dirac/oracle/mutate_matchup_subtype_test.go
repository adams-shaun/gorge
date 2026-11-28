package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestMutateMatchupKeepsHumanSubtype builds the table of a matchup in which
// only a mutate target names Human, and wants the matchup to keep it: on the
// Human creature's mask and in the target filter's negation (CR 702.140a,
// "non-Human creature"). Dropped, the filter reads NoSubtype and the mutating
// cast offers the Human.
func TestMutateMatchupKeepsHumanSubtype(t *testing.T) {
	t.Parallel()
	const mutator, human = "Glowstone Recluse", "James, Wandering Dad"
	decks := [2][]string{{mutator, "Forest"}, {human, "Island"}}
	set := reproSet(t, "custom", &Repro{Cards: decks})
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(decks[0]), entries(decks[1]))
	if err != nil {
		t.Fatal(err)
	}
	card := map[string]int{}
	for i := range tbl.Cards {
		card[tbl.Cards[i].Name] = i
	}
	mi, okM := card[mutator]
	hi, okH := card[human]
	if !okM || !okH || !tbl.Cards[mi].HasMutate {
		t.Fatalf("matchup holds %s %v, %s %v: want both, the first with mutate", mutator, okM, human, okH)
	}
	f := tbl.Filters[tbl.Cards[mi].MutateTgt.Filter]
	if f.NotSub == 0 || int(f.NotSub-1) >= len(tbl.Subtypes) {
		t.Fatalf("mutate target NotSub = %d of %d subtypes, want the index of Human", f.NotSub, len(tbl.Subtypes))
	}
	if got := tbl.Subtypes[f.NotSub-1]; got != "Human" {
		t.Fatalf("mutate target negates %q, want Human", got)
	}
	if !tbl.Cards[hi].Subtypes.Has(int(f.NotSub - 1)) {
		t.Fatalf("%s lost the Human subtype in the matchup", human)
	}
}
