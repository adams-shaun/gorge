package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// A doubling candidate picked as made beside a plus-N addition applies its own
// line's word (Repl.A: Twice 2, City on Fire / Fiery Emancipation Thrice 3),
// not a hard-coded 2. The pick-as-made walk carries the running amount and
// applies each picked replacement at the pick (replOrderPick's roPlusPend arm,
// ncPlusPickNC; CR 616.1e), so a Thrice picked first must triple.

// dblPickFixture is a plus-N host (candidate 0) beside a Thrice host
// (candidate 1), both on seat 0's battlefield.
func dblPickFixture(t *testing.T) (*scen, Obj, Obj) {
	t.Helper()
	tb := newSPTable()
	plus := tb.host("Test Plus", kop.Repl{Kind: kop.ReplDamagePlus, A: 1,
		C: kop.ReplDmgPlayer | kop.ReplDmgPerm, Filter: tb.filter(kop.Filter{})})
	thrice := tb.host("Test Thrice", kop.Repl{Kind: kop.ReplDoubleDamageToOpps, A: 3,
		C: kop.ReplDmgPlayer | kop.ReplDmgPerm, Filter: tb.filter(kop.Filter{})})
	sc := tb.scen(t)
	return sc, sc.perm(plus, 0), sc.perm(thrice, 0)
}

// TestReplDblPickNoncombatThrice: the parked noncombat walk's doubling pick
// applies the picked host's own multiplier (ncPlusPickNC).
func TestReplDblPickNoncombatThrice(t *testing.T) {
	sc, _, thrice := dblPickFixture(t)
	ro := &sc.s.g.ro
	ro.cand[0] = thrice
	ro.an = 2
	if got := sc.s.dblCandMult(sc.k, 0); got != 3 {
		t.Fatalf("precondition: the candidate's multiplier = %d, want 3", got)
	}
	sc.s.ncPlusPickNC(0)
	if ro.an != 6 {
		t.Fatalf("Thrice picked as made: an = %d, want 6", ro.an)
	}
}

// TestReplDblPickCombatThriceFirst: the combat order walk's doubling pick
// applies the picked host's own multiplier when a plus-N candidate is still
// unpicked (replOrderPick's roPlusPend arm); dealAsked applies only the last
// remaining candidate, so picking the Thrice first is the path that was wrong.
func TestReplDblPickCombatThriceFirst(t *testing.T) {
	sc, plus, thrice := dblPickFixture(t)
	src := sc.perm(v.GrizzlyBears, 0)
	ro := &sc.s.g.ro
	ro.on = true
	ro.ac, ro.n = 2, 2
	ro.cand[0], ro.cand[1] = plus, thrice
	ro.plus = 1 // candidate 0 is the addition; candidate 1 is the doubling
	ro.an = 2
	ro.askS, ro.askD = sc.slot(src), NoSlot
	ro.actl, ro.p = 0, 1
	if !sc.s.roPlusPend() {
		t.Fatal("precondition: the ask is not the plus-beside-doubling walk")
	}
	if got := sc.s.dblCandMult(sc.k, 1); got != 3 {
		t.Fatalf("precondition: the candidate's multiplier = %d, want 3", got)
	}
	sc.s.replOrderPick(MakeAction(AkChoose, TCode(thrice), 0, 1))
	if ro.used != 1<<1 {
		t.Fatalf("precondition: the pick was not recorded: used=%#x", ro.used)
	}
	if ro.an != 6 {
		t.Fatalf("Thrice picked first: an = %d, want 6", ro.an)
	}
}
