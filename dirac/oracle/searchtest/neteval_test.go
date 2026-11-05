package searchtest

import (
	"fmt"
	"math"
	"os"
	"testing"

	"github.com/adams-shaun/gorge/bots/gonet"
	"github.com/adams-shaun/gorge/bots/gonet/gonettest"
	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/feat"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/neteval"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/dirac/vanilla"
	"github.com/adams-shaun/gorge/engine/decision"
)

// netPath is the network the evaluator parity tests load: KORACLE_NET
// when set (any .dzgw export, to run the same comparisons on a trained
// network), else gonettest's deterministic synthetic network of the production widths (gonettest.Wide),
// written under t's temp directory. Parity is arithmetic, not playing
// strength, so a synthetic network checks it.
func netPath(t testing.TB) string {
	t.Helper()
	if p := os.Getenv("KORACLE_NET"); p != "" {
		return p
	}
	p, err := gonettest.WriteShape(t.TempDir(), "net.dzgw", gonettest.Wide, policynet.EntityRawWidth, nil)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// netStats are the numeric comparison of dirac/neteval against gorge's
// gonet on the same positions.
type netStats struct {
	decisions, options int
	maxV, maxL         float64
	over               int // |logit diff| > 1e-3 or |value diff| > 1e-4
	first              string
	featGaps           int // decisions whose encoded inputs already differ (dirac/feat's, not the evaluator's)
}

// netCheck compares, at every decision before the game's first divergence
// (first pick of a multi-pick), neteval's value and option logits over the
// dirac position with gonet's over gorge's own encoding of the same
// position.
func netCheck(pool *feat.Pool, w *neteval.Weights, ref *gonet.Net, st *netStats) func(*oracle.GorgeSide, oracle.Side) *oracle.Divergence {
	n := neteval.New(w, pool)
	enc := feat.NewEncoder(pool)
	var kopts []feat.Option
	var gopts, list []decision.Option
	var gencBuf []policynet.Option
	var gpos []int
	var logits, gl [dirac.MaxOptions]float32
	return func(g *oracle.GorgeSide, k oracle.Side) *oracle.Divergence {
		ks, kd, kcanon := oracle.KernelStateOf(k)
		if ks == nil || g.Pending() == nil || g.MidPick() || kd.N == 0 {
			return nil
		}
		// gorge's net scores its own list (GorgeNetList), at each entry's
		// index over that list's length: the inputs both sides score.
		list = g.GorgeNetList(0, list)
		var gst policynet.State
		var ok bool
		var genc []policynet.Option
		gst, genc, gopts, gpos, ok = g.GorgeNetFeatures(kcanon, list, gencBuf, gopts, gpos)
		gencBuf = genc
		if !ok {
			return nil
		}
		kst := enc.EncodeState(ks, kd)
		kopts = enc.EncodeOptions(kd, kopts)
		m := oracle.CompareState(&gst, kst)
		for i := 0; m == nil && i < len(genc); i++ {
			m = oracle.CompareOption(i, &genc[i], &kopts[i])
		}
		if m != nil {
			st.featGaps++
			return nil
		}
		gv := gonet.Sigmoid32(ref.Eval(&gst, genc, gl[:len(genc)]))
		n.NewWorld()
		n.Priors(ks, kd, nil, logits[:kd.N])
		kv := n.Value(ks, kd, kd.Player)
		st.decisions++
		dv := math.Abs(float64(kv - gv))
		if dv > st.maxV {
			st.maxV = dv
		}
		bad := dv > 1e-4
		for i := range genc {
			st.options++
			dl := math.Abs(float64(logits[i] - gl[i]))
			if dl > st.maxL {
				st.maxL = dl
			}
			if dl > 1e-3 {
				bad = true
			}
		}
		if bad {
			st.over++
			if st.first == "" {
				st.first = fmt.Sprintf("%s decision p%d turn %d: value %v vs %v", g.Pending().Kind, g.Pending().Player, g.E.G.Turn, kv, gv)
			}
		}
		return nil
	}
}

func loadNets(t *testing.T) (*neteval.Weights, *gonet.Net) {
	net := netPath(t)
	w, err := neteval.LoadWeights(net)
	if err != nil {
		t.Fatal(err)
	}
	cs := gonet.CacheSize
	gonet.CacheSize = 0
	t.Cleanup(func() { gonet.CacheSize = cs })
	ref, err := gonet.Load(net)
	if err != nil {
		t.Fatal(err)
	}
	ref.SetCardCaches(false)
	return w, ref
}

// TestNetevalMatchesGonetVanilla: neteval over dirac positions equals gonet
// over gorge's encoding of the same positions (vanilla decks, lockstep).
func TestNetevalMatchesGonetVanilla(t *testing.T) {
	c, _ := testCorpus(t)
	w, ref := loadNets(t)
	pool, err := oracle.FeatPool(c.Reg, vanilla.Table, nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg := oracle.RunConfig{}
	mk := oracle.RealKernel(vanilla.Table, nil, cfg.Config)
	var st netStats
	for g := 0; g < 10; g++ {
		a := g % len(vanilla.Decks)
		b := (a + 1 + g/len(vanilla.Decks)%(len(vanilla.Decks)-1)) % len(vanilla.Decks)
		seed := uint64(9100 + g)
		m, err := oracle.KernelMatchup(c.Reg, vanilla.Table, [2]string{fmt.Sprint("vanilla", a), fmt.Sprint("vanilla", b)},
			[2][]kop.CardID{vanilla.Decks[a], vanilla.Decks[b]}, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := mk(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		rc := cfg
		rc.Check = netCheck(pool, w, ref, &st)
		oracle.Lockstep(m, seed, k, oracle.RandomPolicy(seed), rc)
	}
	t.Logf("compared %d decisions, %d option logits: max |Δvalue| %.3g, max |Δlogit| %.3g, %d over tolerance; first: %s",
		st.decisions, st.options, st.maxV, st.maxL, st.over, st.first)
	if st.decisions == 0 || st.over > 0 {
		t.Fatalf("neteval differs from gonet")
	}
}

// TestNetevalMatchesGonetFDN is the FDN version (generated table).
func TestNetevalMatchesGonetFDN(t *testing.T) {
	c, pool := testCorpus(t)
	w, ref := loadNets(t)
	cfg := oracle.RunConfig{}
	var st netStats
	games := 0
	for _, p := range oracle.Pairings(pool, 20, 13) {
		m, tbl, err := oracle.SetMatchup(rawSet("fdn"), c, p.A, p.B, 0)
		if err != nil {
			continue
		}
		fp, err := oracle.FeatPool(c.Reg, tbl, nil)
		if err != nil {
			t.Fatal(err)
		}
		k, err := oracle.RealKernel(tbl, nil, cfg.Config)(m, p.Seed)
		if err != nil {
			continue
		}
		rc := cfg
		rc.Check = netCheck(fp, w, ref, &st)
		oracle.Lockstep(m, p.Seed, k, oracle.RandomPolicy(p.Seed), rc)
		games++
	}
	t.Logf("%d games: compared %d decisions, %d option logits (%d decisions skipped: encoded inputs differ): max |Δvalue| %.3g, max |Δlogit| %.3g, %d over tolerance; first: %s",
		games, st.decisions, st.options, st.featGaps, st.maxV, st.maxL, st.over, st.first)
	if st.over > 0 {
		t.Errorf("neteval differs from gonet on %d decisions", st.over)
	}
}
