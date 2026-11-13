package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// coyoteResolve resolves Driftgloom Coyote's enters trigger against the
// opponent's tgt and reports Coyote's +1/+1 counters afterwards.
func coyoteResolve(t *testing.T, sc *scen, coy CardID, tgt Obj) int {
	t.Helper()
	s := sc.s
	c := sc.perm(coy, 0)
	ab := -1
	for i, a := range sc.k.t.Cards[coy].Abilities {
		if a.Trig.Event != 0 {
			ab = i
		}
	}
	if ab < 0 {
		t.Fatal("Driftgloom Coyote has no trigger")
	}
	s.stack[0] = stackEntry{kind: seTrigger, obj: c, srcVer: s.objs[c].ver, card: coy, ab: uint8(ab), ctrl: 0,
		nt: 1, tgt: [kop.MaxTargets]uint8{uint8(tgt), uint8(NoTarget), uint8(NoTarget), uint8(NoTarget), uint8(NoTarget)}, tver: [kop.MaxTargets]uint8{s.objs[tgt].ver}, trigObj: c}
	s.sp = 1
	s.resolveTop(sc.k)
	if s.objs[tgt].zone != kop.ZoneExile {
		t.Fatalf("target zone %d, want exile", s.objs[tgt].zone)
	}
	return int(s.perms[sc.slot(c)].p1)
}

// TestCoyoteTargetLKI: Driftgloom Coyote's "if that creature had power 2 or
// less" reads the exiled target as it last existed on the battlefield (CR
// 608.2h; gorge conditionDefinedMet/targetCountersLKI substitutes its
// battlefield counters): a 1/1 with two +1/+1 counters (power 3) gives no
// counter, a plain 2/1 (power 2) gives one.
func TestCoyoteTargetLKI(t *testing.T) {
	deck := []cardset.Entry{{Name: "Driftgloom Coyote", Count: 2}, {Name: "Shoreline Looter", Count: 1}, {Name: "Bellowing Crier", Count: 1}, {Name: "Plains", Count: 8}}
	sc, id := newBLBScen(t, deck)
	lo := sc.perm(id["Shoreline Looter"], 1)
	sc.s.perms[sc.slot(lo)].setCtrN(sc.s, int(kop.CtrP1P1), 2)
	if n := coyoteResolve(t, sc, id["Driftgloom Coyote"], lo); n != 0 {
		t.Errorf("1/1 with two +1/+1 counters: Coyote got %d counters, want 0", n)
	}
	sc, id = newBLBScen(t, deck)
	cr := sc.perm(id["Bellowing Crier"], 1)
	if n := coyoteResolve(t, sc, id["Driftgloom Coyote"], cr); n != 1 {
		t.Errorf("plain 2/1: Coyote got %d counters, want 1", n)
	}
}
