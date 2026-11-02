package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestLeftzoneRing pins the two departure readers over a hand-built ring.
// The lockstep (dirac/oracle/leftzone_test.go) cannot reach either
// mutation: no FDN card returns another player's departed permanent to the
// battlefield, and a card leaving a graveyard twice in one turn needs two
// recursions on the same card.
func TestLeftzoneRing(t *testing.T) {
	tbl := *v.Table
	tbl.Exprs = make([]kop.Expr, 1) // force the generated path
	sc := newScenTable(t, &tbl, Config{})
	sc.s.enteredRing()
	sc.s.turn = 1

	// ExLeftZone: one object that left the graveyard twice counts once, a
	// second object counts too, an entry from another zone counts for none.
	a := sc.obj(v.GrizzlyBears, 0)
	b := sc.obj(v.HillGiant, 0)
	sc.s.noteEnter(a, kop.ZoneHand, kop.ZoneGraveyard, false)
	sc.s.noteEnter(a, kop.ZoneHand, kop.ZoneGraveyard, false)
	sc.s.noteEnter(b, kop.ZoneExile, kop.ZoneGraveyard, false)
	sc.s.noteEnter(b, kop.ZoneGraveyard, kop.ZoneBattlefield, true)
	x := &kop.Expr{Kind: kop.ExLeftZone, N: int16(kop.ZoneGraveyard)}
	if got := sc.s.leftZone(sc.k, x, evalCtx{}); got != 2 {
		t.Fatalf("distinct objects that left the graveyard: got %d, want 2", got)
	}
	DebugLeftZoneNoDedupe = true
	got := sc.s.leftZone(sc.k, x, evalCtx{})
	DebugLeftZoneNoDedupe = false
	if got != 3 {
		t.Fatalf("mutation (one per entry): got %d, want 3", got)
	}

	// ExRevolt reads the controller the permanent left under, not its owner
	// (CR 603.10a): a stolen Bears that dies counts for the thief.
	sc.s.g.enteredN = 0
	o := sc.perm(v.GrizzlyBears, 1) // owner 1
	sc.s.perms[sc.slot(o)].ctrl = 0 // under player 0
	sc.s.destroy(sc.k, sc.slot(o))
	if !sc.s.revoltHolds(0) || sc.s.revoltHolds(1) {
		t.Fatalf("stolen permanent died: revolt(0)=%v revolt(1)=%v, want true false", sc.s.revoltHolds(0), sc.s.revoltHolds(1))
	}
	DebugRevoltOwner = true
	r0, r1 := sc.s.revoltHolds(0), sc.s.revoltHolds(1)
	DebugRevoltOwner = false
	if r0 || !r1 {
		t.Fatalf("mutation (owner): revolt(0)=%v revolt(1)=%v, want false true", r0, r1)
	}
}
