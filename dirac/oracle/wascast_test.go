package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The enter-time cast provenance lockstep (gorge WasCast on
// the object a resolution puts onto the battlefield): W/B decks over the
// generated wascast pool. Patched Plaything's enters replacement
// (CheckSVar$ Count$wasCastFromYourHandByYou.1.0) puts two -1/-1 counters on it
// only when its controller cast it from their hand. Cloudshift flickers one
// and Dread Return reanimates one, both entering uncast; Murder, Journey to
// Nowhere and Stitcher's Supplier move Plaything to the graveyard and exile.
//
// Myojin of Night's Reach is the second enters-replacement carrier: its
// K:etbCounter:DIVINITY:1 replacement puts a divinity counter only when it
// was cast from its controller's hand (the same Count$wasCastFromYourHandByYou
// read), and its own static ability reads that counter. The other Standard
// trigger carriers (Doomsday Excruciator, Satoru, Twilight Diviner and the
// ThisTurnEnteredFrom_Graveyard batch) still refuse on a second blocker at
// this seat (by reading).

var kwascastDecks = [2][]cardset.Entry{
	{ // A
		{Name: "Myojin of Night's Reach", Count: 3}, {Name: "Patched Plaything", Count: 4},
		{Name: "Cloudshift", Count: 2}, {Name: "Dread Return", Count: 2}, {Name: "Murder", Count: 3},
		{Name: "Stitcher's Supplier", Count: 2}, {Name: "Journey to Nowhere", Count: 1},
		{Name: "Savannah Lions", Count: 2}, {Name: "Barony Vampire", Count: 1}, {Name: "Child of Night", Count: 2},
		{Name: "Suntail Hawk", Count: 1}, {Name: "Plains", Count: 6}, {Name: "Swamp", Count: 11},
	},
	{ // B
		{Name: "Patched Plaything", Count: 5}, {Name: "Cloudshift", Count: 4}, {Name: "Dread Return", Count: 1},
		{Name: "Murder", Count: 2}, {Name: "Stitcher's Supplier", Count: 3}, {Name: "Journey to Nowhere", Count: 2},
		{Name: "Savannah Lions", Count: 3}, {Name: "Serra Angel", Count: 1}, {Name: "Suntail Hawk", Count: 2},
		{Name: "Child of Night", Count: 2}, {Name: "Barony Vampire", Count: 1}, {Name: "Plains", Count: 8}, {Name: "Swamp", Count: 6},
	},
}

// kwascastRun plays seeds 1..seeds and returns whether a game diverged, the
// kernel capacity stall count, the predicate's coverage counters and the
// number of games in which a battlefield permanent held a DIVINITY counter
// (Myojin's enters replacement).
func kwascastRun(t *testing.T, seeds uint64, report bool) (diverged bool, stalls int, st dirac.WasCastStats, divinity int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("wascast")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kwascastDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kwascastDecks[0], kwascastDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	dirac.DebugWasCastStats = &st
	defer func() { dirac.DebugWasCastStats = nil }()
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kwascast-a", "kwascast-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		// sawDivinity is this game's DIVINITY observation: the hook projects
		// the kernel state every decision the lockstep poses, so a counter
		// that is later removed (Myojin's discard activation) still counts.
		sawDivinity := false
		prevHook := OccupancyHook
		OccupancyHook = func(s *dirac.State) {
			var p dirac.Projection
			s.Project(&p)
			for i := 0; i < int(p.NObj); i++ {
				o := &p.Objs[i]
				if o.Zone != kop.ZoneBattlefield {
					continue
				}
				if o.CountersX[int(kop.CtrDivinity-kop.CtrCharge)] > 0 {
					sawDivinity = true
				}
			}
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		OccupancyHook = prevHook
		CloseSide(k)
		if sawDivinity {
			divinity++
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
		}
		if d := res.Div; d != nil {
			diverged = true
			if report {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			break
		}
	}
	return
}

func TestWascastLockstep(t *testing.T) {
	if isolate(t) {
		return
	}

	div, stalls, st, divinity := kwascastRun(t, 256, true)
	t.Logf("256 games, wascast %+v, %d divergences, %d stalls, %d games with a DIVINITY counter", st, b2i(div), stalls, divinity)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	if divinity < 1 {
		t.Errorf("DIVINITY counter observed in %d games, want at least 1: Myojin's cast-from-hand replacement was not exercised", divinity)
	}
	for _, c := range []struct {
		n, min int
		s      string
	}{
		{st.Reads, 20, "enters replacement reads"},
		{st.Cast, 20, "reads of a resolving spell"},
		{st.FromHand, 20, "reads of a spell cast from its owner's hand by its controller"},
		{st.NotCast, 10, "reads of an object that entered uncast"},
		{st.Matched, 20, "reads that held"},
	} {
		if c.n < c.min {
			t.Errorf("%s: %d, want at least %d", c.s, c.n, c.min)
		}
	}
}

// TestWascastMutation: an enterCast that reads every entering object as cast
// makes a flickered or reanimated Plaything enter with its counters, which
// gorge does not do.
func TestWascastMutation(t *testing.T) {
	if isolate(t) {
		return
	}

	dirac.DebugWasCastAlways = true
	defer func() { dirac.DebugWasCastAlways = false }()
	if diverged, _, _, _ := kwascastRun(t, 64, false); !diverged {
		t.Fatal("DebugWasCastAlways: no divergence in 64 seeds")
	}
}
