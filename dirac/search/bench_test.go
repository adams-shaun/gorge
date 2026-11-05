package search

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// roots collects n positions where seat 0 has a searched decision, from
// random games on the vanilla decks.
func roots(n int) []*dirac.State {
	sr := NewSearcher(DefaultOptions(), Uniform{}, Heuristic{})
	var out []*dirac.State
	for g := uint64(1); len(out) < n; g++ {
		var s dirac.State
		s.Init(v.Table, v.Decks[g%5], v.Decks[(g+2)%5], g, dirac.Config{})
		pol := dirac.NewRandomPolicy(g)
		var d dirac.Decision
		for i := 0; i < 400 && len(out) < n; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			if d.Player == 0 && i%7 == 3 {
				sr.root.CopyFrom(&s)
				sr.root.StepInto(&sr.rootD)
				sr.seat = 0
				for o := 0; o < s.NumObjs(); o++ {
					sr.seen[o] = !s.Hidden(dirac.Obj(o), 0)
				}
				if sr.buildPoint(&sr.root, &sr.rootD, sr.Bot.Pick(&sr.root, &sr.rootD), &sr.rootPt) {
					c := new(dirac.State)
					c.CopyFrom(&s)
					out = append(out, c)
				}
			}
			s.Apply(pol.Pick(&d))
		}
	}
	return out
}

func benchSearch(b *testing.B, opts Options) {
	rs := roots(64)
	sr := NewSearcher(opts, Uniform{}, Heuristic{})
	ProfReset()
	sims, steps, plays, replays := 0, 0, 0, 0
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := sr.Search(rs[i%len(rs)], uint64(i))
		sims += r.Stats.Simulations
		steps += r.Stats.EnvSteps
		plays += r.Stats.Plays
		replays += r.Stats.ReplayPlays
	}
	b.StopTimer()
	ns := float64(b.Elapsed().Nanoseconds())
	b.ReportMetric(ns/float64(sims)/1000, "µs/sim")
	b.ReportMetric(float64(steps+plays)/float64(sims), "submits/sim")
	b.ReportMetric(float64(replays)/float64(plays), "replay-frac")
	if profOn {
		p := ProfNanos()
		names := [...]string{"redeal", "replay", "expand+legal", "backup+select"}
		for k, n := range p {
			b.ReportMetric(float64(n)/float64(sims)/1000, names[k]+"-µs/sim")
		}
	}
}

func BenchmarkSearch(b *testing.B) { benchSearch(b, DefaultOptions()) }

func BenchmarkSearchCachedWorlds(b *testing.B) {
	o := DefaultOptions()
	o.CachedWorlds = 4
	benchSearch(b, o)
}

func BenchmarkSearchCachedWorldsNodeCache(b *testing.B) {
	o := DefaultOptions()
	o.CachedWorlds = 4
	o.NodeCache = 256
	benchSearch(b, o)
}
