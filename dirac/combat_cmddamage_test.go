package dirac

import (
	"testing"

	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestCommanderDamageLoses: a commander that connects for 7 three times
// takes the defender out with life far above 0; a real combat adds its
// power to the tally.
func TestCommanderDamageLoses(t *testing.T) {
	sc := newScen(t, Config{NoElide: true, Commander: true})
	dm := sc.perm(v.ColossalDreadmaw, 0)
	if sc.s.cfg&cfgCommander == 0 {
		t.Fatal("scenario is not a commander game")
	}
	sc.s.pl[0].cmd = dm
	sc.combat(t, []Obj{dm}, nil)
	if got := sc.s.pl[1].cmdDmg[0]; got != 6 {
		t.Fatalf("after a real 6-power hit the tally is %d, want 6", got)
	}
	sl := sc.slot(dm)
	sc.s.perms[sl].atk = PlayerTarget0 + 1
	sc.s.pl[1].cmdDmg[0] = 0
	for i := 1; i <= 3; i++ {
		var r dmgRec
		r.dealtPl[sl] = 7
		sc.s.noteCmdDamage(&r)
		sc.s.sba(sc.k)
		if i < 3 && sc.s.pl[1].lost {
			t.Fatalf("lost after %d hits (%d damage)", i, sc.s.pl[1].cmdDmg[0])
		}
	}
	if sc.s.pl[1].cmdDmg[0] != 21 || !sc.s.pl[1].lost || sc.s.pl[1].life <= 0 {
		t.Fatalf("tally %d lost %v life %d; want 21, lost, life above 0", sc.s.pl[1].cmdDmg[0], sc.s.pl[1].lost, sc.s.pl[1].life)
	}
}

// TestCommanderDamagePrevented: prevented combat damage, a non-commander's
// damage and damage outside a commander game leave the tally alone.
func TestCommanderDamagePrevented(t *testing.T) {
	sc := newScen(t, Config{NoElide: true, Commander: true})
	dm := sc.perm(v.ColossalDreadmaw, 0)
	other := sc.perm(v.HillGiant, 0)
	sc.s.pl[0].cmd = dm
	sl, so := sc.slot(dm), sc.slot(other)
	sc.s.perms[sl].atk = PlayerTarget0 + 1
	sc.s.perms[so].atk = PlayerTarget0 + 1
	var r dmgRec
	r.dealtPl[sl], r.dealtPl[so] = 5, 9
	r.dealtPl[sl] -= 5 // prevention takes it back out of dealtPl
	sc.s.noteCmdDamage(&r)
	if sc.s.pl[1].cmdDmg != [2]uint8{} {
		t.Fatalf("tally %v, want none", sc.s.pl[1].cmdDmg)
	}
	sc.s.loseLife(sc.k, 1, 30) // non-combat damage is life loss only
	sc.s.cmdDamageSBA(sc.k)
	if sc.s.pl[1].cmdDmg != [2]uint8{} {
		t.Fatalf("tally %v after non-combat damage", sc.s.pl[1].cmdDmg)
	}
	r.dealtPl[sl] = 4
	sc.s.cfg &^= cfgCommander
	sc.s.noteCmdDamage(&r)
	if sc.s.pl[1].cmdDmg != [2]uint8{} {
		t.Fatalf("tally %v outside commander", sc.s.pl[1].cmdDmg)
	}
}
