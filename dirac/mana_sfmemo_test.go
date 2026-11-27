package dirac

import (
	"strings"
	"testing"
)

const sfMemoStale = "scriptFilt memo answered a stale mask"

// playEDHSF plays the bench's edh pods for n transitions with scriptFilt's
// shadow check on; it returns the shadow check's panic, if any, and the hits.
func playEDHSF(t *testing.T, n int) (stale string, hits uint32) {
	t.Helper()
	pods := edhPods(t, 16)
	DebugSFMemoShadow = true
	t.Cleanup(func() { DebugSFMemoShadow = false })
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
					if msg, ok := r.(string); ok && strings.Contains(msg, sfMemoStale) {
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
				if s.pool != nil {
					hits = max(hits, s.pool.sfm.hits)
				}
			}
		}()
	}
	return stale, hits
}

// TestSFMemoShadowEDH: every hit equals the recomputed mask, and the memo answers.
func TestSFMemoShadowEDH(t *testing.T) {
	stale, hits := playEDHSF(t, 40000)
	if stale != "" {
		t.Fatalf("shadow check: %s", stale)
	}
	if hits == 0 {
		t.Fatal("the memo never answered")
	}
	t.Logf("%d memo hits, all equal to the uncached mask", hits)
}

// TestSFMemoShadowMutation drops the invalidation; the shadow check must catch it.
func TestSFMemoShadowMutation(t *testing.T) {
	debugSFMemoNoInval = true
	t.Cleanup(func() { debugSFMemoNoInval = false })
	if stale, _ := playEDHSF(t, 40000); stale == "" {
		t.Fatal("the memo kept its entries across decisions and the shadow check saw no stale mask")
	}
}

// TestSFMemoOffMatchesOn: same option streams with the memo off and on.
func TestSFMemoOffMatchesOn(t *testing.T) {
	pods := edhPods(t, 4)
	play := func(off bool) (h uint64) {
		DebugSFMemoOff = off
		defer func() { DebugSFMemoOff = false }()
		var s State
		var d Decision
		for g := range pods {
			pods[g].init(&s, uint64(g)+1)
			pol := NewRandomPolicy(uint64(g) + 1)
			func() {
				defer func() { _ = recover() }()
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
