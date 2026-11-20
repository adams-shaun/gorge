package dirac_test

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestStageWideObjectCodes stages a blocker whose attacker is object 259:
// beyond a byte, the width the staging types carried before the widening.
// Two 100-card decks put the tokens at objects 200..259; the blocker's
// Blocks mark is what a byte could not express.
func TestStageWideObjectCodes(t *testing.T) {
	t.Parallel()
	const deck = 100
	w := dirac.World{
		Tbl: vanilla.Table, NSeat: 2, Turn: 1, Step: kop.StepBlockers,
		Active: 0, Prio: 0, Life: [dirac.MaxSeats]int32{20, 20},
	}
	for p := uint8(0); p < 2; p++ {
		for i := 0; i < deck; i++ {
			c := vanilla.Plains
			if i%2 == 1 {
				c = vanilla.SavannahLions
			}
			o := dirac.WorldObj{Card: c, Owner: p, Ctrl: p, Zone: kop.ZoneLibrary, Pos: uint8(i - 7)}
			if i < 7 {
				o.Zone, o.Pos = kop.ZoneHand, uint8(i)
			}
			w.Objs = append(w.Objs, o)
		}
	}
	// A seat-1 Savannah Lions on the battlefield: the last card of its library.
	lions := len(w.Objs) - 1
	w.Objs[lions].Zone, w.Objs[lions].Pos = kop.ZoneBattlefield, 0
	// 60 tokens on seat 0's battlefield, at objects 200..259.
	for j := 0; j < 60; j++ {
		w.Objs = append(w.Objs, dirac.WorldObj{
			Card: vanilla.SoldierToken, Owner: 0, Ctrl: 0, Zone: kop.ZoneBattlefield,
			Pos: uint8(j), Token: true,
		})
	}
	attacker := len(w.Objs) - 1 // 259
	w.Objs[attacker].Attacking, w.Objs[attacker].AtkSeat, w.Objs[attacker].Blocked = true, 1, true
	w.Objs[lions].Blocking, w.Objs[lions].Blocks, w.Objs[lions].BlockOrd = true, dirac.Obj(attacker), 1

	s, err := dirac.NewFromWorld(&w)
	if err != nil {
		t.Fatalf("the world does not stage: %v", err)
	}
	var p dirac.Projection
	s.Project(&p)
	if got := p.Objs[lions].Blocks; got != dirac.Obj(attacker) {
		t.Fatalf("lions blocks %d, want %d", got, attacker)
	}
	if p.Objs[attacker].Flags&dirac.ProjAttacking == 0 {
		t.Fatalf("token %d does not project as attacking", attacker)
	}
}
