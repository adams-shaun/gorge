package kgen

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The standard-meta pool (the 500 cards of 60 MTGGoldfish Standard decks) builds as
// one table: its 146 subtypes are past the old 128-bit masks, so the widening to
// kop.MaxSubtypes is what lets it build, and 53 of its cards stay refused.
func TestStdMetaPoolBuilds(t *testing.T) {
	cards := tableCorpus(t)
	p, _, prev, err := readPool(os.DirFS("../.."), "standard-meta", cards)
	if err != nil {
		t.Fatal(err)
	}
	if len(prev) == 0 {
		t.Fatal("standard-meta has no ids.tsv; run make dirac-tables")
	}
	d, cv, _, err := tableOf(p, "standard-meta", prev)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d cards, %d supported, %d subtypes", cv.cards, cv.cardsOK, len(d.Subtypes))
	if cv.cards != 500 {
		t.Errorf("%d cards, want 500", cv.cards)
	}
	if len(d.Subtypes) <= 128 {
		t.Errorf("%d subtypes: the pool no longer needs more than the old 128", len(d.Subtypes))
	}
	if len(d.Subtypes) > kop.MaxSubtypes {
		t.Errorf("%d subtypes, a table's masks hold %d", len(d.Subtypes), kop.MaxSubtypes)
	}
	if cv.cardsOK != 447 {
		t.Errorf("%d cards supported, want 447 (53 refused)", cv.cardsOK)
	}
	refused := map[string]bool{}
	for _, r := range cv.refused {
		refused[r.Card] = true
	}
	if len(refused) != 53 {
		t.Errorf("%d distinct refused cards, want 53", len(refused))
	}
}
