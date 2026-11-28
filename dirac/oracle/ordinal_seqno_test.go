package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// ordinalAgreedFloor is the fewest decisions every game must play with the
// counters equal before the first drift: the mulligans and the opening turns,
// which precede the first multi-pick declaration and the first internal answer.
const ordinalAgreedFloor = 20

// TestLockstepOrdinalMatchesSeqNo runs OrdinalCheck over two-seat FDN pairs
// and a four-seat cmd5 pod. The kernel counts each pick of a multi-pick
// declaration (attackers, blockers, multi-target, multi-choose, trigger
// order) where gorge counts one answer, and gorge answers internal decisions
// (payment windows, forced choices) the kernel never poses, so the counters
// still drift at the first such decision (filed:
// dirac-decisions-count-gorge-answers). Until that lands this pins what
// agrees: every decision before the first drift, at least the opening, and the
// check must fire as an "ordinal" divergence, not some other one.
func TestLockstepOrdinalMatchesSeqNo(t *testing.T) {
	type game struct {
		name string
		res  Result
		n    int
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
		var n int
		res := Lockstep(m, p.Seed, k, RandomPolicy(p.Seed), RunConfig{Config: cfg, MaxTurns: 40, Check: OrdinalCheck(&n)})
		CloseSide(k)
		games = append(games, game{"fdn " + p.A + " vs " + p.B, res, n})
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
		var n int
		res := Lockstep(m, pod.Seed, k, AffordableTargetPolicy(RandomPolicy(pod.Seed)), RunConfig{MaxTurns: 40, Check: OrdinalCheck(&n)})
		CloseSide(k)
		games = append(games, game{"cmd5 pod", res, n})
	}
	if two == 0 || len(games) == two {
		t.Fatalf("precondition: %d two-seat games, %d four-seat", two, len(games)-two)
	}
	for _, g := range games {
		if g.n == 0 {
			t.Fatalf("%s: the check compared no decision", g.name)
		}
		d := g.res.Div
		if d == nil {
			t.Logf("%s: counters agree at all %d decisions", g.name, g.n)
			continue
		}
		if d.What != "ordinal" {
			t.Fatalf("%s: divergence %s/%s at %d before any ordinal drift: %s", g.name, d.What, d.Field, d.At, d.Detail)
		}
		if g.n-1 < ordinalAgreedFloor {
			t.Errorf("%s: counters drift after %d agreed decisions (floor %d): %s", g.name, g.n-1, ordinalAgreedFloor, d.Detail)
		}
		t.Logf("%s: %d decisions agree, then %s", g.name, g.n-1, d.Detail)
	}
}
