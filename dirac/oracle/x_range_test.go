package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The largest X a seat can announce or pay is bounded by what its sources make
// (CR 107.3, 601.2b, 118.3). A source that makes no mana right now adds nothing
// to that bound in either engine: Command Tower with no commander has no colour
// identity to produce (CR 903.4f; gorge's potentialIdentityEmpty), so the
// kernel's potential-mana walk must not admit its printed any-colour ability as
// an open source and offer Decree of Justice's cycling "you may pay {X}" up to
// 64 where gorge offers what the lands pay. The trigger's own pay election
// follows on the same bound: a gate land (Sea Gate's "{U} or the chosen
// colour") counts as one mana of those colours in gorge's payment bound, as the
// kernel counts it, not as an open source.

var xRangeWatch = map[string]bool{
	"Decree of Justice": true, "Tyvar's Stand": true, "Debt to the Deathless": true, "Subterranean Tremors": true,
}

var xRangeWB = []xRangeEntry{
	{"Decree of Justice", 4}, {"Debt to the Deathless", 4}, {"Hill Giant", 4}, {"Grizzly Bears", 4},
	{"Command Tower", 3}, {"Sea Gate", 3}, {"Plains", 9}, {"Swamp", 9},
}

var xRangeGR = []xRangeEntry{
	{"Tyvar's Stand", 4}, {"Subterranean Tremors", 4}, {"Llanowar Elves", 4}, {"Grizzly Bears", 4},
	{"Command Tower", 3}, {"Sea Gate", 3}, {"Forest", 9}, {"Mountain", 9},
}

type xRangeEntry struct {
	name  string
	count int
}

func xRangeDeck(es []xRangeEntry) []cardset.Entry {
	var d []cardset.Entry
	for _, e := range es {
		d = append(d, cardset.Entry{Name: e.name, Count: e.count})
	}
	return d
}

// TestXRangeLockstep plays the four X spells beside Command Tower and Sea Gate
// to the end of 150 games in lockstep. It asks that no game diverges and that
// each X spell was cast.
func TestXRangeLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	decks := []fixtureDecks{
		{xRangeDeck(xRangeWB), xRangeDeck(xRangeGR)},
		{xRangeDeck(xRangeGR), xRangeDeck(xRangeWB)},
	}
	run := runFixtureLockstep(t, "xrange", decks, xRangeWatch, 150, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v, first %q", run.games, run.divs, run.stalls, run.casts, run.first)
	for name := range xRangeWatch {
		if run.casts[name] == 0 {
			t.Errorf("%s was never cast: its X ask was not reached", name)
		}
	}
}
