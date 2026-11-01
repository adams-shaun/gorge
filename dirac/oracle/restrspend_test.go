package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestRestrictedManaCrafted plays crafted Giada / Bloom Tender / Angel
// pairs (testdata/giada: two GW decks with eight Tenders, four Giadas and
// six Youthful Valkyries) in lockstep without the autopay surface, the
// oracle lowering potential plays (Config.GorgeLowering), where an Angel
// spell is reached through a script that taps Giada: gorge prices the
// script with her restricted {W} (the restriction is not read there), an
// Angel spell's payment sees it (pay.AvailableFor) and spends it first
// (emitRestrictedManaSpend), and while it floats gorge's potential plays
// are all "unsupported: pool" (no scripts). With Giada as the only gap
// the census is relaxable: a play the relaxed proof cannot pay takes no
// search. Pairs 5, 19 and 38 spend Giada's {W} on Angels 3, 4 and 3 times;
// all three diverged before (craft pool 138 of 200 clean -> 200). Since gorge
// (85fddcb68) an untapped Giada is no gap but an ordinary planner
// source for an Angel spell (CR 106.6, 601.2g: restrsrc.go), so the same
// three pairs diverge without that model (TestRestrictedManaCraftedMutation).
func restrCraftedRun(t *testing.T, pairs []int, report bool) (diverged int) {
	t.Helper()
	c0, _ := testCorpus(t)
	c := &Corpus{Reg: c0.Reg, DecksDir: "testdata/giada/decks", decks: map[string][]*cards.Card{}}
	pool, err := ReadPool("testdata/giada/pool.tsv", "all")
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{}
	for _, pr := range pairs {
		p := PairingsFrom(pool, pr, 1, 1)[0]
		m, tbl, err := SetMatchup(rawSet("fdn"), c, p.A, p.B, 0)
		if err != nil {
			t.Fatalf("pair %d: %v", pr, err)
		}
		k, err := SetSide(rawSet("fdn"), tbl, cfg)(m, p.Seed)
		if err != nil {
			t.Fatalf("pair %d: %v", pr, err)
		}
		res := Lockstep(m, p.Seed, k, RandomPolicy(p.Seed), RunConfig{Config: cfg, MaxDecisions: 20000, MaxTurns: 100})
		CloseSide(k)
		if d := res.Div; d != nil {
			diverged++
			if report {
				t.Errorf("pair %d: diverged at decision %d: %s %s %v", pr, d.At, d.What, d.Field, d.Diff)
			}
		}
	}
	return diverged
}

func TestRestrictedManaCrafted(t *testing.T) {
	t.Parallel()
	restrCraftedRun(t, []int{5, 19, 38}, true)
}

// TestRestrictedManaCraftedPool plays the first 100 pairs of the crafted
// pool (pairs 5, 19 and 38 among them) and the next 100 in lockstep.
func TestRestrictedManaCraftedPool(t *testing.T) {
	t.Parallel()
	pairs := make([]int, 100)
	for i := range pairs {
		pairs[i] = i
	}
	restrCraftedRun(t, pairs, true)
}

func TestRestrictedManaCraftedPool2(t *testing.T) {
	t.Parallel()
	pairs := make([]int, 100)
	for i := range pairs {
		pairs[i] = 100 + i
	}
	restrCraftedRun(t, pairs, true)
}

// TestRestrictedManaCraftedMutation: with the restricted sources ignored
// (dirac.DebugRestrictedSource = 2: the offer gate and the planner leave Giada out),
// the three pairs diverge.
func TestRestrictedManaCraftedMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugRestrictedSource = 2
	defer func() { dirac.DebugRestrictedSource = 0 }()
	n := restrCraftedRun(t, []int{5, 19, 38}, false)
	if n == 0 {
		t.Fatal("no pair diverged with the restricted sources ignored")
	}
}
