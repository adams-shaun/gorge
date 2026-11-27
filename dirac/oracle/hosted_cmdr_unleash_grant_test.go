package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestHostedCmdrGrantedUnleashMutation: with the kernel's granted-Unleash
// block gate disabled (dirac.DebugNoGrantedUnleash, TestDemoRiotMutation's
// shape), the Pack Leader Probe match diverges from gorge on some seed —
// gorge still refuses the granted hound as a blocker while it carries its
// +1/+1 counter (combat/block.go's derived HasKW read), the kernel offers it,
// and the lockstep catches the extra option. It fails unless the grant read
// is what makes the green lockstep agree, so
// TestHostedCmdrKeywordsCostsLockstep/hellhound_probe_granted_unleash_block
// cannot pass by never reaching the gate.
func TestHostedCmdrGrantedUnleashMutation(t *testing.T) {
	fixtures := map[string]string{
		"Pack Leader Probe":   packLeaderProbeScript,
		"Counter Hound Probe": counterHoundProbeScript,
	}
	dir := filters11Corpus(t, fixtures, "Swamp", "Forest", "Grizzly Bears", "Mountain")
	reg, err := cards.SharedCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	decks := [2][]string{
		append(deckOf(8, "Grizzly Bears"), deckOf(32, "Forest")...),
		append(append(deckOf(12, "Pack Leader Probe"), deckOf(12, "Counter Hound Probe")...),
			deckOf(16, "Mountain")...),
	}
	var ds [2][]*cards.Card
	for s, names := range decks {
		for _, n := range names {
			card, ok := reg.Lookup(n)
			if !ok {
				t.Fatalf("%s not in the corpus", n)
			}
			ds[s] = append(ds[s], card)
		}
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	set := printedDelveSetIn(t, dir, decks)
	tbl, _, _, err := set.Matchup(entries(decks[0]), entries(decks[1]))
	if err != nil {
		t.Fatal(err)
	}
	c := hostedCmdrCase{carrier: "Pack Leader Probe", seats: [2]string{"hck-hound-a", "hck-hound-b"}}
	cfg := Config{}
	dirac.DebugNoGrantedUnleash = true
	defer func() { dirac.DebugNoGrantedUnleash = false }()
	divs := 0
	for seed := uint64(1); seed <= 60; seed++ {
		m, err := NewMatchup(reg, c.seats, ds, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, hostedCmdrKwCostsPolicy(m, c, seed), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 40})
		if res.Div != nil {
			divs++
		}
		CloseSide(k)
	}
	if divs == 0 {
		t.Fatal("DebugNoGrantedUnleash: no divergence in 60 games")
	}
	t.Logf("mutation diverged in %d of 60 games", divs)
}
