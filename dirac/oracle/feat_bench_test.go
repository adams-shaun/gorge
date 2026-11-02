package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/feat"
)

type featPos struct {
	s    *dirac.State
	d    dirac.Decision
	pool *feat.Pool
}

// featPositions plays the real kernel alone (random policy) on FDN pairings
// and keeps every decision's position.
func featPositions(b *testing.B, n int) []featPos {
	c, pool := testCorpus(b)
	var out []featPos
	for _, p := range Pairings(pool, 60, 23) {
		m, tbl, err := SetMatchup(rawSet("fdn"), c, p.A, p.B, 0)
		if err != nil {
			continue
		}
		fp, err := FeatPool(c.Reg, tbl, nil)
		if err != nil {
			b.Fatal(err)
		}
		side, err := RealKernel(tbl, nil, Config{PassElision: true})(m, p.Seed)
		if err != nil {
			continue
		}
		r := side.(*realSide)
		rng := rand.New(rand.NewPCG(p.Seed, 1))
		func() {
			defer func() { recover() }() // an unimplemented op ends the game
			for i := 0; i < 400 && len(out) < n; i++ {
				r.s.StepInto(&r.d)
				if r.d.Kind == dirac.DkNone {
					return
				}
				s := new(dirac.State)
				s.CopyFrom(&r.s)
				out = append(out, featPos{s: s, d: r.d, pool: fp})
				r.s.Apply(r.d.Options[rng.IntN(int(r.d.N))])
			}
		}()
		if len(out) >= n {
			break
		}
	}
	return out
}

// BenchmarkFeatEncode is the kernel-side encode of one position: the state
// and every option of its decision.
func BenchmarkFeatEncode(b *testing.B) {
	pos := featPositions(b, 2000)
	encs := map[*feat.Pool]*feat.Encoder{}
	for _, p := range pos {
		if encs[p.pool] == nil {
			encs[p.pool] = feat.NewEncoder(p.pool)
		}
	}
	var opts []feat.Option
	nOpts := 0
	for _, p := range pos {
		nOpts += int(p.d.N)
	}
	b.Run("state", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			p := &pos[i%len(pos)]
			encs[p.pool].EncodeState(p.s, &p.d)
		}
	})
	b.Run("state+options", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			p := &pos[i%len(pos)]
			e := encs[p.pool]
			e.EncodeState(p.s, &p.d)
			opts = e.EncodeOptions(&p.d, opts)
		}
	})
	incs := map[*feat.Pool]*feat.Encoder{}
	for _, p := range pos {
		if incs[p.pool] == nil {
			incs[p.pool] = feat.NewEncoder(p.pool)
		}
	}
	b.Run("inc-state", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			p := &pos[i%len(pos)]
			incs[p.pool].EncodeStateInc(p.s, &p.d)
		}
	})
	b.Run("inc-state+options", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			p := &pos[i%len(pos)]
			e := incs[p.pool]
			e.EncodeStateInc(p.s, &p.d)
			opts = e.EncodeOptions(&p.d, opts)
		}
	})
	var st feat.IncStats
	for _, e := range incs {
		st.Encodes += e.IncStats.Encodes
		st.Rebuilt += e.IncStats.Rebuilt
		st.Reused += e.IncStats.Reused
		st.Fallback += e.IncStats.Fallback
	}
	b.Logf("incremental: %d encodes, %d cards rebuilt, %d reused, %d full fallbacks", st.Encodes, st.Rebuilt, st.Reused, st.Fallback)
	b.Logf("%d positions, %.2f options each", len(pos), float64(nOpts)/float64(len(pos)))
}
