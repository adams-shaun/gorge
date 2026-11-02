package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// poolReproLockstep replays a recorded game on a deck pool's table, past its
// script, and fails on any divergence. An unverified card plays: the fixture names
// the game that found a difference.
func poolReproLockstep(t *testing.T, file, pool string, cfg Config) {
	t.Helper()
	r, err := ReadRepro("testdata/" + file + ".repro.json")
	if err != nil {
		t.Fatal(err)
	}
	reg := synthReg(t)
	set, err := cardset.RawByName(pool)
	if err != nil {
		t.Skipf("no %s table (make dirac-tables): %v", pool, err)
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
	k, err := SetSide(set, tbl, cfg)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	rc := RunConfig{Config: cfg, MaxDecisions: len(r.Actions) + 200}
	script := &ScriptPolicy{Actions: r.Actions}
	res := Lockstep(m, r.Seed, k, script, rc)
	if script.Skipped != 0 {
		t.Errorf("%d scripted actions were not offered", script.Skipped)
	}
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s\n gorge %v\n dirac %v\n %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.RefOptions, res.Div.KerOptions, res.Div.Diff)
	}
}

// Heap Gate's "{1}, {T}, Tap an untapped Gate": gorge arms the mana plan before the
// Gate is elected, and keeps it unless the election takes one of its sources, so the
// {1} is paid by the source the plan chose with every Gate still untapped.
func TestHeapGatePlanSurvivesTheElectionLockstep(t *testing.T) {
	t.Parallel()
	poolReproLockstep(t, "heap_gate_plan_survives_election", "pauper", Config{})
}

// A dry plan for an activation, on a table that holds a sacrifice-only mana source
// (Makeshift Munitions' pool), found its plan: the retry for a census that proved
// insufficient has nothing to add, and its empty plan did not replace the real one.
func TestDryPlanBesideASacOnlySourceLockstep(t *testing.T) {
	t.Parallel()
	poolReproLockstep(t, "heap_gate_dry_plan_with_sac_only_source", "pauper", Config{})
}
