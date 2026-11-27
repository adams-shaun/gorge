package dirac

import (
	"strings"
	"testing"
	"unsafe"
)

// TestPCMemoEntSize pins the memo entry at 16 bytes: 192 slots stay 3 KB of L1.
func TestPCMemoEntSize(t *testing.T) {
	if n := unsafe.Sizeof(pcMemoEnt{}); n != 16 {
		t.Fatalf("pcMemoEnt is %d bytes, want 16", n)
	}
}

// pcMemoStale is the shadow check's panic text (mana_pcmemo.go).
const pcMemoStale = "manaPCGen memo answered a stale view"

// playEDHShadow plays the edh pods of BenchmarkTransitionEDH4 for n
// transitions with the shadow check on. A kernel panic other than the shadow
// check's ends that game (the bench's ~4% "autopay found no plan" games) and the
// next starts; the shadow check's is returned. It reports the memo's hits.
func playEDHShadow(t *testing.T, n int) (stale string, hits uint32) {
	t.Helper()
	pods := edhPods(t, 16)
	DebugPCMemoShadow = true
	t.Cleanup(func() { DebugPCMemoShadow = false })
	var s State
	var d Decision
	var pol RandomPolicy
	g := 0
	start := func() {
		pods[g%len(pods)].init(&s, uint64(g)+1)
		pol = NewRandomPolicy(uint64(g) + 1)
	}
	start()
	for i := 0; i < n && stale == ""; {
		func() {
			defer func() {
				if r := recover(); r != nil {
					if msg, ok := r.(string); ok && strings.Contains(msg, pcMemoStale) {
						stale = msg
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
	if s.pool != nil {
		hits = s.pool.pcm.hits
	}
	return stale, hits
}

// TestPCMemoShadowEDH plays the Commander pods with every memo hit recomputed:
// a hit that differs from the uncached view panics. The memo must also answer
// (a memo that never hits proves nothing).
func TestPCMemoShadowEDH(t *testing.T) {
	stale, hits := playEDHShadow(t, 40000)
	if stale != "" {
		t.Fatalf("shadow check: %s", stale)
	}
	if hits == 0 {
		t.Fatal("the memo never answered")
	}
	t.Logf("%d memo hits, all equal to the uncached view", hits)
}

// TestStaticCacheShadowMutation drops the memo's invalidation (the opening's
// generation bump and the object-write key): entries then outlive the walk
// that wrote them, and the shadow check must catch the stale views.
func TestStaticCacheShadowMutation(t *testing.T) {
	debugPCMemoNoInval = true
	t.Cleanup(func() { debugPCMemoNoInval = false })
	if stale, _ := playEDHShadow(t, 40000); stale == "" {
		t.Fatal("the memo kept its entries across decisions and the shadow check saw no stale view")
	}
}

// TestPCMemoOffMatchesOn plays the same pods with the memo off and on and
// requires the same decisions, option for option.
func TestPCMemoOffMatchesOn(t *testing.T) {
	pods := edhPods(t, 4)
	play := func(off bool) (h uint64) {
		DebugPCMemoOff = off
		defer func() { DebugPCMemoOff = false }()
		var s State
		var d Decision
		for g := range pods {
			pods[g].init(&s, uint64(g)+1)
			pol := NewRandomPolicy(uint64(g) + 1)
			func() {
				defer func() { _ = recover() }() // a kernel panic ends the game in both runs alike
				for i := 0; i < 3000; i++ {
					s.StepInto(&d)
					if d.Kind == DkNone {
						return
					}
					for j := 0; j < int(d.N); j++ {
						h = h*1099511628211 ^ uint64(d.Options[j])
					}
					s.Apply(pol.Pick(&d))
				}
			}()
		}
		return h
	}
	if off, on := play(true), play(false); off != on {
		t.Fatalf("decision streams differ: memo off %x, on %x", off, on)
	}
}
