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
	t.Logf("%d transitions: %d hits / %d misses (%.1f%%), every hit equal to the uncached answer",
		r.steps, r.cHit, r.cMis, 100*float64(r.cHit)/float64(r.cHit+r.cMis))
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
