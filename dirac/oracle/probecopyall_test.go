package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

func TestProbeCopyAllRepro(t *testing.T) {
	if os.Getenv("PROBE_COPYALL") == "" {
		t.Skip("set PROBE_COPYALL=1")
	}
	if os.Getenv("PROBE_ONLY_RANDOM") == "1" {
		t.Skip("script variant skipped")
	}
	reg := synthReg(t)
	raw, err := cardset.SynthByName("std-otj")
	if err != nil {
		t.Fatal(err)
	}
	set := cardset.AllowUnverified(raw)
	reproPath := "copy_all_sweep_parks_on_aura_host_ask.repro.json"
	if p := os.Getenv("PROBE_REPRO"); p != "" {
		reproPath = p
	}
	r, err := ReadRepro(filepath.Join("testdata", "repro", reproPath))
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 200
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	sp := &ScriptPolicy{Actions: r.Actions}
	res := Lockstep(m, r.Seed, k, sp, rc)
	t.Logf("script: stall=%q div=%v decisions=%d skipped=%d exhausted=%v", res.Stall, res.Div, res.Decisions, sp.Skipped, sp.Exhausted())
	_, files, err := SynthPool(reg, set, 1, SynthDecks)
	if err != nil {
		t.Fatal(err)
	}
	cc := &Corpus{Reg: reg, decks: map[string][]*cards.Card{}}
	cc.AddDecks(files)
	m2, tbl2, err := SetMatchup(set, cc, r.Decks[0], r.Decks[1], r.Mulligans)
	if err != nil {
		t.Fatal(err)
	}
	k2, err := SetSide(set, tbl2, rc.Config)(m2, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k2)
	res2 := Lockstep(m2, r.Seed, k2, RandomPolicy(r.Seed), rc)
	t.Logf("random: stall=%q div=%v decisions=%d", res2.Stall, res2.Div, res2.Decisions)
	if res2.Actions != nil {
		la := res2.Actions
		n := min(len(la), len(r.Actions))
		first := -1
		for i := 0; i < n; i++ {
			if la[i] != r.Actions[i] {
				first = i
				break
			}
		}
		t.Logf("random vs script: live=%d scripted=%d first-diff=%d", len(la), len(r.Actions), first)
		if first >= 0 && first < len(la) {
			t.Logf("live[%d:]=%v script[%d:]=%v", first, la[first:min(first+6, len(la))], first, r.Actions[first:min(first+6, len(r.Actions))])
		}
	}
}
