package dirac

import (
	"strings"
	"testing"
)

// tyShadowRun is one shadow run's result.
type tyShadowRun struct {
	stale      string
	hits, bad  int
	cHit, cMis uint32
	gHit, gMis uint32
	steps      int
}

// playTyShadow plays pods (game g is pod g%len(pods) under seed g+1+seed0,
// config cfg) for n transitions with the types cache's shadow check on. A
// kernel panic other than the shadow check's ends that game and the next
// starts.
func playTyShadow(t *testing.T, pods []edhPod, cfg Config, n int, seed0 uint64) (r tyShadowRun) {
	t.Helper()
	DebugTyCacheShadow, DebugTyCacheHits, DebugTyCacheBad = true, 0, 0
	t.Cleanup(func() { DebugTyCacheShadow = false })
	var s State
	var d Decision
	var pol RandomPolicy
	g := 0
	start := func() {
		s.InitN(pods[g%len(pods)].tbl, pods[g%len(pods)].decks, MaxSeats, uint64(g)+1+seed0, cfg)
		pol = NewRandomPolicy(uint64(g) + 1 + seed0)
	}
	start()
	for i := 0; i < n && r.stale == ""; {
		func() {
			defer func() {
				if x := recover(); x != nil {
					if msg, ok := x.(string); ok && strings.Contains(msg, "typesGen cache answered") {
						r.stale = msg
						return
					}
					i++
					g++
					start()
				}
			}()
			for ; i < n; i++ {
				s.StepInto(&d)
				if d.Kind == DkNone {
					g++
					start()
					s.StepInto(&d)
				}
				s.Apply(pol.Pick(&d))
			}
		}()
	}
	if p := s.pool; p != nil {
		r.cHit, r.cMis = p.tyc.hits, p.tyc.miss
		r.gHit, r.gMis = p.grc.hits, p.grc.miss
	}
	r.hits, r.bad, r.steps = DebugTyCacheHits, DebugTyCacheBad, n
	return r
}

func checkTyRun(t *testing.T, r tyShadowRun) {
	t.Helper()
	if r.stale != "" || r.bad != 0 {
		t.Fatalf("shadow check: %d stale of %d hits: %s", r.bad, r.hits, r.stale)
	}
	if r.hits == 0 {
		t.Fatal("the types cache never answered")
	}
	t.Logf("%d transitions: types %d hits / %d misses, grant lists %d hits / %d misses, every hit equal to the uncached answer",
		r.steps, r.cHit, r.cMis, r.gHit, r.gMis)
}

// The bench's Commander pods with every cache hit recomputed, in chunks (the
// per-test budget): the bench's own config, then LockstepConfig's auto-pay
// surface.
func TestTyCacheShadowEDH0(t *testing.T) {
	checkTyRun(t, playTyShadow(t, edhPods(t, 16)[:8], Config{Mulligans: 2}, 12000, 0))
}

func TestTyCacheShadowEDH1(t *testing.T) {
	checkTyRun(t, playTyShadow(t, edhPods(t, 16)[8:], Config{Mulligans: 2}, 12000, 0))
}

func TestTyCacheShadowEDHAutopay(t *testing.T) {
	checkTyRun(t, playTyShadow(t, edhPods(t, 16), LockstepConfig(SurfaceAutopay), 12000, 100))
}

// TestTyCacheShadowFDN is the FDN soak's loop (fdnGame) over the committed
// bench decks with the shadow check on.
func TestTyCacheShadowFDN(t *testing.T) {
	DebugTyCacheShadow, DebugTyCacheHits, DebugTyCacheBad = true, 0, 0
	t.Cleanup(func() { DebugTyCacheShadow = false })
	decks := benchDecks(t, "fdn")
	for g := 0; g < 40; g++ {
		fdnGame(t, decks, g, 30000)
	}
	if DebugTyCacheBad != 0 {
		t.Fatalf("%d stale of %d hits", DebugTyCacheBad, DebugTyCacheHits)
	}
	t.Logf("40 pairings: %d hits checked, none stale", DebugTyCacheHits)
}

// TestTyCacheShadowMutation drops the context check: entries then outlive
// the turn, step, life and scratch changes they were written under, and the
// shadow check must catch the stale answers.
func TestTyCacheShadowMutation(t *testing.T) {
	debugTyCacheNoInval = true
	t.Cleanup(func() { debugTyCacheNoInval = false })
	if r := playTyShadow(t, edhPods(t, 16), Config{Mulligans: 2}, 20000, 0); r.stale == "" {
		t.Fatalf("no stale answer in %d hits with the context check dropped", r.hits)
	}
}

// TestTyCacheExcludesNonObjectSources: on the bench pods that carry
// Duplicant, whose type static's condition reads an expression over its
// exiled-with set, the table stays in the cache with Duplicant's card
// excluded per source (tySelfOnly), so tySrcs puts Duplicant's slot in exM, which
// both caches compute every ask for. Random play never moved
// Duplicant's types without a write to it (7.3M hits with the exclusion
// dropped and none stale), so the exclusion is held by this check and
// tySelfOnly's argument rather than by a mutation the soak can catch.
func TestTyCacheExcludesNonObjectSources(t *testing.T) {
	all := edhPods(t, 16)
	seen := 0
	// The pods are found, not named by index: a pool change moves which pods hold Duplicant
	// (11-28: a batch went red twice on the six pinned indices).
	for i := range all {
		var s State
		all[i].init(&s, uint64(i)+1)
		k := s.k()
		dup := -1
		for c := range k.t.Cards {
			if k.t.Cards[c].Name == "Duplicant" {
				dup = c
			}
		}
		if dup < 0 || !k.tyCache {
			continue // no Duplicant, or a table whose type statics read more than objects computes every ask
		}
		seen++
		if k.tyEx == nil || !k.tyEx[dup] {
			t.Fatalf("pod %d: the table is cached without Duplicant excluded (ex %v)", i, k.tyEx != nil)
		}
	}
	if seen == 0 {
		t.Fatal("no pod holds Duplicant on a cached table: the exclusion is untested")
	}
	t.Logf("%d of %d pods hold Duplicant on a cached table", seen, len(all))
}
