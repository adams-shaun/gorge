package oracle

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestFourSeatBecomesTargetHighObjectLockstep replays two four-seat cmd5 pods
// (full 100-card precons, recorded by kfuzz -kernel cmd5 -seats 4) in which a
// Thunderbreak Regent trigger ("deals 3 damage to that player", Defined$
// TriggeredSourceController, CR 603.2 / 608.2h) is queued by a spell or
// ability whose object is past 255. The kernel kept the targeting object + 1
// in a byte (stackEntry.trigSA, pendingTrig.trigSA), so object 270 read as
// object 15 and the 3 damage went to the wrong player (Life[0] diverged at
// decisions 1202 and 1914). Both replays must play on with no divergence.
func TestFourSeatBecomesTargetHighObjectLockstep(t *testing.T) {
	set, err := cardset.ByName("cmd5")
	if errors.Is(err, cardset.ErrNoCorpus) {
		t.Skipf("no cmd5 table or corpus (make dirac-tables): %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	set = cardset.AllowUnverified(set)
	for _, name := range []string{"cmd5_thunderbreak_high_object_trigsa", "cmd5_witchs_clinic_high_object_trigsa"} {
		t.Run(name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro4", name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			if len(r.MoreDecks) != 2 {
				t.Fatalf("%d further seats, want a four-seat pod", len(r.MoreDecks))
			}
			m, err := r.Matchup(synthReg(t))
			if err != nil {
				t.Fatal(err)
			}
			if m.NSeat() != 4 {
				t.Fatalf("matchup has %d seats, want 4", m.NSeat())
			}
			all := append([][]string{r.Cards[0], r.Cards[1]}, r.MoreCards...)
			lists := make([][]cardset.Entry, len(all))
			for i, names := range all {
				for _, n := range names {
					lists[i] = append(lists[i], cardset.Entry{Name: n, Count: 1})
				}
			}
			tbl, _, err := set.MatchupN(lists)
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
			res := Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Fatalf("the kernel stalled: %q", res.Stall)
			}
			if res.Decisions < 1202 {
				t.Fatalf("the game ended at decision %d", res.Decisions)
			}
		})
	}
}
