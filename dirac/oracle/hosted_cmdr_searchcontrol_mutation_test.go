package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestHostedCmdrSearchControlMutation turns each of the kernel's Opposition
// Agent reads off in turn (the search pick's redirect; the found card's
// exile) and requires the lockstep to diverge from gorge on some seed: a
// green TestHostedCmdrSearchControlLockstep is evidence only because each
// read is load-bearing.
func TestHostedCmdrSearchControlMutation(t *testing.T) {
	agentDeck := append(deckOf(14, "Opposition Agent"), deckOf(26, "Swamp")...)
	searchDeck := append(append(deckOf(14, "Search Probe"), deckOf(8, "Grizzly Bears")...), deckOf(18, "Forest")...)
	names := [2][]string{agentDeck, searchDeck}
	dir := filters11Corpus(t, map[string]string{"Search Probe": searchProbeScript}, "Opposition Agent", "Swamp", "Forest", "Grizzly Bears")
	reg, err := cards.SharedCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, ns := range names {
		for _, n := range ns {
			c, ok := reg.Lookup(n)
			if !ok {
				t.Fatalf("%s not in the corpus", n)
			}
			decks[s] = append(decks[s], c)
		}
	}
	entries := func(ns []string) []cardset.Entry {
		es := make([]cardset.Entry, len(ns))
		for i, n := range ns {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	set := printedDelveSetIn(t, dir, names)
	tbl, _, _, err := set.Matchup(entries(names[0]), entries(names[1]))
	if err != nil {
		t.Fatal(err)
	}
	hc := hostedCmdrCase{name: "opposition_agent", carrier: "Opposition Agent", seats: [2]string{"hcs-agent-a", "hcs-agent-b"}, decks: names}
	for _, mut := range []struct {
		name string
		flag *bool
	}{{"redirect off", &dirac.DebugSearchControlOff}, {"exile off", &dirac.DebugFoundSearchExileOff}} {
		*mut.flag = true
		diverged := false
		for seed := uint64(1); seed <= 20 && !diverged; seed++ {
			m, err := NewMatchup(reg, hc.seats, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, Config{})(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, hostedCmdrKwCostsPolicy(m, hc, seed), RunConfig{MaxDecisions: 6000, MaxTurns: 40})
			diverged = res.Div != nil
			CloseSide(k)
		}
		*mut.flag = false
		if !diverged {
			t.Errorf("%s: no seed diverged, so the lockstep cannot see the read", mut.name)
		}
	}
}
