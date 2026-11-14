package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestExtraBlockSecondBlockOrder: a blocker of two attackers sits in its
// second attacker's block order where that second block was declared, not
// where its first one was (gorge appends each declared pair to the
// attacker's BlockedBy, and its division lists the blockers in that order).
// The 4/4's blockers are the Bear c2 (declared third) then the Twins
// (declared fourth), so the 4/4's division mode 3 gives c2 three and the
// Twins one.
func TestExtraBlockSecondBlockOrder(t *testing.T) {
	sc := xbScen(t, Config{NoElide: true})
	a3, a4 := sc.perm(xbA3, 0), sc.perm(xbA4, 0)
	tw, c1, c2 := sc.perm(xbTwins, 1), sc.perm(xbBear, 1), sc.perm(xbBear, 1)
	xbAttack(t, sc, a3, a4)
	for _, p := range [][2]Obj{{tw, a3}, {c1, a3}, {c2, a4}, {tw, a4}} {
		sc.do(t, AkBlock, TCode(p[0]), TCode(p[1]))
	}
	sc.do(t, AkDone, scAny, scAny)
	if s := sc.s; s.blockKey(sc.slot(c2), sc.slot(a4)) >= s.blockKey(sc.slot(tw), sc.slot(a4)) {
		t.Fatalf("precondition: c2's block of the 4/4 (key %d) is not declared before the Twins' (key %d)",
			s.blockKey(sc.slot(c2), sc.slot(a4)), s.blockKey(sc.slot(tw), sc.slot(a4)))
	}
	// a3: mode 1 gives the Twins (first) one and c1 two; a4: mode 3 gives
	// its first blocker three and its second one; the Twins: mode 0 gives
	// its first attacker nothing
	modes := map[Obj]uint8{a3: 1, a4: 3, tw: 0}
	asked := map[Obj]bool{}
	for i := 0; i < 80 && sc.s.step != kop.StepMain2; i++ {
		d := sc.s.Step()
		switch d.Kind {
		case DkPriority:
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		case DkChoose:
			if d.Sub != ChDivide {
				t.Fatalf("choose sub %d", d.Sub)
			}
			src := Obj(d.Options[0].Source())
			m, ok := modes[src]
			if !ok || asked[src] {
				t.Fatalf("unexpected division of %d", src)
			}
			asked[src] = true
			sc.s.Apply(MakeAction(AkChoose, TCode(src), 0, m))
		default:
			t.Fatalf("decision kind %d", d.Kind)
		}
	}
	if len(asked) != 3 {
		t.Fatalf("divisions asked: %v, want the two attackers and the Twins", asked)
	}
	if got := sc.s.perms[sc.slot(tw)].damage; got != 2 {
		t.Fatalf("the Twins has %d damage, want 1 (from the 3/3) + 1 (from the 4/4, its second blocker) = 2", got)
	}
	if sc.onBF(c1) || sc.onBF(c2) {
		t.Fatalf("c1 alive=%v c2 alive=%v; want both dead (2 and 3 damage)", sc.onBF(c1), sc.onBF(c2))
	}
}
