package oracle

import (
	"os"
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// Throwaway triage probe (never committed): the fixture matchup under
// castFirstPolicy, a seed sweep with divergence reporting.
func TestZZTriageSweep(t *testing.T) {
	lo, hi := 1, 200
	if v := os.Getenv("ZZ_LO"); v != "" {
		lo, _ = strconv.Atoi(v)
	}
	if v := os.Getenv("ZZ_HI"); v != "" {
		hi, _ = strconv.Atoi(v)
	}
	r, err := ReadRepro("testdata/granted_prefix_tap_trigger_pass.repro.json")
	if err != nil {
		t.Fatal(err)
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-tla")
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
	div := 0
	for seed := lo; seed <= hi; seed++ {
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		gseed := uint64(seed)*7919 + 13
		k, err := SetSide(set, tbl, Config{})(m, gseed)
		if err != nil {
			t.Fatal(err)
		}
		base := castFirstPolicy(gseed)
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if os.Getenv("ZZ_SEQ") != "" {
				println("ZZSEQ", c.Seq)
			}
			return base.Pick(c)
		})
		res := Lockstep(m, gseed, k, pol, RunConfig{Config: Config{}, MaxDecisions: 3000, MaxTurns: 100})
		CloseSide(k)
		if res.Div != nil {
			div++
			t.Logf("ZZDIV seed=%d at=%d what=%s field=%s detail=%s implicates=%v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Detail, res.Div.Implicates(m))
			t.Logf("ZZOPT ref=%v ker=%v", res.Div.RefOptions, res.Div.KerOptions)
		}
	}
	t.Logf("ZZTOTAL div=%d", div)
}
