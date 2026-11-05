package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
)

// A counter doubler beside a counter plus-N on one placement is a CR 616.1
// order competition (doubling then adding one is not adding one then
// doubling): gorge asks the recipient's controller which applies first. Dr.
// Joseph M'Benga doubles what its controller puts on a creature; Brad
// Boimler, Eager Ensign's tap trigger adds one until end of turn. The kernel
// poses the same ask before a resolving program's OpPutCounter and applies
// the answered order; the expected answer at every decision is gorge's.
var counterOrderDecks = [2][]cardset.Entry{
	{
		{Name: "Dr. Joseph M'Benga", Count: 4}, {Name: "Lt. Cmdr. Geordi La Forge", Count: 2}, {Name: "Brad Boimler, Eager Ensign", Count: 4},
		{Name: "Dr. Beverly Crusher", Count: 3}, {Name: "Federation Field Medic", Count: 3}, {Name: "Shields Up!", Count: 3},
		{Name: "Support Mission", Count: 3}, {Name: "Tactical Officer", Count: 3}, {Name: "Plains", Count: 8}, {Name: "Forest", Count: 7},
	},
	{{Name: "Plains", Count: 40}},
}

// counterOrderGames plays seeds games of counterOrderDecks and returns the
// stalls and divergences (the first divergence's text).
func counterOrderGames(t *testing.T, seeds uint64) (stalls map[string]int, div string) {
	t.Helper()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-trk")
	var cs [2][]*cards.Card
	for s, d := range counterOrderDecks {
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
	tbl, _, _, err := set.Matchup(counterOrderDecks[0], counterOrderDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	stalls = map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"fixture-a", "fixture-b"}, cs, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if res.Stall != "" {
			stalls[res.Stall]++
		}
		if d := res.Div; d != nil && div == "" {
			div = d.What + " " + d.Field + " " + d.Detail
		}
	}
	return stalls, div
}

func TestCounterReplacementOrderLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	asked := dirac.CtrOrderStats.Asked.Load()
	stalls, div := counterOrderGames(t, 150)
	if div != "" {
		t.Fatalf("diverged: %s", div)
	}
	if stalls["kernel replctr"] != 0 {
		t.Fatalf("stalls %v: the order ask is not posed", stalls)
	}
	if got := dirac.CtrOrderStats.Asked.Load() - asked; got < 10 {
		t.Fatalf("%d order asks posed: the pool does not exercise the competition", got)
	}
	t.Logf("asks %d, stalls %v", dirac.CtrOrderStats.Asked.Load()-asked, stalls)
}

// TestCounterReplacementOrderMutation proves the answers matter: with the
// kernel ignoring them a game whose controller picks the other order diverges.
func TestCounterReplacementOrderMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	old := dirac.DebugCtrOrderIgnore
	dirac.DebugCtrOrderIgnore = true
	defer func() { dirac.DebugCtrOrderIgnore = old }()
	if _, div := counterOrderGames(t, 150); div == "" {
		t.Fatal("ignoring the order answers diverged nowhere")
	}
}
