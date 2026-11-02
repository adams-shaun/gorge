package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestSBTParity plays FDN pairings in lockstep with gorge's sb-tactical
// choosing and asks the kernel's sbt agent every decision in shadow
// (SBTParityGame): the agent must give sb-tactical's answer at >= 99% of
// priority decisions and >= 98% of every other kind it is asked (ksbt
// measures 300 pairs: 99.99% / >= 99.3%).
func TestSBTParity(t *testing.T) {
	t.Parallel()

	c, pool := testCorpus(t)
	type tally struct{ n, agree int }
	kinds := map[string]*tally{}
	ps := PairingsFrom(pool, 0, 12, 78)
	for _, p := range ps {
		res, err := SBTParityGame(c, p.A, p.B, p.Seed, Config{GorgeAutopaySurface: true, PassElision: true}, 20000)
		if err != nil {
			t.Fatalf("pair %d: %v", p.Index, err)
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
				t.Logf("pair %d: %s", p.Index, r.Where)
			}
		}
	}
	if kinds["priority"] == nil || kinds["priority"].n < 500 {
		t.Fatalf("too few priority decisions compared: %+v", kinds["priority"])
	}
	for k, x := range kinds {
		rate := float64(x.agree) / float64(x.n)
		t.Logf("%-16s %5d %.4f", k, x.n, rate)
		want := 0.98
		if k == "priority" {
			want = 0.99
		}
		if x.n >= 20 && rate < want {
			t.Errorf("%s: agreement %.4f < %.2f over %d decisions", k, rate, want, x.n)
		}
	}
}

// TestSBTPlaysKernelGames plays the agent against itself on the pool table
// with the default kernel configuration (every castable spell offered):
// every game ends, no answer falls back.
func TestSBTPlaysKernelGames(t *testing.T) {
	t.Parallel()

	c, pool := testCorpus(t)
	set := cardsettest.Set(t, "fdn")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	dirac.Register(tbl)
	cs := NewSBTCards(c.Reg, tbl, nil)
	ag := NewSBT(cs)
	ps := PairingsFrom(pool, 0, 8, 78)
	for _, p := range ps {
		m, _, err := SetMatchup(rawSet("fdn"), c, p.A, p.B, 0)
		if err != nil {
			t.Fatal(err)
		}
		var decks [2][]dirac.CardID
		for s := 0; s < 2; s++ {
			for _, gc := range m.Decks[s] {
				i, ok := set.Lookup(cardName(gc))
				if !ok {
					t.Fatalf("%s not in the pool", cardName(gc))
				}
				decks[s] = append(decks[s], set.PoolID(i))
			}
		}
		var s dirac.State
		s.Init(tbl, decks[0], decks[1], p.Seed, dirac.Config{})
		ag.NewGame([2]uint64{SeatSeed(p.Seed, 0), SeatSeed(p.Seed, 1)})
		var d dirac.Decision
		n := 0
		for ; n < 20000; n++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			s.Apply(d.Options[ag.Pick(&s, &d)])
		}
		if over, _ := s.Terminal(); !over {
			t.Errorf("pair %d: not over after %d decisions (turn %d)", p.Index, n, s.Turn())
		}
	}
	if ag.Stats.Fallbacks > 0 {
		t.Errorf("%d of %d answers fell back", ag.Stats.Fallbacks, ag.Stats.Decisions)
	}
	t.Logf("%d decisions, %d served from a queued declaration", ag.Stats.Decisions, ag.Stats.Queued)
}
