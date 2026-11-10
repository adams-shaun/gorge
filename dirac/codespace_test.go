package dirac

import "testing"

// TestCodeSpaceDisjoint holds the target-code space to its partition in
// whichever size class the build has: objects 0..MaxObj-1, stack abilities
// StackTarget0..PlayerTarget0-1, one player code per seat, then the departed
// stack target and none. Every byte falls in exactly one range, and every
// seat's player code round-trips.
func TestCodeSpaceDisjoint(t *testing.T) {
	if MaxObj > int(StackTarget0) {
		t.Fatalf("MaxObj %d overlaps the stack codes at %#x", MaxObj, StackTarget0)
	}
	if int(PlayerTarget0)+MaxSeats > int(StackTargetGone) || StackTargetGone == NoTarget {
		t.Fatalf("player codes %#x..+%d run into %#x/%#x", PlayerTarget0, MaxSeats, StackTargetGone, NoTarget)
	}
	if maxStackTgt != 16 {
		t.Errorf("%d stack codes, want 16", maxStackTgt)
	}
	for c := 0; c < 256; c++ {
		b := uint8(c)
		n := 0
		for _, in := range []bool{isObjTarget(b), isStackTarget(b), isPlayerTarget(b), b == StackTargetGone, b == NoTarget} {
			if in {
				n++
			}
		}
		// the bytes between MaxObj and StackTarget0 (none today) are unused
		if n != 1 && !(n == 0 && c >= MaxObj && c < int(StackTarget0)) {
			t.Errorf("code %#x is in %d ranges", c, n)
		}
	}
	for p := uint8(0); p < MaxSeats; p++ {
		c := PlayerCode(p)
		if !isPlayerTarget(c) || CodeSeat(c) != p {
			t.Errorf("seat %d: code %#x does not round-trip", p, c)
		}
		var o object
		o.acts = objActsMask
		o.setOwner(p)
		if o.owner() != p || o.activations() != objActsMask {
			t.Errorf("seat %d: owner/activations pack gives %d/%d", p, o.owner(), o.activations())
		}
	}
	if objOwnerBits&objActsMask != 0 || objOwnerBits|objActsMask != 0xff || objOwnerBits>>objOwnerShift != MaxSeats-1 {
		t.Errorf("acts bits %#x/%#x/shift %d do not split the byte for %d seats", objOwnerBits, objActsMask, objOwnerShift, MaxSeats)
	}
}
