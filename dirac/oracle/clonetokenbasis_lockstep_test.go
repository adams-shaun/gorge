package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
)

// A token copy keeps its own copiable values (CR 707.2, 707.9b). Replicator's
// {2}, {T} copies a Neuroflux Heirloom token that Gwyndala, the Progeny's {2}
// has made a copy of another Equipment: the new token is that Equipment,
// minted on Neuroflux Heirloom's card. When Gwyndala then makes it a copy of
// something else until end of turn, the cleanup step puts it back on the
// copy it was created as, not on its printed card. cloneTokenBasisGames plays
// the pool; every game must agree and none may stop on something the kernel
// does not model.
func cloneTokenBasisGames(t *testing.T, seeds uint64) (st dirac.CloneCopyStats, div int, capStalls map[string]int) {
	t.Helper()
	decks := [2][]cardset.Entry{
		{
			{Name: "Gwyndala, the Progeny", Count: 12}, {Name: "Replicator", Count: 8},
			{Name: "Type-2 Phaser", Count: 4}, {Name: "Tricorder", Count: 4},
			{Name: "Island", Count: 20},
		},
		{
			{Name: "Gwyndala, the Progeny", Count: 12}, {Name: "Replicator", Count: 8},
			{Name: "Bat'leth", Count: 4},
			{Name: "Island", Count: 14}, {Name: "Swamp", Count: 7},
		},
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-trk")
	var cs [2][]*cards.Card
	for s, d := range decks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				cs[s] = append(cs[s], c)
			}
		}
	}
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	dirac.DebugCloneCopyStats = &st
	defer func() { dirac.DebugCloneCopyStats = nil }()
	capStalls = map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"tokbasis-a", "tokbasis-b"}, cs, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if res.Div != nil {
			div++
			if !dirac.DebugCloneCopyNoHold {
				d := res.Div
				t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s", seed, d.At, d.What, d.Field, d.Diff, d.Detail)
			}
			continue
		}
		if res.StallUnmodelled {
			t.Fatalf("seed %d: kernel stopped on something it does not model: %s", seed, res.Stall)
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			capStalls[res.Stall]++
		}
	}
	return st, div, capStalls
}

func TestCloneTokenBasisLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	// not parallel: it sets dirac.DebugCloneCopyStats
	st, _, capStalls := cloneTokenBasisGames(t, 120)
	t.Logf("clone copies: %+v; capacity stalls: %v", st, capStalls)
	if st.Minted == 0 || st.Held == 0 || st.Restored == 0 {
		t.Fatalf("no token copy of a copy was copied over and settled back to another card: %+v", st)
	}
}

// TestCloneTokenBasisMutation: with the held copy dropped the token settles
// on its printed card, and some game must diverge.
func TestCloneTokenBasisMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	// not parallel: it flips dirac.DebugCloneCopyNoHold
	dirac.DebugCloneCopyNoHold = true
	defer func() { dirac.DebugCloneCopyNoHold = false }()
	if _, div, _ := cloneTokenBasisGames(t, 120); div == 0 {
		t.Fatal("no game diverged with a token copy's own copy dropped at cleanup")
	}
}
