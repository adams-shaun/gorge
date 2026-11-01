package dirac

import "testing"

// TestSlotMask covers the slot-set helpers at whatever MaxPerm is: the
// slots used straddle every word boundary there is (none at MaxPerm <= 64).
func TestSlotMask(t *testing.T) {
	var picks []Slot
	for _, sl := range []Slot{0, 1, 62, 63, 64, 65, 127, 128, MaxPerm / 2, MaxPerm - 2, MaxPerm - 1} {
		if int(sl) < MaxPerm {
			dup := false
			for _, p := range picks {
				dup = dup || p == sl
			}
			if !dup {
				picks = append(picks, sl)
			}
		}
	}
	want := map[Slot]bool{}
	for _, sl := range picks {
		want[sl] = true
	}

	var m slotMask
	if m.any() || m.count() != 0 || m.full() || m.many() {
		t.Fatal("zero mask is not empty")
	}
	// set in descending order so iteration order is not insertion order
	for i := len(picks) - 1; i >= 0; i-- {
		m = m.with(picks[i])
	}
	if !m.any() || m.count() != len(want) || m.many() != (len(want) >= 2) {
		t.Fatalf("count %d, want %d", m.count(), len(want))
	}
	for sl := 0; sl < MaxPerm; sl++ {
		if m.has(Slot(sl)) != want[Slot(sl)] {
			t.Fatalf("has(%d) = %v", sl, m.has(Slot(sl)))
		}
		if slotBit(Slot(sl)).count() != 1 || !slotBit(Slot(sl)).has(Slot(sl)) {
			t.Fatalf("slotBit(%d)", sl)
		}
	}
	// out-of-range slots are never members, and setting one is a no-op
	if m.has(NoSlot) || slotBit(NoSlot).any() {
		t.Fatal("NoSlot is a member")
	}
	c := m
	c = c.with(NoSlot)
	c = c.without(NoSlot)
	if c != m {
		t.Fatal("NoSlot set/clr changed the mask")
	}

	// the iteration idiom: ascending slot order, every member once
	var got []Slot
	for x := m; x.any(); x = x.rest() {
		got = append(got, x.first())
	}
	for i := 1; i < len(got); i++ {
		if got[i] <= got[i-1] {
			t.Fatalf("iteration order not ascending: %v", got)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("iterated %d, want %d", len(got), len(want))
	}
	for _, sl := range got {
		if !want[sl] {
			t.Fatalf("iterated %d, not a member", sl)
		}
	}
	if (slotMask{}).rest() != (slotMask{}) {
		t.Fatal("rest of empty")
	}
	if got[0] != 0 || got[len(got)-1] != MaxPerm-1 || m.first() != 0 {
		t.Fatalf("first %d last %d", got[0], got[len(got)-1])
	}

	// set algebra
	hi := slotBit(MaxPerm - 1)
	lo := slotBit(0)
	both := lo.or(hi)
	if both.count() != 2 || !both.many() || both.and(hi) != hi || both.andNot(hi) != lo {
		t.Fatal("or/and/andNot")
	}
	m = m.without(0)
	m = m.without(MaxPerm - 1)
	if m.has(0) || m.has(MaxPerm-1) || m.count() != len(want)-2 {
		t.Fatal("clr")
	}
	// the pointer forms, on a mask in memory (as State's fields are)
	var h struct{ m slotMask }
	h.m = m
	h.m.set(MaxPerm - 1)
	h.m.set(0)
	h.m.clr(0)
	h.m.set(NoSlot)
	h.m.clr(NoSlot)
	if !h.m.has(MaxPerm-1) || h.m.has(0) || h.m != m.with(MaxPerm-1) {
		t.Fatal("set/clr")
	}

	// full / not over exactly MaxPerm slots
	var f slotMask
	for sl := 0; sl < MaxPerm; sl++ {
		if f.full() {
			t.Fatalf("full at %d slots", sl)
		}
		f = f.with(Slot(sl))
	}
	if !f.full() || f.count() != MaxPerm || f.not().any() {
		t.Fatal("full")
	}
	f = f.without(MaxPerm - 1)
	if f.full() || f.not() != hi || f.not().first() != MaxPerm-1 {
		t.Fatal("not")
	}
	if (slotMask{}).not() != f.or(hi) {
		t.Fatal("not of empty")
	}
}
