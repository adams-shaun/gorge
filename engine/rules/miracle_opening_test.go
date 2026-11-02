package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The cards dealt into an opening hand are drawn, but they are not "the first
// card you've drawn this turn": no turn has begun (CR 103.5, 702.94a). A miracle
// card on top of the shuffled library gets no offer before turn 1.
func TestMiracleNotOfferedOnTheOpeningHand(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	terminus := lookup(t, reg, "Terminus")
	deck := make([]*cards.Card, 60)
	for i := range deck {
		deck[i] = terminus
	}
	for _, mulligans := range []int{0, 2} {
		cfg := seatZeroStart(Config{Seed: 7, Names: []string{"a", "b"}, Tokens: reg.Tokens,
			Decks: [][]*cards.Card{deck, deck}, Mulligans: mulligans})
		e := New(cfg)
		e.Advance()
		for d := e.Pending(); d != nil && e.G.Turn <= 1; d = e.Pending() {
			if d.Kind == decision.KTriggerOptional {
				t.Fatalf("mulligans %d: miracle offered at turn %d, before any turn's draw: %q", mulligans, e.G.Turn, d.Prompt)
			}
			if d.Kind == decision.KPriority {
				passOnce(t, e)
			} else {
				submitChoices(t, e, d.Options[0].Index)
			}
		}
	}
}
