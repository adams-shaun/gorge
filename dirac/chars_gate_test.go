package dirac

import (
	"strings"
	"testing"
)

// TestCharsFxGateShadow plays the BenchmarkTransitionEDH4 pods with
// DebugCharsGateShadow on: behind every charsFxMay no the full collection
// and walk runs, and it must collect nothing and leave the result and the
// overflow state alone.
func TestCharsFxGateShadow(t *testing.T) {
	pods := edhPods(t, 16)
	DebugCharsGateShadow, charsGateShadowN = true, 0
	defer func() { DebugCharsGateShadow = false }()
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
				if msg, ok := r.(string); ok && strings.Contains(msg, "charsFxMay") {
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
	if charsGateShadowN == 0 {
		t.Fatal("the gate skipped no walk: the shadow checked nothing")
	}
	t.Logf("%d games, %d gated walks checked", g+1, charsGateShadowN)
}
