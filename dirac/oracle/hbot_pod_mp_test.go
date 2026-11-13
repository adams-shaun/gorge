//go:build dirac_mp

package oracle

import (
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/search/hbot"
)

// TestHBotParityFourSeat is TestHBotParity in four-seat FDN pods
// (HBotParityPod): gorge's heuristic autopay bot chooses at every seat and
// the native port is asked every decision in shadow. Attacks pick a
// defender among three opponents, blocks read only the attackers coming
// at the blocker, and the lethal and life reads fold over every opponent.
// Every compared decision agrees.
func TestHBotParityFourSeat(t *testing.T) {
	t.Parallel()

	c, pool := testCorpus(t)
	type tally struct{ n, agree int }
	kinds := map[string]*tally{}
	mk := func(tbl *kop.Table) KernelBot { return hbot.New(HBotFacts(c.Reg, tbl, nil)) }
	ps := Pairings(pool, 16, 13)
	pods := 0
	for i := 0; i+1 < len(ps) && pods < 8; i += 2 {
		names := []string{ps[i].A, ps[i].B, ps[i+1].A, ps[i+1].B}
		res, err := HBotParityPod(c, names, ps[i].Seed, Config{GorgeAutopaySurface: true, PassElision: true}, 20000, mk, HBotParityOptions{})
		if err != nil {
			t.Logf("pod %v: %v", names, err)
			continue
		}
		if len(res.Recs) == 0 {
			t.Logf("pod %v: nothing compared (%s)", names, res.Res.Stall)
			continue
		}
		pods++
		if res.Res.Div != nil {
			// the rules divergence is the lockstep gates' to find; the
			// decisions before it compare
			t.Logf("pod %v: rules divergence after %d decisions: %s", names, len(res.Recs), res.Res.Div)
		}
		for _, r := range res.Recs {
			if r.Unoffered {
				continue
			}
			k := kinds[r.Kind]
			if k == nil {
				k = &tally{}
				kinds[r.Kind] = k
			}
			k.n++
			if r.Agree {
				k.agree++
			} else {
				t.Logf("pod %d: %s", pods, r.Where)
			}
		}
	}
	if pods < 6 {
		t.Fatalf("%d pods played", pods)
	}
	for _, k := range []string{"priority", "attackers", "blockers", "target"} {
		if x := kinds[k]; x == nil || x.n < 20 {
			t.Fatalf("%s: too few decisions compared: %+v", k, x)
		}
	}
	names := make([]string, 0, len(kinds))
	for k := range kinds {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		x := kinds[k]
		t.Logf("%-16s %5d %.4f", k, x.n, float64(x.agree)/float64(x.n))
		if x.agree != x.n {
			t.Errorf("%s: %d of %d decisions disagree", k, x.n-x.agree, x.n)
		}
	}
}
