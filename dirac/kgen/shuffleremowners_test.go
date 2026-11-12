package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Shuffle Defined$ RememberedOwner lowers only after the Remember writes the
// kernel honors: Elixir of Immortality's self move from the battlefield and
// graveyard sweep lower; Serene Remembrance (the spell moved from the stack,
// targeted graveyard cards) stays refused by name, since the kernel's
// remembered set would miss both.
func TestShuffleRememberedOwnerLowersOnlyHonoredWrites(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	lower := func(name string) *lCard {
		card, ok := reg.Lookup(name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", name)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		return lowerCard(p, card.Faces[0].Name, card, false)
	}
	if lc := lower("Elixir of Immortality"); len(lc.Refusals) != 0 {
		t.Fatalf("Elixir of Immortality refusals = %v, want none", lc.Refusals)
	}
	lc := lower("Serene Remembrance")
	if len(lc.Refusals) != 1 || !strings.Contains(lc.Refusals[0].Reason, "RememberedOwner") {
		t.Fatalf("Serene Remembrance refusals = %v, want exactly the RememberedOwner reason", lc.Refusals)
	}
}
