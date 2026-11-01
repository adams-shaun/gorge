package oracle

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// testPoolCorpus opens a registered pool's corpus once per pool (the registry
// order indexes onceFor). $DIRAC_POOL_<NAME> overrides the pool directory; a
// box without the pool skips (and says so).
type poolCorpus struct {
	once    sync.Once
	corpus  *Corpus
	pool    []PoolDeck
	missing string
}

var poolCorpora [2]poolCorpus // len(Pools())

func testPoolCorpus(t testing.TB, name string) (*Corpus, []PoolDeck) {
	t.Helper()
	idx := -1
	for i, p := range Pools() {
		if p.Name == name {
			idx = i
		}
	}
	if idx < 0 || idx >= len(poolCorpora) {
		t.Fatalf("pool %q has no corpus slot", name)
	}
	pc := &poolCorpora[idx]
	pc.once.Do(func() {
		p, err := PoolByName(name)
		if err != nil {
			pc.missing = err.Error()
			return
		}
		pool, err := ReadPool(filepath.Join(p.Inputs, "pool.tsv"), "all")
		if err != nil {
			pc.missing = "no " + name + " deck pool (set GORGE_POOLS; dirac/oracle/pools/decks.py builds it): " + err.Error()
			return
		}
		cardsDir, err := cardset.CorpusDir()
		if err != nil {
			pc.missing = err.Error()
			return
		}
		if _, err := os.Stat(filepath.Join(cardsDir, "cardsfolder")); err != nil {
			pc.missing = "no gorge corpus at " + cardsDir
			return
		}
		c, err := OpenCorpus(cardsDir, filepath.Join(p.Inputs, "decks"))
		if err != nil {
			pc.missing = err.Error()
			return
		}
		pc.corpus, pc.pool = c, pool
	})
	if pc.corpus == nil {
		t.Skip(pc.missing)
	}
	return pc.corpus, pc.pool
}

// TestBLBPoolSupported: every BLB pool deck resolves in gorge and is fully
// supported by the generated blb table, so kfuzz -kernel blb skips no pair.
func TestBLBPoolSupported(t *testing.T) {
	t.Parallel()

	c, pool := testPoolCorpus(t, "blb")
	if len(pool) < 2 {
		t.Fatalf("BLB pool has %d decks", len(pool))
	}
	set := rawSet("blb")
	for _, p := range pool {
		d, err := c.Deck(p.Name)
		if err != nil {
			t.Fatal(err)
		}
		if len(d) != 40 {
			t.Errorf("%s: %d cards, want 40", p.Name, len(d))
		}
		if !SetDeckSupported(set, d) {
			t.Errorf("%s: a card the blb table does not support", p.Name)
		}
	}
	t.Logf("%d BLB pool decks, all supported", len(pool))
}

// TestBLBLockstepRuns plays the first BLB pairings (kfuzz's seed-1 census
// configuration) in lockstep.
func TestBLBLockstepRuns(t *testing.T) {
	t.Parallel()

	c, pool := testPoolCorpus(t, "blb")
	cfg := Config{GorgeAutopaySurface: true}
	for _, p := range PairingsFrom(pool, 0, 4, 1) {
		m, tbl, err := SetMatchup(rawSet("blb"), c, p.A, p.B, 1)
		if err != nil {
			t.Fatalf("pair %d: %v", p.Index, err)
		}
		k, err := SetSide(rawSet("blb"), tbl, cfg)(m, p.Seed)
		if err != nil {
			t.Fatalf("pair %d: %v", p.Index, err)
		}
		res := Lockstep(m, p.Seed, k, RandomPolicy(p.Seed), RunConfig{Config: cfg, MaxDecisions: 20000, MaxTurns: 100})
		CloseSide(k)
		if res.Decisions == 0 {
			t.Errorf("pair %d (%s vs %s): no decision played", p.Index, p.A, p.B)
		}
		if d := res.Div; d != nil {
			t.Errorf("pair %d (%s vs %s): diverged at decision %d: %s %s", p.Index, p.A, p.B, d.At, d.What, d.Field)
		}
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Errorf("pair %d (%s vs %s): %v", p.Index, p.A, p.B, err)
		}
		t.Logf("pair %d (%s vs %s): %d decisions, over %v, stall %q", p.Index, p.A, p.B, res.Decisions, res.Over, res.Stall)
	}
}

// blbRegressPairs are tail pairs (kfuzz -kernel blb, seed 1, past the
// gate's slices) that once diverged and now must not.
var blbRegressPairs = []struct {
	Index int
	Why   string
}{
	{3108, "until-leaves return inside a departure batch: its enters check is held to the batch end (CR 603.6a)"},
	{66, "reverting the Coyote last-known power diverges here"},
	{1645, "Driftgloom Coyote reads its exiled target's last known power (CR 608.2h)"},
}

// TestBLBRegressPairs replays each regress pair in kfuzz's BLB
// configuration (0 mulligans, autopay surface, RandomPolicy) and asserts no
// divergence.
func TestBLBRegressPairs(t *testing.T) {
	t.Parallel()

	c, pool := testPoolCorpus(t, "blb")
	cfg := Config{GorgeAutopaySurface: true}
	for _, rp := range blbRegressPairs {
		ps := PairingsFrom(pool, rp.Index, 1, 1)
		if len(ps) != 1 {
			t.Fatalf("pair %d: not in the pool", rp.Index)
		}
		p := ps[0]
		m, tbl, err := SetMatchup(rawSet("blb"), c, p.A, p.B, 0)
		if err != nil {
			t.Fatalf("pair %d: %v", p.Index, err)
		}
		k, err := SetSide(rawSet("blb"), tbl, cfg)(m, p.Seed)
		if err != nil {
			t.Fatalf("pair %d: %v", p.Index, err)
		}
		res := Lockstep(m, p.Seed, k, RandomPolicy(p.Seed), RunConfig{Config: cfg, MaxDecisions: 20000, MaxTurns: 100})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Errorf("pair %d (%s): diverged at decision %d: %s %s %v %s", p.Index, rp.Why, d.At, d.What, d.Field, d.Diff, d.Detail)
		}
	}
}
