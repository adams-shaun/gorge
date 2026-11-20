package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Pump Duration$ AsLongAsControl lockstep: a keyword grant that gorge
// anchors to its host (the resolving ability's source) and ends when the host
// changes controller, phases out or leaves (expireAsLongAsControl; the
// kernel's WhileHost + AsLongAsControl override record). Act of Treason makes
// a host control change reachable and Shock makes a host leave.
var kpumpControlDeck = []cardset.Entry{
	{Name: "Aegis Angel", Count: 4},
	{Name: "Kyodai, Soul of Kamigawa", Count: 4},
	{Name: "Thancred Waters", Count: 4},
	{Name: "Possession Engine", Count: 4},
	{Name: "There and Back Again", Count: 4},
	{Name: "Tale of Tinúviel", Count: 4},
	{Name: "Act of Treason", Count: 4},
	{Name: "Shock", Count: 4},
	{Name: "Plains", Count: 3},
	{Name: "Island", Count: 3},
	{Name: "Mountain", Count: 2},
}

func TestPumpControlLockstep(t *testing.T) {
	t.Parallel()
	corpusDir, err := cardset.CorpusDir()
	if err != nil {
		t.Skipf("no card corpus: %v", err)
	}
	reg, err := cards.SharedCorpus(corpusDir)
	if err != nil {
		t.Fatal(err)
	}
	var names [2][]string
	var gd [2][]*cards.Card
	for s := range gd {
		for _, e := range kpumpControlDeck {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			names[s] = append(names[s], e.Name)
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(gd[s]))
		}
	}
	set := printedDelveSet(t, names)
	decks := [2][]cardset.Entry{kpumpControlDeck, kpumpControlDeck}
	const seeds = 40
	clean, casts, div := lockstepSeeds(t, reg, set, "pumpcontrol", decks, gd, 1, seeds, true, nil, nil, castFirstPolicy)
	t.Logf("%d seeds, %d clean, %d divergent; %d casts/activations", seeds, clean, div, casts)
	if div != 0 {
		t.Fatalf("%d divergent games", div)
	}
	if casts == 0 {
		t.Fatal("no cast or activation in the games; the lockstep proves nothing")
	}
}
