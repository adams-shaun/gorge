package dirac

import "testing"

// A full pool stalls with its own reason instead of dropping the insert
// a generated table at its next step, the core at once.
func TestCapFullStallsWithReason(t *testing.T) {
	s := new(State)
	s.g.on = true
	for i := 0; i < maxUsed; i++ {
		s.markUsed(Obj(i), 0)
	}
	if s.Stalled() {
		t.Fatalf("stalled before the pool was full: %s", StallName(s.StallReason()))
	}
	s.markUsed(Obj(maxUsed), 0)
	s.markUsed(Obj(maxUsed+1), 0) // a later full insert keeps the first reason
	if s.StallReason() != StallUsed || s.over {
		t.Fatalf("generated table: reason %q over %v, want used and not yet over", StallName(s.StallReason()), s.over)
	}
	s.remember(1)
	for i := 0; i <= maxRem; i++ {
		s.remember(Obj(i + 1))
	}
	if s.StallReason() != StallUsed {
		t.Fatalf("the first reason was replaced: %q", StallName(s.StallReason()))
	}

	k1 := new(State)
	for i := 0; i <= maxRem; i++ {
		k1.remember(Obj(i + 1))
	}
	if k1.StallReason() != StallRemember || !k1.over || k1.win != -1 {
		t.Fatalf("core: reason %q over %v win %d, want remember, over, draw", StallName(k1.StallReason()), k1.over, k1.win)
	}
	// every defined reason has a name: TestRegistryNamesComplete
}
