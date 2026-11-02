package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Gathering Stone chosen-type lockstep (CR 601.2f, 701.20a, 205.3e): the
// as-enters creature type is read by the may-reveal's RevealValid$
// Card.ChosenType and by the ReduceCost Card.ChosenType static, both off the
// source permanent. ktoplibPolicy takes the may-reveal three times in four and the
// pinned gorge does not shuffle after it, so the ask is exercised either way --
// the stone chooses among the matchup's whole subtype vocabulary and a matching
// top card is rare, so the count of asks stays low.
var gatheringStoneDecks = [2][]cardset.Entry{
	{
		{Name: "Gathering Stone", Count: 12}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Forest", Count: 10}, {Name: "Island", Count: 10},
	},
	{
		{Name: "Gathering Stone", Count: 12}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Forest", Count: 10}, {Name: "Island", Count: 10},
	},
}

func TestGatheringStoneLockstep(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("peekreveal")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range gatheringStoneDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(gatheringStoneDecks[0], gatheringStoneDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	st := &ktoplibStats{bySource: map[string]int{}}
	games, divs, stalls := 0, 0, 0
	for seed := uint64(1); seed <= 256; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"gs-a", "gs-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, ktoplibPolicy(seed, st), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 20})
		CloseSide(k)
		games++
		if res.Stall != "" && res.Stall != "turns" {
			stalls++
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			divs++
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games, %d divergences, %d stalls; may-reveal asks answered yes %d, no %d; defined_library_optional asks answered yes %d, no %d, by source %v",
		games, divs, stalls, st.reveal[1], st.reveal[0], st.ask[1], st.ask[0], st.bySource)
	if st.reveal[0]+st.reveal[1] < 1 {
		t.Errorf("%d may-reveal asks, want at least 1", st.reveal[0]+st.reveal[1])
	}
	if st.ask[1] < 20 {
		t.Errorf("%d defined_library_optional asks answered yes, want at least 20", st.ask[1])
	}
	if st.ask[0] < 10 {
		t.Errorf("%d defined_library_optional asks answered no, want at least 10", st.ask[0])
	}
	if n := st.bySource["Gathering Stone"]; n < 20 {
		t.Errorf("%d defined_library_optional asks from Gathering Stone, want at least 20", n)
	}
}
