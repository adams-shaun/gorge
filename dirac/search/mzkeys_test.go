package search

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// mzxWalk plays random games of tbl and calls visit at every seat-0
// priority decision (a copy of the state) until visit has returned true n
// times or the seeds run out.
func mzxWalk(t *testing.T, tbl *kop.Table, da, db []dirac.CardID, n int, visit func(s *dirac.State, d *dirac.Decision) bool) int {
	t.Helper()
	hits := 0
	for seed := uint64(1); seed <= 60 && hits < n; seed++ {
		s := dirac.NewConfig(tbl, da, db, seed, dirac.Config{})
		pol := dirac.NewRandomPolicy(seed ^ 0x6d7a78)
		var d dirac.Decision
		for i := 0; i < 3000 && hits < n; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			if d.Player == 0 && d.Kind == dirac.DkPriority {
				var c dirac.State
				c.CopyFrom(s)
				if visit(&c, &d) {
					hits++
				}
			}
			s.Apply(pol.Pick(&d))
		}
	}
	return hits
}

func offersKind(d *dirac.Decision, k dirac.ActionKind) bool {
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == k {
			return true
		}
	}
	return false
}

func rootHasKind(sr *Searcher, n int, k dirac.ActionKind) bool {
	for i := 0; i < n; i++ {
		if dirac.Action(sr.RootCand(i)[0]).Kind() == k {
			return true
		}
	}
	return false
}

// TestAltCandsSearchesBestowAndPlot: off, a priority decision offering a
// bestow cast (Nyxborn Hydra) or the plot action never has one as a
// candidate unless it is the bot's answer; on, it is a candidate.
func TestAltCandsSearchesBestowAndPlot(t *testing.T) {
	kbestowSet := cardsettest.Set(t, "bestow")
	bt, ba, bb, err := kbestowSet.Matchup(
		[]cardset.Entry{{Name: "Nyxborn Hydra", Count: 24}, {Name: "Forest", Count: 24}, {Name: "Grizzly Bears", Count: 12}},
		[]cardset.Entry{{Name: "Forest", Count: 24}, {Name: "Grizzly Bears", Count: 36}})
	if err != nil {
		t.Fatal(err)
	}
	plotSet := cardsettest.Set(t, "plot")
	pt, pa, pb, err := plotSet.Matchup(
		[]cardset.Entry{{Name: "Loan Shark", Count: 24}, {Name: "Island", Count: 24}, {Name: "Plan the Heist", Count: 12}},
		[]cardset.Entry{{Name: "Island", Count: 24}, {Name: "Loan Shark", Count: 36}})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name   string
		tbl    *kop.Table
		da, db []dirac.CardID
		k      dirac.ActionKind
	}{{"bestow", bt, ba, bb, dirac.AkBestow}, {"plot", pt, pa, pb, dirac.AkPlot}} {
		off := landSearcher(c.tbl, func(o *Options) {})
		on := landSearcher(c.tbl, func(o *Options) { o.AltCands = true })
		hits := mzxWalk(t, c.tbl, c.da, c.db, 8, func(s *dirac.State, d *dirac.Decision) bool {
			if !offersKind(d, c.k) {
				return false
			}
			bot := d.Options[off.Bot.Pick(s, d)].Kind()
			if bot == dirac.AkPlayLand {
				return false // a land-play answer is unsearched without LandSearch
			}
			r0 := *off.Search(s, 3)
			if r0.Stats.Searched == 1 && bot != c.k && rootHasKind(off, r0.N, c.k) {
				t.Fatalf("%s off: a candidate of kind %d", c.name, c.k)
			}
			r1 := *on.Search(s, 3)
			if r1.Stats.Searched != 1 || !rootHasKind(on, r1.N, c.k) {
				t.Fatalf("%s on: searched %d, %d candidates, none of kind %d", c.name, r1.Stats.Searched, r1.N, c.k)
			}
			return true
		})
		if hits == 0 {
			t.Fatalf("%s: the walk offered no option of kind %d", c.name, c.k)
		}
	}
}

// TestCastDedupMergesCopies: on, no two cast candidates share card, mode
// and target, and a decision with two copies of a castable card in hand
// has fewer candidates than off.
func TestCastDedupMergesCopies(t *testing.T) {
	tbl, _, _ := landMatchup(t)
	_, da, db := landMatchup(t)
	off := landSearcher(tbl, func(o *Options) {})
	on := landSearcher(tbl, func(o *Options) { o.CastDedup = true })
	hits := mzxWalk(t, tbl, da, db, 20, func(s *dirac.State, d *dirac.Decision) bool {
		var tmp point
		dup := false
		for i := 0; i < int(d.N); i++ {
			a := d.Options[i]
			if castDup(s, &tmp, a) {
				dup = true
			}
			tmp.addSeq([]uint32{uint32(a)})
		}
		if !dup {
			return false
		}
		r0 := *off.Search(s, 4)
		r1 := *on.Search(s, 4)
		if r0.Stats.Searched != 1 {
			return false
		}
		for i := 0; i < r1.N; i++ {
			var seen point
			for j := 0; j < i; j++ {
				seen.addSeq(on.RootCand(j))
			}
			if castDup(s, &seen, dirac.Action(on.RootCand(i)[0])) {
				t.Fatalf("on: candidate %d repeats an earlier cast", i)
			}
		}
		if r1.N >= r0.N {
			t.Fatalf("on: %d candidates, off %d, with a duplicate cast offered", r1.N, r0.N)
		}
		return true
	})
	if hits == 0 {
		t.Fatal("the walk offered no duplicate cast")
	}
}
