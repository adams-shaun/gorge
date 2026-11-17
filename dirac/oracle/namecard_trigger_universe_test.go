package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestTriggerBodyNameAskerNarrowsTheNameUniverse: a deck whose only name ask
// is a trigger's Execute$ body (Cheering Fanatic: "Whenever it attacks, choose
// a card name") makes the matchup name only its own cards, the set the kernel
// lists (Table.NameCards). Left on the registry's whole universe, gorge offered
// every corpus name where the kernel offered the decks' (dr-mbc-abaa pair 3).
func TestTriggerBodyNameAskerNarrowsTheNameUniverse(t *testing.T) {
	t.Parallel()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range [2][2]string{{"Cheering Fanatic", "Mountain"}, {"Forest", "Forest"}} {
		for i := 0; i < 40; i++ {
			c, ok := reg.Lookup(d[i%2])
			if !ok {
				t.Fatalf("%s not in the corpus", d[i%2])
			}
			decks[s] = append(decks[s], c)
		}
	}
	// precondition: the ask sits in the card's trigger, not in an ability or
	// a replacement the older scan already reads
	fan, _ := reg.Lookup("Cheering Fanatic")
	for _, f := range fan.Faces {
		if len(f.Repls) != 0 || len(f.Abilities) != 0 || len(f.Triggers) == 0 {
			t.Fatalf("precondition: Cheering Fanatic holds %d replacements, %d abilities, %d triggers", len(f.Repls), len(f.Abilities), len(f.Triggers))
		}
	}
	m, err := NewMatchup(reg, [2]string{"trigname-a", "trigname-b"}, decks, 1)
	if err != nil {
		t.Fatal(err)
	}
	if m.NameUniverse == nil {
		t.Fatal("NewMatchup left a trigger-body name asker's matchup on the registry universe")
	}
}
