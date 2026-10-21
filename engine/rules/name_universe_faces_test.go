package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCorpusNameUniverseHasEveryNameableFace is CR 201.4b-f, 712.19, 715.5,
// 720.5 and 722.5 over the real corpus: the second half of a split card, a
// transforming and a modal DFC's back face, a flip card's alternative name,
// a meld pair's combined back face, an Adventure, an Omen and a preparation
// name are all nameable; and the imaged universe answers exactly what the
// plain card-slice universe does.
func TestCorpusNameUniverseHasEveryNameableFace(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	names := reg.Universe().Names()
	for _, want := range []string{"Ice", "Insectile Aberration", "Tibalt, Cosmic Impostor", "Kenzo the Hardhearted",
		"Brisela, Voice of Nightmares", "Stomp", "Skimming Strike", "Heroic Stanza"} {
		if _, ok := slices.BinarySearch(names, want); !ok {
			t.Errorf("name universe lacks %q", want)
		}
	}
	if _, ok := slices.BinarySearch(names, "Alora, Cheerful Mastermind"); ok {
		t.Error("name universe offers a specialize face (Alora, Cheerful Mastermind)")
	}
	if got := cards.UniverseOf(reg.AllCards()).Names(); !slices.Equal(got, names) {
		t.Errorf("imaged universe names (%d) differ from the card-slice universe's (%d)", len(names), len(got))
	}
}
