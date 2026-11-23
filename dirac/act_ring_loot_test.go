package dirac

import "testing"

// TestRingLootFinishesOnce: the Ring emblem's level-2 loot resolves once.
// ringLootDiscard leaves the pop to its caller, and resolveTop's abDelayed
// arm already owns the delayed resolution's finish: an empty hand (no ask)
// must not finish it a second time. A second finishResolve decrements sp
// below zero (uint8, so 255), and the next projection reads past the stack.
func TestRingLootFinishesOnce(t *testing.T) {
	sc, _ := newRingBearerScen(t)
	s, k := sc.s, sc.k
	if s.pl[0].handN != 0 {
		t.Fatalf("handN = %d, want 0: the empty-hand arm is the one under test", s.pl[0].handN)
	}
	if dredgeOn(k) {
		t.Fatal("dredge is on: the loot would pose its dredge ask instead of finishing")
	}
	s.push(stackEntry{kind: seTrigger, ab: abDelayed, ctrl: 0, obj: NoObj, trigAmt: ringLootAmt})
	if s.sp != 1 {
		t.Fatalf("sp = %d, want 1 after the push", s.sp)
	}
	s.resolveTop(k)
	if s.sp != 0 {
		t.Fatalf("sp = %d, want 0: the loot finished twice", s.sp)
	}
	if s.res.active {
		t.Fatal("res.active is still true after the loot resolved")
	}
	if s.pl[0].handN != 0 {
		t.Fatalf("handN = %d, want 0: the loot drew one card and discarded it", s.pl[0].handN)
	}
}
