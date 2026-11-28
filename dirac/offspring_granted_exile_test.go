package dirac

import "testing"

// A creature cast with Zinnia's GRANTED offspring that leaves the battlefield with the
// granted trigger still on the stack still leaves its 1/1 copy, minted when the trigger
// resolves (CR 702.175a pay-time fact, CR 608.2h). gorge's twin is
// TestOffspringGrantedSourceThatLeftStillMints; the printed twin is
// TestOffspringSourceThatLeftStillMints.
func TestOffspringGrantedSourceThatLeftStillMints(t *testing.T) {
	sc, id := giftScen(t, "Zinnia, Valley's Voice", "Pawpatch Recruit", "Forest", "Island", "Mountain", "Plains")
	for _, n := range []string{"Forest", "Forest", "Forest", "Forest", "Island", "Mountain", "Plains"} {
		sc.perm(id[n], 0)
	}
	sc.perm(id["Zinnia, Valley's Voice"], 0)
	pr := sc.hand(id["Pawpatch Recruit"], 0)
	sc.castMode(t, pr, 1)
	sc.passUntilOnBF(t, pr)
	if sc.s.sp == 0 {
		t.Fatal("no granted offspring trigger on the stack")
	}
	sc.s.destroy(sc.k, sc.slot(pr))
	if n := len(sc.copiesOf(id["Pawpatch Recruit"])); n != 0 {
		t.Fatalf("%d copies before the trigger resolved, want 0", n)
	}
	sc.resolveAll(t)
	cs := sc.copiesOf(id["Pawpatch Recruit"])
	if len(cs) != 1 {
		t.Fatalf("%d copies, want the token", len(cs))
	}
	if p, _ := sc.s.pt(sc.k, cs[0]); p != 1 {
		t.Fatalf("the token has power %d, want 1", p)
	}
}
