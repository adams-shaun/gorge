package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// stdReproLockstep replays a repro recorded on a Standard set's table, on the
// autopay surface a random-deck sweep plays, and fails on any divergence or stall.
func stdReproLockstep(t *testing.T, path, table string) {
	t.Helper()
	reg := synthReg(t)
	set := cardsettest.Synth(t, table)
	r, err := ReadRepro(path)
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	entries := func(names []string) []cardset.Entry {
		var es []cardset.Entry
		for _, n := range names {
			es = append(es, cardset.Entry{Name: n, Count: 1})
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{GorgeAutopaySurface: true}
	k, err := SetSide(set, tbl, cfg)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.Config = cfg
	rc.MaxDecisions = len(r.Actions) + 200
	res := Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
	CloseSide(k)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v\n gorge %v\n dirac %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.RefOptions, res.Div.KerOptions)
	}
	if res.StallUnmodelled {
		t.Fatalf("dirac stopped at a rule it does not model: %s", res.Stall)
	}
}
