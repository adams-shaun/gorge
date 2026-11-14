package dirac

import "testing"

// TestExtraBlockTableCapacity: the second-block table holds maxXblk blockers of
// two attackers, finds each by its blocker slot, frees a record on xblkDrop
// (and so on a slot's reuse), and refuses a record past the last.
func TestExtraBlockTableCapacity(t *testing.T) {
	sc := xbScen(t, Config{NoElide: true})
	s := sc.s
	if !s.xblkRoom() || s.xblkOf(5) != NoSlot {
		t.Fatal("a fresh table has no record and room")
	}
	for i := 0; i < maxXblk; i++ {
		if !s.xblkAdd(Slot(i), Slot(100+i), uint8(i+1)) {
			t.Fatalf("record %d of %d refused", i, maxXblk)
		}
	}
	if s.xblkRoom() || s.xblkAdd(Slot(60), 1, 1) {
		t.Fatal("a full table took a record")
	}
	for i := 0; i < maxXblk; i++ {
		if s.xblkOf(Slot(i)) != Slot(100+i) || s.xblkFind(Slot(i)).ord != uint8(i+1) {
			t.Fatalf("blocker %d: second attacker %v", i, s.xblkOf(Slot(i)))
		}
	}
	s.arenaClrSlot(3) // the slot takes a new permanent
	if s.xblkOf(3) != NoSlot || !s.xblkRoom() || !s.xblkAdd(60, 1, 9) || s.xblkOf(60) != 1 {
		t.Fatal("a reused slot's record was not freed for another blocker")
	}
	s.xblkClear()
	if !s.xblkRoom() || s.xblkOf(0) != NoSlot {
		t.Fatal("combat's end left a record")
	}
}
