package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// bandFormTwo declares a1 and a2 attacking and accepts a2 into a1's band
// (a1 has the granted banding), leaving the game at the next decision.
func bandFormTwo(t *testing.T, sc *scen, a1, a2 Obj) {
	t.Helper()
	sc.passUntil(t, kop.StepAttackers)
	sc.do(t, AkAttack, TCode(a1), scAny)
	sc.do(t, AkAttack, TCode(a2), scAny)
	sc.do(t, AkDone, scAny, scAny)
	d := sc.expect(t, DkChoose, ChBand)
	sc.s.Apply(bandOpt(t, d, 1))
	if got := sc.s.bandOf(sc.slot(a2)); got != sc.slot(a1) {
		t.Fatalf("a2's band names slot %d, want the leader %d", got, sc.slot(a1))
	}
}

// TestBandBlocksAsGroup: CR 702.22h, a blocker declared against one member of
// a band blocks every member (dirac/combat_band.go bandExpandBlocks), and
// CR 702.22k hands the band blocker's damage division to the attacking
// player because a blocked attacker has banding.
func TestBandBlocksAsGroup(t *testing.T) {
	sc := bandDivScen(t)
	a1, a2, b := sc.perm(xbBear, 0), sc.perm(xbBear, 0), sc.perm(xbBear, 1)
	sc.grantBanding(a1)
	bandFormTwo(t, sc, a1, a2)
	sc.passUntil(t, kop.StepBlockers)
	sc.do(t, AkBlock, TCode(b), TCode(a1))
	sc.do(t, AkDone, scAny, scAny)
	if sc.s.g.stalled != 0 {
		t.Fatalf("the block declaration stalled %d (%s)", sc.s.g.stalled, StallName(sc.s.g.stalled))
	}
	s, bs, s1, s2 := sc.s, sc.slot(b), sc.slot(a1), sc.slot(a2)
	if !s.blocks(bs, s1) || !s.blocks(bs, s2) {
		t.Fatalf("the blocker does not block the whole band: a1 %v, a2 %v", s.blocks(bs, s1), s.blocks(bs, s2))
	}
	for _, sl := range []Slot{s1, s2} {
		if s.perms[sl].nblock != 1 || s.perms[sl].flags&pfBlocked == 0 {
			t.Fatalf("attacker %d: nblock %d, blocked %v; want one blocker and blocked", sl, s.perms[sl].nblock, s.perms[sl].flags&pfBlocked != 0)
		}
	}
	if r := s.xblkAt(bs, s2); r == nil {
		t.Fatal("the second band member has no further-block record")
	}
	d := xbToDivide(t, sc)
	if d.Player != 0 {
		t.Fatalf("the band blocker's division is asked of player %d, want the attacking player 0 (CR 702.22k)", d.Player)
	}
}

// TestBandRemovedFromCombat: CR 702.22f, a creature removed from combat
// leaves its band, and the rest of the band keeps the leader as its identity.
func TestBandRemovedFromCombat(t *testing.T) {
	sc := bandDivScen(t)
	a1, a2 := sc.perm(xbBear, 0), sc.perm(xbBear, 0)
	sc.grantBanding(a1)
	bandFormTwo(t, sc, a1, a2)
	s := sc.s
	leader := sc.slot(a1)
	s.leaveCombat(sc.slot(a2))
	if got := s.bandOf(sc.slot(a2)); got != NoSlot {
		t.Fatalf("the removed member still names band %d, want none", got)
	}
	if got := s.bandOf(leader); got != leader {
		t.Fatalf("the leader's band moved to %d, want itself %d", got, leader)
	}
	s.bandClear()
	if got := s.bandOf(leader); got != NoSlot {
		t.Fatalf("bandClear left band %d, want none", got)
	}
}
