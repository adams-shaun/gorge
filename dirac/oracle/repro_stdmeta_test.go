package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestStdMetaReprosLockstep replays the standard-meta hunt's NStack divergences
// (five mechanisms): a prepared copy's cast reads as cast (Appa's cast-from-exile
// trigger), the delayed earthbend return keeps its stack entry while the shock land
// asks, a Treasure's Sacrificed trigger is placed before priority and survives
// the cast it was queued in being reversed, a prepared creature entering beside
// an opponent's enters-tapped replacement asks the CR 616.1 order, and an
// activation's fixed PayLife<N> is rechecked after the window's life-paying mana
// ability left too little life, so it reverses instead of paying into negative
// life (CR 119.4, 733.1). Each repro runs on the repro's own
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
		"stdmeta_pool_unnamed_colours_s31p34",
		"stdmeta_pool_unnamed_colours_s51p31",
		"stdmeta_pool_unnamed_colours_s79p32",
		"stdmeta_pool_unnamed_colours_s84p9",
		"stdmeta_web_watch_beside_grant_s37p11",
		"stdmeta_web_watch_beside_grant_s41p37",
		"stdmeta_web_watch_beside_grant_s49p43",
		"stdmeta_web_watch_beside_grant_s58p46",
		"stdmeta_web_watch_beside_grant_s71p25",
		"stdmeta_web_watch_beside_grant_s72p25",
		"stdmeta_verge_ability0_first_s19p46",
		"stdmeta_verge_ability0_first_s60p6",
		"stdmeta_verge_ability0_first_s76p19",
		"stdmeta_verge_ability0_first_s90p18",
		"stdmeta_x_witness_unspendable_census_s11p1",
		"stdmeta_node_limit_verge_first_s87p22",
		"stdmeta_sacrificed_pending_s13p16",
		"stdmeta_sacrificed_reversed_s21p8",
		"stdmeta_prepared_beside_enters_tapped_s27p43",
		"stdmeta_prepared_beside_enters_tapped_s28p23",
		"stdmeta_prepared_beside_enters_tapped_s34p46",
		"stdmeta_prepared_beside_enters_tapped_s54p24",
		"stdmeta_prepared_beside_enters_tapped_s63p43",
		"stdmeta_prepared_beside_enters_tapped_s67p34",
		"stdmeta_prepared_beside_enters_tapped_s68p22",
		"stdmeta_prepared_beside_enters_tapped_s68p46",
		"stdmeta_prepared_beside_enters_tapped_s69p19",
		"stdmeta_prepared_beside_enters_tapped_s90p11",
		"stdmeta_prepared_beside_enters_tapped_s97p33",
		"stdmeta_life_cost_below_zero_s54p16",
		"stdmeta_life_cost_below_zero_s73p37",
		"stdmeta_life_cost_below_zero_s83p36",
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
			tbl, _, _, err := set.Matchup(oneOfEachEntries(r.Cards[0]), oneOfEachEntries(r.Cards[1]))
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
