package dirac

import (
	"strings"
	"testing"
)

// TestGrantGateShadow plays the BenchmarkTransitionEDH4 pods with
// DebugGrantGateShadow on: behind every manaGrantMay no the full grant walk
// runs, and it must find no mana grant and leave the stall state alone.
func TestGrantGateShadow(t *testing.T) {
	pods := edhPods(t, 16)
	DebugGrantGateShadow, grantGateShadowN = true, 0
	defer func() { DebugGrantGateShadow = false }()
	var s State
	var d Decision
	var pol RandomPolicy
	g := 0
	start := func() {
		pods[g%len(pods)].init(&s, uint64(g)+1)
		pol = NewRandomPolicy(uint64(g) + 1)
	}
	const steps = 30000
	i := 0
	segment := func() {
		defer func() {
			if r := recover(); r != nil {
				if msg, ok := r.(string); ok && strings.Contains(msg, "manaGrantMay") {
					t.Fatalf("game %d step %d: %s", g, i, msg)
				}
				g++ // a kernel panic of the random policy: the bench abandons the game too
				start()
			}
		}()
		for ; i < steps; i++ {
			s.StepInto(&d)
			if d.Kind == DkNone {
				g++
				start()
				s.StepInto(&d)
			}
			s.Apply(pol.Pick(&d))
		}
	}
	start()
	for i < steps {
		segment()
	}
	if grantGateShadowN == 0 {
		t.Fatal("the gate skipped no grant walk: the shadow checked nothing")
	}
	t.Logf("%d games, %d gated grant walks checked", g+1, grantGateShadowN)
}
