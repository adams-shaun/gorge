package main

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestTyCacheShadowPods plays the substituted pods under LockstepConfig's
// auto-pay surface, the heuristic bot on every seat and then the random
// policy, with dirac's derived-types cache recomputing every hit
// (dirac.DebugTyCacheShadow): no hit may differ from the uncached answer.
func TestTyCacheShadowPods(t *testing.T) {
	reg, p := testPod(t)
	dirac.DebugTyCacheShadow = true
	t.Cleanup(func() { dirac.DebugTyCacheShadow = false })
	for _, bot := range []bool{true, false} {
		dirac.DebugTyCacheHits, dirac.DebugTyCacheMisses, dirac.DebugTyCacheBad = 0, 0, 0
		gm := &game{cfg: dirac.LockstepConfig(dirac.SurfaceAutopay)}
		gm.cfg.Commander = true
		if bot {
			gm.useHBot(reg, p)
		}
		var sum summary
		for g := 0; g < 4; g++ {
			gm.playOne(p, 11, g, &sum)
		}
		if dirac.DebugTyCacheBad != 0 {
			t.Fatalf("hbot %v: %d of %d cache hits stale", bot, dirac.DebugTyCacheBad, dirac.DebugTyCacheHits)
		}
		// no hit is not a failure: a pod table whose type statics read more
		// than objects (Duplicant, Goddric, Cloaked Reveler) computes every ask
		t.Logf("hbot %v, %d games, %d decisions (%d errors): %d hits, %d misses, %.2f hits per decision, none stale", bot, sum.Games, sum.Decisions,
			sum.Errors, dirac.DebugTyCacheHits, dirac.DebugTyCacheMisses, float64(dirac.DebugTyCacheHits)/float64(max(sum.Decisions, 1)))
	}
}
