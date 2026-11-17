package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestAttackerOfABattleThatReturnsDealsItNoDamage: a battle that leaves and
// comes back during the same combat is a new object (CR 400.7), so the
// attacker is no longer attacking it and deals no combat damage (CR 506.4c).
// dirac's damage routing used to match the persistent Obj and the new
// permanent's current type, which sent the damage to the returned battle.
func TestAttackerOfABattleThatReturnsDealsItNoDamage(t *testing.T) {
	tbl := *v.Table
	tbl.Name = "battle-return-test"
	tbl.Exprs = make([]kop.Expr, 1) // a generated-table prep (prep.gen)
	battleCard := CardID(len(v.Table.Cards))
	tbl.Cards = append(slices.Clone(v.Table.Cards), kop.CardDef{Name: "Test Battle", Types: kop.TBattle, Defense: 3})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	s := sc.s

	attacker := sc.perm(v.SavannahLions, 0)
	battle := sc.perm(battleCard, 0)
	battleSlot := sc.slot(battle)
	if s.typesAt(sc.k, battleSlot)&kop.TBattle == 0 {
		t.Fatal("precondition: the attacked permanent is not a battle")
	}

	sc.passUntil(t, kop.StepAttackers)
	sc.do(t, AkAttack, TCode(attacker), TCode(battle))
	sc.do(t, AkDone, scAny, scAny)

	atkSlot := sc.slot(attacker)
	if q := s.attackedPerm(sc.k, atkSlot); q != battleSlot {
		t.Fatalf("precondition: attackedPerm = %d, want the declared battle slot %d", q, battleSlot)
	}

	oldVer := s.objs[battle].ver
	s.leave(sc.k, battleSlot, kop.ZoneExile, false)
	returned := s.enter(sc.k, battle, 0, false, kop.ZoneExile)
	if s.objs[battle].ver == oldVer {
		t.Fatal("precondition: the returning battle is not a new object")
	}
	if s.objs[battle].zone != kop.ZoneBattlefield || s.typesAt(sc.k, returned)&kop.TBattle == 0 {
		t.Fatal("precondition: the battle did not return as a battle")
	}

	var r dmgRec
	s.toDefender(sc.k, &r, atkSlot, 2)
	if got := r.toPerm[returned]; got != 0 {
		t.Fatalf("the returned battle was dealt %d combat damage, want 0 (CR 506.4c)", got)
	}
	if got := r.dealt[atkSlot]; got != 0 {
		t.Fatalf("attacker damage record = %d, want 0: it is not attacking any permanent", got)
	}
}
