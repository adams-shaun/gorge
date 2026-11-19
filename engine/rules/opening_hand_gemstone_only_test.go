package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestGemstoneOnlyHandExilePickIsSkipped: a seat whose FINAL opening hand holds
// only Gemstone Caverns (mulliganed down to a single card) has no other card to
// exile, so the mandatory opening_exile pick has no legal answer. The opening
// round must skip that pick -- the Caverns enter with their luck counter, the
// hand stays empty -- instead of parking on an ask nothing can answer and
// never reaching turn one.
func TestGemstoneOnlyHandExilePickIsSkipped(t *testing.T) {
	t.Parallel()
	gem := corpusAlternativeCard(t, "Gemstone Caverns")
	fill := card(t, "Name:Filler\nTypes:Basic Land\nOracle:x\n")
	gemDeck := make([]*cards.Card, 40)
	for i := range gemDeck {
		gemDeck[i] = gem
	}
	fillDeck := make([]*cards.Card, 40)
	for i := range fillDeck {
		fillDeck[i] = fill
	}
	// A four-seat game: the free multiplayer mulligan keeps one card on the
	// table after seven mulligans (7 - (7-1)), so a Gemstone-only hand is
	// reachable; with two seats the seventh mulligan empties the hand entirely
	// and no opening effect is offered at all.
	for seed := uint64(1); seed < 500; seed++ {
		e := New(Config{Seed: seed, Names: []string{"a", "b", "c", "d"},
			Decks: [][]*cards.Card{fillDeck, gemDeck, fillDeck, fillDeck}, Mulligans: 7})
		e.Advance()
		start := e.mulligan.seats[0]
		if start == 1 {
			continue // !PlayFirst: the starting seat cannot use the Caverns
		}
		// Mulligan seat 1 down to a one-card hand: seven mulligans, then keep
		// (the allowance is spent), then bottom six of the seven redrawn cards.
		for d := e.Pending(); d != nil && d.Kind == decision.KMulligan; d = e.Pending() {
			i := 0
			for j, s := range e.mulligan.seats {
				if s == 1 {
					i = j
				}
			}
			choices := []int{0}
			if !e.mulligan.bottom && d.Player == 1 && e.mulligan.taken[i] < e.mulligan.limit {
				choices = []int{1}
			} else if e.mulligan.bottom && d.Player == 1 {
				choices = []int{0, 1, 2, 3, 4, 5}
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
				t.Fatal(err)
			}
		}
		d := e.Pending()
		if d == nil || d.Options[0].Kind != "opening_yes" || d.Player != 1 {
			continue
		}
		if n := len(e.G.Zone(state.ZHand, 1)); n != 1 {
			t.Fatalf("precondition: seat 1's final hand holds %d cards, want 1 (Gemstone Caverns only)", n)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
			t.Fatal(err)
		}
		if e.opening.exile != 0 {
			t.Fatalf("opening round parked on an opening_exile pick with no legal options (hand %v, exile %v)",
				e.G.Zone(state.ZHand, 1), e.opening.exile)
		}
		e.Advance()
		if e.G.Turn == 0 {
			t.Fatal("the opening round never advanced to turn one")
		}
		var caverns *state.Object
		for _, id := range e.G.Zone(state.ZBattlefield, 1) {
			if e.G.Obj(id).Face().Name == "Gemstone Caverns" {
				caverns = e.G.Obj(id)
			}
		}
		if caverns == nil || caverns.Counter("LUCK") != 1 {
			t.Fatalf("Gemstone Caverns did not enter with its luck counter: %+v", caverns)
		}
		return
	}
	t.Fatal("could not construct a Gemstone-only opening hand for a non-starting seat")
}
