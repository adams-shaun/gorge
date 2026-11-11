package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestStdMetaReprosLockstep replays the standard-meta hunt's NStack divergences
// (four mechanisms): a prepared copy's cast reads as cast (Appa's cast-from-exile
// trigger), the delayed earthbend return keeps its stack entry while the shock land
// asks, and a Treasure's Sacrificed trigger is placed before priority and survives
// the cast it was queued in being reversed. Each repro runs on the repro's own
// surface; a missing generated table fails rather than skips.
func TestStdMetaReprosLockstep(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	set, err := cardset.ByName("standard-meta")
	if err != nil {
		t.Fatalf("no standard-meta table (make dirac-tables): %v", err)
	}
	set = cardset.AllowUnverified(set)
	for _, name := range []string{
		"stdmeta_appa_cast_from_exile_s14p32",
		"stdmeta_appa_cast_from_exile_s15p14",
		"stdmeta_appa_cast_from_exile_s16p2",
		"stdmeta_appa_cast_from_exile_s19p47",
		"stdmeta_appa_cast_from_exile_s25p34",
		"stdmeta_earthbend_shock_s14p23",
		"stdmeta_earthbend_shock_s15p47",
		"stdmeta_earthbend_shock_s18p17",
		"stdmeta_earthbend_shock_s22p0",
		"stdmeta_earthbend_shock_s22p16",
		"stdmeta_earthbend_shock_s29p4",
		"stdmeta_earthbend_shock_s33p29",
		"stdmeta_earthbend_shock_s6p4",
		"stdmeta_earthbend_shock_s8p2",
		"stdmeta_sacrificed_pending_s13p16",
		"stdmeta_sacrificed_reversed_s21p8",
	} {
		t.Run(name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			m, err := r.Matchup(reg)
			if err != nil {
				t.Fatal(err)
			}
			tbl, _, _, err := set.Matchup(reproEntries(r.Cards[0]), reproEntries(r.Cards[1]))
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
			script := &ScriptPolicy{Actions: r.Actions}
			res := Lockstep(m, r.Seed, k, script, rc)
			if res.Div != nil {
				t.Fatalf("divergence at %d of %d: %s %s %v", res.Div.At, len(r.Actions), res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if len(res.Actions) < len(r.Actions) {
				t.Errorf("played %d of the %d recorded actions", len(res.Actions), len(r.Actions))
			}
		})
	}
}

func reproEntries(names []string) []cardset.Entry {
	es := make([]cardset.Entry, len(names))
	for i, n := range names {
		es[i] = cardset.Entry{Name: n, Count: 1}
	}
	return es
}
