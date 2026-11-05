package search

import (
	"math"
	"math/rand/v2"
)

// Generation-mode exploration (azmcts Options.Noise / Options.Sample, the
// :explore spec key): Noise mixes symmetric Dirichlet(DirichletAlpha)
// noise into the root's tree prior with weight DirichletEps (the recorded
// Result.Prior stays the clean prior, as azmcts records it); Sample picks
// the root move with probability proportional to its visits instead of the
// argmax. Both draw from one PCG stream seeded from the decision seed, so a
// game stays a pure function of its seeds. The zero value (both off) is the
// eval behaviour; the draws are not bit-identical to azmcts's.

// explore seeds the stream and, under Noise, draws the noisy root prior
// into sr.noisy (per Search, shared by every CachedWorlds tree). land is a
// LandSearch land root: its noisy prior is the clean one, no draw
// (landsearch.go).
func (sr *Searcher) explore(n int, land bool) {
	if !sr.Opts.Noise && !sr.Opts.Sample {
		return
	}
	if sr.rng == nil {
		sr.rng = rand.New(&sr.pcg)
	}
	sr.pcg.Seed(sr.seed^sr.Opts.ChoiceSalt, sr.seed^0x9e3779b97f4a7c15)
	if !sr.Opts.Noise {
		return
	}
	if land {
		copy(sr.noisy[:n], sr.rootPt.prior[:n])
		return
	}
	alpha, eps := sr.Opts.DirichletAlpha, sr.Opts.DirichletEps
	if alpha <= 0 {
		alpha = 0.3
	}
	if eps <= 0 || eps > 1 {
		eps = 0.25
	}
	sum := 0.0
	for i := 0; i < n; i++ {
		sr.noisy[i] = gammaDraw(sr.rng, alpha)
		sum += sr.noisy[i]
	}
	for i := 0; i < n; i++ {
		eta := 1 / float64(n)
		if sum > 0 {
			eta = sr.noisy[i] / sum
		}
		sr.noisy[i] = (1-eps)*sr.rootPt.prior[i] + eps*eta
	}
}

// noiseRoot replaces the priors of the root node's edges (one per root
// candidate, in candidate order) with the noisy prior.
func (sr *Searcher) noiseRoot(top int32) {
	if !sr.Opts.Noise || top < 0 {
		return
	}
	i := 0
	for e := sr.t.nodes[top].first; e >= 0 && i < sr.rootPt.n; e = sr.t.edges[e].sib {
		sr.t.edges[e].prior = sr.noisy[i]
		i++
	}
}

// sampleVisits draws a root index among the first n with probability
// proportional to its visits (Result.Visits, or the pruned Target); best
// when nothing has a visit.
func (sr *Searcher) sampleVisits(n int, visits *[MaxCands]int32, best int) int {
	total := 0
	for i := 0; i < n; i++ {
		total += int(visits[i])
	}
	if total <= 0 {
		return best
	}
	r := sr.rng.IntN(total)
	for i := 0; i < n; i++ {
		r -= int(visits[i])
		if r < 0 {
			return i
		}
	}
	return best
}

// gammaDraw is azmcts gammaDraw: Marsaglia and Tsang's Gamma(alpha, 1)
// sampler; alpha < 1 uses Gamma(alpha) = Gamma(alpha+1) * U^(1/alpha).
func gammaDraw(rng *rand.Rand, alpha float64) float64 {
	if alpha < 1 {
		u := rng.Float64()
		for u == 0 {
			u = rng.Float64()
		}
		return gammaDraw(rng, alpha+1) * math.Pow(u, 1/alpha)
	}
	d := alpha - 1.0/3
	c := 1 / math.Sqrt(9*d)
	for {
		x := rng.NormFloat64()
		v := 1 + c*x
		if v <= 0 {
			continue
		}
		v = v * v * v
		u := rng.Float64()
		if u < 1-0.0331*x*x*x*x || math.Log(u) < 0.5*x*x+d*(1-v+math.Log(v)) {
			return d * v
		}
	}
}
