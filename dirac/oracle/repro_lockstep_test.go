package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// synthReproLockstep replays a recorded game on a synthetic pool's table, past its
// script, and fails on any divergence. A refused card plays: the fixture names the
// game that found a difference, whatever the list says today.
func synthReproLockstep(t *testing.T, file, pool string, cfg Config) {
	t.Helper()
	r, err := ReadRepro("testdata/" + file + ".repro.json")
	if err != nil {
		t.Fatal(err)
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, pool)
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
	if res := Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc); res.Div != nil {
		t.Fatalf("divergence at %d: %s %s\n gorge %v\n dirac %v\n %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.RefOptions, res.Div.KerOptions, res.Div.Diff)
	}
}

// A trigger exiling the object that fired it, then returning it: Salvation Swan and a
// Bird entering, S.H.I.E.L.D. Flying Car's trigger copied onto the crewed Car. Both
// engines return the object at the end step, and its entering trigger follows.
func TestDelayedReturnOfTheObjectItsTriggerExiledLockstep(t *testing.T) {
	t.Parallel()
	autopay := Config{GorgeAutopaySurface: true}
	synthReproLockstep(t, "swan_exiles_its_bird", "std-blb", autopay)
	synthReproLockstep(t, "flying_car_exiles_itself", "std-msh", autopay)
}

// Zurgo's Vanguard is */3, and Fresh Start takes its power's CDA away: it stays a -5/3
// on the battlefield, whose toughness the lost ability did not set.
func TestLostCDALeavesThePrintedToughnessLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "lost_cda_toughness", "std-tdm", Config{GorgeAutopaySurface: true})
}
