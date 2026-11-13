package oracle

import "testing"

// TestAffordableTargetPolicyClosesTheAsk: Officious Interrogation's target ask
// prices {W}{U} per target beyond the first. The engine publishes the
// affordable count (engine Decision.AffordableTargets, carried to the oracle
// as CDecision.TargetLeft); a policy that fires one more target makes the cast
// unpayable, and the CR 733.1 reversal leaves gorge's and the kernel's
// payment-window taps in a different colour order (std-mkm pair 632, first
// divergence projection Pool[0][0]). kfuzz's random policy wraps itself in
// AffordableTargetPolicy, which must close the answer with Done at zero, as
// bots/botpolicy already does. The raw policies stay untouched so tests that
// deliberately drive a CR 733.1 reversal keep reachable.
func TestAffordableTargetPolicyClosesTheAsk(t *testing.T) {
	targets := []Canon{
		Pack(ActTarget, RefNone, RefPlayer, 0),
		Pack(ActTarget, RefNone, RefPlayer+1, 0),
	}
	done := Pack(ActDone, RefNone, RefNone, 0)
	open := &CDecision{Kind: DTarget, Player: 0, Options: append(append([]Canon{}, targets...), done)}

	// Precondition: with no budget stated the random policy fires a target
	// for at least one seed, so the capped assertions below can be observed
	// to fail without the wrapper.
	fired := false
	for seed := uint64(1); seed <= 32; seed++ {
		if p := RandomPolicy(seed).Pick(&PickCtx{D: open}); p.Kind() == ActTarget {
			fired = true
		}
	}
	if !fired {
		t.Fatal("RandomPolicy never picked a target without a budget; the test cannot observe the fix")
	}

	capped := *open
	capped.TargetLimited = true
	capped.TargetLeft = 0
	for seed := uint64(1); seed <= 32; seed++ {
		pol := AffordableTargetPolicy(RandomPolicy(seed))
		if got := pol.Pick(&PickCtx{D: &capped}); got != done {
			t.Fatalf("seed %d picked %v, want Done with no affordable target left", seed, got)
		}
	}

	// One affordable pick is still offered: a target or the Done that closes
	// the answer, never a second target.
	one := *open
	one.TargetLimited = true
	one.TargetLeft = 1
	for seed := uint64(1); seed <= 32; seed++ {
		pol := AffordableTargetPolicy(RandomPolicy(seed))
		if got := pol.Pick(&PickCtx{D: &one}); got != done && got.Kind() != ActTarget {
			t.Fatalf("budget 1 seed %d picked %v, want a target or Done", seed, got)
		}
	}
}
