package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestLockstepOrdinalMatchesSeqNo runs OrdinalCheck over two-seat FDN pairs
// and two four-seat cmd5 pods: at every decision of every game the kernel's
// SeqNo is gorge's Ordinal net of the answers gorge gives beneath the
// canonical surface. The games reach multi-pick declarations (attackers,
// blockers, trigger orders, cost picks), which gorge answers once and the
// kernel pick by pick; the kernel counts the closing pick only.
func TestLockstepOrdinalMatchesSeqNo(t *testing.T) {
	type game struct {
		name  string
		res   Result
		n     int
		multi int
	}
	var games []game
	c, pool := testCorpus(t)
	for _, p := range Pairings(pool, 4, 20261128) {
		m, tbl, err := SetMatchup(rawSet("fdn"), c, p.A, p.B, 0)
		if err != nil {
			continue
		}
		cfg := Config{GorgeAutopaySurface: true}
		k, err := SetSide(rawSet("fdn"), tbl, cfg)(m, p.Seed)
		if err != nil {
			continue
		}
		var n, multi int
		res := Lockstep(m, p.Seed, k, RandomPolicy(p.Seed), RunConfig{Config: cfg, MaxTurns: 40, Check: multiPickCounted(&multi, OrdinalCheck(&n))})
		CloseSide(k)
		games = append(games, game{"fdn " + p.A + " vs " + p.B, res, n, multi})
	}
	two := len(games)
	pc, names := cmd5MiniCorpus(t, 40)
	set, err := cardset.ByName("cmd5")
	if err != nil {
		t.Skipf("no cmd5 table (make dirac-tables): %v", err)
	}
	set = cardset.AllowUnverified(set)
	pp := make([]PoolDeck, len(names))
	for i, nm := range names {
		pp[i] = PoolDeck{Name: nm}
	}
	for _, i := range []int{0, 1} {
		pod := PodAt(pp, i, 1, 4)
		m, tbl, err := SetMatchupN(set, pc, pod.Decks(), 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{})(m, pod.Seed)
		if err != nil {
			t.Fatal(err)
		}
		var n, multi int
		res := Lockstep(m, pod.Seed, k, AffordableTargetPolicy(RandomPolicy(pod.Seed)), RunConfig{MaxTurns: 40, Check: multiPickCounted(&multi, OrdinalCheck(&n))})
		CloseSide(k)
		games = append(games, game{"cmd5 pod", res, n, multi})
	}
	if two == 0 || len(games) == two {
		t.Fatalf("precondition: %d two-seat games, %d four-seat", two, len(games)-two)
	}
	for _, g := range games {
		if g.n == 0 {
			t.Fatalf("%s: the check compared no decision", g.name)
		}
		if d := g.res.Div; d != nil {
			t.Errorf("%s: divergence %s/%s at %d after %d decisions compared: %s", g.name, d.What, d.Field, d.At, g.n, d.Detail)
			continue
		}
		if g.multi == 0 {
			t.Errorf("%s: precondition: no multi-pick declaration in %d decisions", g.name, g.n)
		}
		t.Logf("%s: counters agree at all %d decisions (%d multi-pick declarations)", g.name, g.n, g.multi)
	}
}

// multiPickCounted wraps check, counting in *multi the gorge decisions the
// kernel posed as two or more picks (the same gorge decision pending at
// consecutive checks).
func multiPickCounted(multi *int, check func(*GorgeSide, Side) *Divergence) func(*GorgeSide, Side) *Divergence {
	last, counted := ^uint64(0), false
	return func(ref *GorgeSide, k Side) *Divergence {
		if gd := ref.Pending(); gd != nil {
			if gd.Seq != last {
				last, counted = gd.Seq, false
			} else if !counted {
				*multi++
				counted = true
			}
		}
		return check(ref, k)
	}
}
