package rules

import "testing"

// The default object cap. WithDefaultObjectCap is the ONE helper every
// harness/host config goes through (testkit/bench's drive loop, the host's
// match config, cmd/mtgsim), so these pins are the contract every caller
// inherits: a caller with no budget gets DefaultMaxObjs, an explicit budget
// wins, and the Disabled opt-out is never overridden.

func TestWithDefaultObjectCapNilArmsDefault(t *testing.T) {
	t.Parallel()
	g := WithDefaultObjectCap(nil)
	if g == nil || g.MaxObjs != DefaultMaxObjs {
		t.Fatalf("nil guard armed %+v, want MaxObjs %d", g, DefaultMaxObjs)
	}
	if g.CycleEvents != 0 || g.RunawayEvents != 0 || g.MaxPeriod != 0 || g.Disabled {
		t.Fatalf("nil guard armed with non-default thresholds: %+v", g)
	}
}

func TestWithDefaultObjectCapFillsUnsetGuard(t *testing.T) {
	t.Parallel()
	in := &LoopGuard{CycleEvents: 6, MaxPeriod: 8}
	g := WithDefaultObjectCap(in)
	if g == in {
		t.Fatal("an unset guard must be copied, not filled in place")
	}
	if g.MaxObjs != DefaultMaxObjs || g.CycleEvents != 6 || g.MaxPeriod != 8 {
		t.Fatalf("fill lost fields: %+v", g)
	}
	if in.MaxObjs != 0 {
		t.Fatalf("the caller's guard was mutated: %+v", in)
	}
}

func TestWithDefaultObjectCapExplicitWins(t *testing.T) {
	t.Parallel()
	in := &LoopGuard{MaxObjs: 121}
	if g := WithDefaultObjectCap(in); g != in || g.MaxObjs != 121 {
		t.Fatalf("explicit budget overridden: %p %+v", g, in)
	}
}

func TestWithDefaultObjectCapDisabledUnchanged(t *testing.T) {
	t.Parallel()
	in := &LoopGuard{Disabled: true}
	if g := WithDefaultObjectCap(in); g != in || g.MaxObjs != 0 {
		t.Fatalf("the Disabled opt-out was overridden: %p %+v", g, in)
	}
}

func TestDefaultObjectCapIsAVolumeBound(t *testing.T) {
	t.Parallel()
	// The default cap must sit at or above the big-board population that
	// fixed the runaway reason at, and far above the measured ordinary
	// high-water (554) -- otherwise ordinary games trip it.
	if DefaultMaxObjs < RunawayBigBoardObjs {
		t.Fatalf("DefaultMaxObjs %d below the big-board population %d", DefaultMaxObjs, RunawayBigBoardObjs)
	}
	if DefaultMaxObjs < 10_000 {
		t.Fatalf("DefaultMaxObjs %d below the documented volume bound", DefaultMaxObjs)
	}
}

func TestIsVolumeReasonClassification(t *testing.T) {
	t.Parallel()
	for _, r := range []string{ObjectCapReason, RunawayBigBoardReason} {
		if !IsVolumeReason(r) {
			t.Fatalf("%q should classify as volume", r)
		}
	}
	for _, r := range []string{"repeating cycle", "token plan", ""} {
		if IsVolumeReason(r) {
			t.Fatalf("%q should NOT classify as volume", r)
		}
	}
}
