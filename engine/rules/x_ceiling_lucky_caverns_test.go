package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 601.2b/106.1: an untapped Gemstone Caverns with a luck counter makes one
// mana of any colour, so with {U} already floating, Breakthrough ({X}{U}) is
// payable at X = 1 and the announcement must offer it. The cast-window reach
// priced the lucky Caverns as nothing (its Any arm is not a free plain
// literal and the probe read only the root's {C}, whose gate had failed).
func TestXCeilingCountsLuckyGemstoneCaverns(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	spell := mustCorpusCard(t, reg, "Breakthrough")
	caverns := mustCorpusCard(t, reg, "Gemstone Caverns")
	for _, lucky := range []bool{true, false} {
		e := counterHands(t, []*cards.Card{spell}, nil, []*cards.Card{caverns}, nil)
		id := battlefieldIDOrFail(t, e, "Gemstone Caverns")
		if lucky {
			e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "LUCK", Amount: 1})
		}
		e.G.Players[0].Pool[state.MU] = 1
		e.askPriority(0)
		spellID := handObj(t, e, 0, "Breakthrough")
		submitChoices(t, e, passToCast(t, e, spellID))
		d := e.Pending()
		if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "x" {
			t.Fatalf("lucky=%v: no X announcement ask pending: %+v", lucky, d)
		}
		if !xOptionHas(d.Options, 0) || !xOptionHas(d.Options, 1) {
			t.Fatalf("lucky=%v: Breakthrough with {U} floating and an untapped Gemstone Caverns offers %+v, want X = 0 and 1", lucky, d.Options)
		}
	}
}
