package adapter

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestDeckSupportNamesEachCard: on the edh set foundations-wretched-ranks
// plays with nothing listed; with one Swamp swapped for a corpus card edh
// lacks, that card is the one verdict, "not in set", one copy, and the deck
// is refused.
func TestDeckSupportNamesEachCard(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, err := New([]string{"edh"}, reg, dirac.SurfaceAutopay)
	if err != nil {
		t.Skip(err)
	}
	_, d, err := deck.Load(reg, filepath.Join("..", "..", "testkit", "testutil", "decks", "foundations-wretched-ranks.json"))
	if err != nil {
		t.Fatal(err)
	}
	v := e.DeckSupport(d)
	if v.Err != nil || v.Set != "edh" || v.Cards == nil || len(v.Cards) != 0 {
		t.Fatalf("foundations-wretched-ranks on edh: %+v, want set edh, no cards, no error", v)
	}

	edh, err := cardset.ByName("edh")
	if err != nil {
		t.Fatal(err)
	}
	var absent *cards.Card
	for i := range reg.Len() {
		c := reg.Card(i)
		if c == nil || frontName(c) == "" {
			continue
		}
		if _, ok := edh.Lookup(poolName(edh, c)); !ok {
			absent = c
			break
		}
	}
	if absent == nil {
		t.Fatal("every corpus card is in edh")
	}
	at := slices.IndexFunc(d, func(c *cards.Card) bool { return frontName(c) == "Swamp" })
	if at < 0 {
		t.Fatal("foundations-wretched-ranks holds no Swamp")
	}
	swapped := slices.Clone(d)
	swapped[at] = absent
	v = e.DeckSupport(swapped)
	want := CardVerdict{Name: frontName(absent), Count: 1, Status: "not in set"}
	if v.Err == nil || len(v.Cards) != 1 || v.Cards[0] != want {
		t.Fatalf("with %q for a Swamp: %+v, want only %+v and an error", frontName(absent), v, want)
	}
}
