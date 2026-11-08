package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// A double-striking deathtouch trampler that is blocked by an indestructible
// creature marks it Deathtouched in the first-strike damage step. That mark
// alone is lethal (CR 704.5h), so the second-strike pass owes it no damage and
// may trample the whole power over (CR 702.19b): gorge's lethalDamageNeed
// returns 0 for a marked blocker.
func TestDeathtouchMarkLetsSecondStrikeTrampleOver(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	dm := sc.perm(v.ColossalDreadmaw, 0) // 6/6 trample
	sc.s.perms[sc.slot(dm)].eotKw |= kop.KwDoubleStrike | kop.KwDeathtouch
	giant := sc.perm(v.HillGiant, 1) // 3/3
	sc.s.perms[sc.slot(giant)].eotKw |= kop.KwIndestructible
	sc.combat(t, []Obj{dm}, [][2]Obj{{giant, dm}})
	g := sc.s.perms[sc.slot(giant)]
	if g.flags&pfDeathtouched == 0 {
		t.Fatal("precondition: the blocker is not Deathtouched")
	}
	if sc.s.pl[1].life != 9 {
		t.Fatalf("defender life %d, want 9 (1 lethal first strike, then 5 + 6 trample)", sc.s.pl[1].life)
	}
	if g.damage != 1 {
		t.Fatalf("blocker damage %d, want 1: the second-strike pass owes nothing to a marked blocker", g.damage)
	}
}
