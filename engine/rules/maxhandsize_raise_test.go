package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestRaiseMaxHandSize is CR 402.2: Jin-Gitaxias, Core Augur's
// RaiseMaxHandSize$ -7 reduces each opponent's maximum hand size from seven to
// zero and leaves its controller's at seven.
func TestRaiseMaxHandSize(t *testing.T) {
	reg := searchTestRegistry(t)
	deck := append([]*cards.Card{searchCorpusCard(t, reg, "Jin-Gitaxias, Core Augur")}, mountainDeck(t, 39)...)
	cfg := seatZeroStart(Config{Seed: 9421, Names: []string{"jin", "opp"},
		Decks:  [][]*cards.Card{deck, mountainDeck(t, 40)},
		Tokens: map[string]*cards.Card{}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	searchMoveByName(t, e, "Jin-Gitaxias, Core Augur", state.ZBattlefield)
	if got := e.maxHandSizeFor(1); got != 0 {
		t.Fatalf("opponent's maximum hand size = %d, want 0 (7 - 7)", got)
	}
	if got := e.maxHandSizeFor(0); got != maxHandSize {
		t.Fatalf("controller's maximum hand size = %d, want %d", got, maxHandSize)
	}
}
