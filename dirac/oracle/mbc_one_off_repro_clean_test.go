package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// oneOffReproClean replays the recorded "one-off" game testdata/repro/<name>.repro.json
// in lockstep over the table built from the recording's own decks (the way
// checkRefusedRepro builds it) and fails on any divergence or stall.
func oneOffReproClean(t *testing.T, name string) {
	t.Helper()
	r, res, _ := oneOffReproRun(t, name)
	if !res.Clean() {
		t.Fatalf("%s: the recorded game diverges (%d decisions): %v", name, res.Decisions, res.Div)
	}
	stallCheck(t, r, res, false)
}

// oneOffReproRun is oneOffReproClean's replay: the recording, its lockstep
// result and the script policy that served it.
func oneOffReproRun(t *testing.T, name string) (*Repro, Result, *ScriptPolicy) {
	t.Helper()
	r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.Matchup(synthReg(t))
	if err != nil {
		t.Fatal(err)
	}
	set := cardset.Raw(printedDelveSet(t, r.Cards))
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
	return r, Lockstep(m, r.Seed, k, sp, rc), sp
}
