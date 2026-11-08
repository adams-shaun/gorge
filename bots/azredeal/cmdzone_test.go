package azredeal

import (
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestClosedSeatSearchesTheCommanderZone pins the paired bot's commander-zone
// kind: a ClosedFactory seat asks for an Env at the choice (CR 903.9), while
// the registered hosted seat keeps answering it by rule.
func TestClosedSeatSearchesTheCommanderZone(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	d, err := testutil.LoadRepoDeck(reg, "foundations-tramplesaurus-rex")
	if err != nil {
		t.Fatal(err)
	}
	belief, err := searchprobe.NewBelief([][]*cards.Card{d})
	if err != nil {
		t.Fatal(err)
	}
	closed, err := ClosedFactory(belief, 1)(bots.Options{Seed: 1})
	if err != nil {
		t.Fatal(err)
	}
	cz := &decision.Decision{Kind: decision.KCommanderZone, Min: 1, Max: 1, Options: []decision.Option{
		{Index: 0, Kind: "command_zone"},
		{Index: 1, Kind: "leave"},
	}}
	if !closed.(bots.EnvSeat).WantsEnv(cz) {
		t.Error("a ClosedFactory seat does not search the commander-zone choice")
	}

	hosted, err := bots.New(Policy, bots.Options{Seed: 5})
	if err != nil {
		t.Fatal(err)
	}
	if hosted.(bots.EnvSeat).WantsEnv(cz) {
		t.Error("the registered hosted seat searches the commander-zone choice; only the paired bot may")
	}
}
