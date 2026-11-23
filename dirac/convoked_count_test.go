package dirac

import "testing"

// TestConvokedAmountExcludesImproviseAndHarmonizePicks: one cast that picks in
// a convoke mode (0-5), improvise (6) and harmonize (7) records a
// Convoked$Amount that equals the stamped convoked set's size; the improvise
// and harmonize picks are in neither.
func TestConvokedAmountExcludesImproviseAndHarmonizePicks(t *testing.T) {
	sc, _, _ := convokedScen(t)
	b := sc.s.bld()
	base := int(b.pickN)
	if base != 1 {
		t.Fatalf("precondition: %d picks, want the one Elves pick", base)
	}
	b.picks[base], b.pickKs[base] = Obj(base+10), convokePickMask|pickKind(6)
	b.picks[base+1], b.pickKs[base+1] = Obj(base+11), convokePickMask|pickKind(7)
	b.pickN = uint8(base + 2)
	if sc.s.convokePickCount() != 3 {
		t.Fatalf("precondition: every tap-pay pick counts for the cost side, got %d", sc.s.convokePickCount())
	}
	sc.s.stampConvoked(sc.k)
	e := sc.s.bld().e
	set := sc.s.convokedOf(&e)
	if len(set) != 1 {
		t.Fatalf("convoked set %v, want just the one convoke pick", set)
	}
	if got := sc.s.convokedAmount(); got != len(set) {
		t.Fatalf("Convoked$Amount %d, stamped set size %d: the count must exclude modes 6/7", got, len(set))
	}
}
