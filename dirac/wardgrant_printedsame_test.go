package dirac

import "testing"

// TestWardGrantEqualToPrintedStillTriggers: two Hexing Squelchers each print
// "Ward—Pay 2 life" and grant it to the other, so the targeted one holds two
// instances and both trigger (CR 113.2c, 702.21a), as gorge's
// checkGrantedWardTriggers counts one derived entry per printed line and
// queues every other.
func TestWardGrantEqualToPrintedStillTriggers(t *testing.T) {
	sc, id := synthScen(t, "std-ecl", "Hexing Squelcher", "Sear", "Mountain")
	s, k := sc.s, sc.k
	if k.gen == nil || !k.gen.anyWardGrant {
		t.Fatal("precondition: the table has no granted ward")
	}
	first := sc.perm(id["Hexing Squelcher"], 0)
	second := sc.perm(id["Hexing Squelcher"], 0)
	if first == second || !sc.onBF(first) || !sc.onBF(second) {
		t.Fatal("precondition: two Hexing Squelchers are not on the battlefield")
	}
	if got := wardAmt(&k.t.Cards[id["Hexing Squelcher"]]); got != -2 {
		t.Fatalf("precondition: printed ward amount %d, want the pay 2 life ward", got)
	}
	spell := sc.obj(id["Sear"], 1)
	e := &stackEntry{obj: spell, ctrl: 1}
	s.npend = 0
	s.targetedOne(k, e, TCode(first))
	want := int16(2) | wardGrantedBits | wardGrantedLifeBit
	if s.npend != 2 || s.pend[0].trigAmt != -2 || s.pend[1].trigAmt != want {
		t.Fatalf("pending %d: %+v, want the printed pay-2-life ward, then the granted one", s.npend, s.pend[:s.npend])
	}
}
