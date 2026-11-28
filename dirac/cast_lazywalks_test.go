package dirac

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// The nested-walk shadow: scriptOffers builds a nested priority walk only when
// its candidate loop can read it (script_gen.go scriptOffersLazy). These tests
// hold the lazy path to the eager one it replaced (scriptOffersEager) in two
// ways: DebugShadowNestedWalks compares every call's options, spent cap,
// searched count and State against the eager twin run on a fork, and the
// decision stream of whole games played lazily must equal the one played
// with DebugEagerNestedWalks.

// edhStream plays n transitions of BenchmarkTransitionEDH4's loop (pods,
// seeds, policy, restarts on game over and on a kernel panic) under cfg and
// returns one hash per transition of the Decision posed and the pick.
func edhStream(t *testing.T, pods []edhPod, cfg Config, n int) []uint64 {
	t.Helper()
	var s State
	var d Decision
	g := 0
	var pol RandomPolicy
	start := func() {
		pods[g%len(pods)].initCfg(&s, uint64(g)+1, cfg)
		pol = NewRandomPolicy(uint64(g) + 1)
	}
	out := make([]uint64, 0, n)
	segment := func() {
		defer func() {
			if r := recover(); r != nil {
				if m, ok := r.(string); ok && strings.HasPrefix(m, "dirac: scriptOffers lazy") {
					t.Fatalf("transition %d (game %d): %s%s", len(out), g, m, shadowDiff())
				}
				out = append(out, 0xdead) // the kernel panic ends the game, as the bench's segment does
				g++
				start()
			}
		}()
		for len(out) < n {
			s.StepInto(&d)
			if d.Kind == DkNone {
				g++
				start()
				s.StepInto(&d)
			}
			a := pol.Pick(&d)
			out = append(out, decisionHash(&d, a))
			s.Apply(a)
		}
	}
	start()
	for len(out) < n {
		segment()
	}
	return out
}

func decisionHash(d *Decision, a Action) uint64 {
	h := uint64(14695981039346656037)
	mix := func(v uint64) {
		h ^= v
		h *= 1099511628211
	}
	mix(uint64(d.Kind))
	mix(uint64(d.Player))
	mix(uint64(d.N))
	for i := 0; i < int(d.N); i++ {
		mix(uint64(d.Options[i]))
	}
	mix(uint64(a))
	return h
}

// shadowDiff names the stateCore fields the last shadow mismatch differs in.
func shadowDiff() string {
	a, b := &shadowMismatch.lazy, &shadowMismatch.eager
	var names []string
	seen := map[string]bool{}
	av := reflect.ValueOf(&a.stateCore).Elem()
	bv := reflect.ValueOf(&b.stateCore).Elem()
	var walk func(x, y reflect.Value, path string)
	walk = func(x, y reflect.Value, path string) {
		switch x.Kind() {
		case reflect.Struct:
			for i := 0; i < x.NumField(); i++ {
				walk(x.Field(i), y.Field(i), path+"."+x.Type().Field(i).Name)
			}
		case reflect.Array:
			for i := 0; i < x.Len(); i++ {
				walk(x.Index(i), y.Index(i), fmt.Sprintf("%s[%d]", path, i))
			}
		default:
			if x.CanInt() && x.Int() != y.Int() || x.CanUint() && x.Uint() != y.Uint() || x.Kind() == reflect.Bool && x.Bool() != y.Bool() {
				if !seen[path] && len(names) < 20 {
					seen[path] = true
					names = append(names, path)
				}
			}
		}
	}
	walk(av, bv, "")
	if a.perms != b.perms {
		names = append(names, "perms")
	}
	return fmt.Sprintf(" (stateCore fields differing: %v)", names)
}

func withNestedHooks(t *testing.T, shadow, eager bool) {
	t.Helper()
	DebugShadowNestedWalks, DebugEagerNestedWalks = shadow, eager
	shadowCalls, shadowPlays = 0, 0
	t.Cleanup(func() { DebugShadowNestedWalks, DebugEagerNestedWalks = false, false })
}

// nestedShadowN is the transitions each EDH shadow test plays per run.
const nestedShadowN = 60000

func testNestedWalksEDH(t *testing.T, cfg Config) {
	pods := edhPods(t, 16)
	withNestedHooks(t, true, false)
	lazy := edhStream(t, pods, cfg, nestedShadowN) // every call checked against the eager twin
	t.Logf("%d scriptOffers calls shadowed, %d searched a play", shadowCalls, shadowPlays)
	if shadowPlays == 0 {
		t.Fatal("the shadow compared no call that searched a play")
	}
	withNestedHooks(t, false, true)
	eager := edhStream(t, pods, cfg, nestedShadowN)
	for i := range eager {
		if lazy[i] != eager[i] {
			t.Fatalf("transition %d: the lazy and eager nested walks pose different decisions", i)
		}
	}
}

// TestNestedWalksShadowEDH4: the EDH bench pods under Config{Mulligans: 2}.
func TestNestedWalksShadowEDH4(t *testing.T) {
	testNestedWalksEDH(t, Config{Mulligans: 2})
}

// TestNestedWalksShadowEDH4Autopay: the same pods on LockstepConfig's
// auto-pay surface, where scriptOffers' real walk runs.
func TestNestedWalksShadowEDH4Autopay(t *testing.T) {
	testNestedWalksEDH(t, edhAutopayConfig())
}
