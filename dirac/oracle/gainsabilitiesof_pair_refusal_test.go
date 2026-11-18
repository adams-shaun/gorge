package oracle

import (
	"errors"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestGainsOfOnceAbilityPairRefused: Marvin, Murderous Mimic beside Possessed
// Goat (an "activate only once" ability) is refused when the matchup is built,
// as gainsof naming both cards: gorge counts
// a gained once-limited activation per recipient and foreign object and the
// kernel keeps no such record, so the pair used to stall mid-game (std-dsk
// autopay seed 601411 pair 0). Marvin beside the non-mana ping creatures of
// kmarvinDecks and beside Llanowar Elves (a mana ability, a runtime stall only
// if it reaches the battlefield) still builds.
func TestGainsOfOnceAbilityPairRefused(t *testing.T) {
	corpusDirOrSkip(t)
	deck := func(partner string) []cardset.Entry {
		return []cardset.Entry{{Name: "Marvin, Murderous Mimic", Count: 2}, {Name: partner, Count: 4}, {Name: "Swamp", Count: 20}}
	}
	partners := []string{"Possessed Goat"}
	names := []string{"Island", "Mountain", "Swamp", "Prodigal Sorcerer", "Prodigal Pyromancer", "Marvin, Murderous Mimic"}
	names = append(names, partners...)
	names = append(names, "Llanowar Elves")
	set := printedDelveSet(t, [2][]string{names, names})
	for _, p := range append([]string{"Marvin, Murderous Mimic", "Llanowar Elves"}, partners...) {
		if _, ok := set.Lookup(p); !ok {
			t.Fatalf("precondition: %s not in the set", p)
		}
	}
	d := kmarvinDecks()
	if _, _, _, err := set.Matchup(d[0], d[1]); err != nil {
		t.Fatalf("Marvin beside non-mana ping creatures is refused: %v", err)
	}
	if _, _, _, err := set.Matchup(deck("Llanowar Elves"), deck("Llanowar Elves")); err != nil {
		t.Fatalf("Marvin beside a mana creature is refused: %v", err)
	}
	for _, p := range partners {
		_, _, _, err := set.Matchup(deck(p), deck(p))
		var r *cardset.InteractionRefusal
		if !errors.As(err, &r) {
			t.Fatalf("Marvin vs %s: err %v, want an InteractionRefusal", p, err)
		}
		if r.Reason != "gainsof" || !slices.Contains(r.Cards, "Marvin, Murderous Mimic") || !slices.Contains(r.Cards, p) {
			t.Fatalf("Marvin vs %s: refused %s naming %v, want gainsof naming both", p, r.Reason, r.Cards)
		}
	}
}
