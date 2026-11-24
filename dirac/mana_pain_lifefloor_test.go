package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPainDamageRiderHonoursTheLifeFloor: a painland's rider is damage, so a
// ReplLifeReduced floor (Fortune Thief, Ali from Cairo, Worship) clamps its
// loss like any damage's and a payer at the floor survives its own tap (gorge
// foldDamage folds the floor into the mana ability's damage). The damage
// notes still record the full 1 (the floor rewrites the life loss, not the
// damage dealt). Tapping for a paid-life colour is a life cost (CR 119.4),
// pinned by TestReplModePortsLifeFloor's loseLife arm.
func TestPainDamageRiderHonoursTheLifeFloor(t *testing.T) {
	tb := newSPTable()
	thief := tb.host("Test Thief", kop.Repl{Kind: kop.ReplLifeReduced, A: 1})
	tb.tbl.Cards = append(tb.tbl.Cards, kop.CardDef{Name: "Test Pain Land", Types: kop.TLand})
	land := CardID(len(tb.tbl.Cards) - 1)
	sc := tb.scen(t)
	src := sc.perm(land, 0)
	sc.perm(thief, 0)
	if sc.k.gen == nil || sc.k.gen.anyRepl2&rb2LifeFloor == 0 {
		t.Fatal("precondition: the life-floor table did not set the kernel's gate word")
	}
	sc.s.pl[0].life = 1
	if got := sc.s.lifeFloorTake(sc.k, 0, 1); got != 0 {
		t.Fatalf("precondition: lifeFloorTake(1 life, 1) = %d, want 0 (the floor clamps the loss)", got)
	}
	sc.s.painDamage(sc.k, sc.slot(src), 0, 1)
	if got := sc.s.pl[0].life; got != 1 {
		t.Fatalf("life after the rider at the floor = %d, want 1 (damage floored)", got)
	}
}
