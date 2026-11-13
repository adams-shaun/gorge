package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// infectScen: kwEvasionScen with Valley Floodcaller given printed infect (and
// toxic tox) in this matchup's table copy.
func infectScen(t *testing.T, infect bool, tox uint8) (*scen, Obj, Obj) {
	t.Helper()
	sc, a, b := kwEvasionScen(t)
	c := sc.s.objs[a].card
	sc.k.t.Cards[c].Infect, sc.k.t.Cards[c].Toxic = infect, tox
	sc.k.gen.anyInfect = true
	return sc, a, b
}

// TestInfectDamageTo: an infect source's damage (gorge emitPlayerDamage /
// emitObjectDamage, convertInfectDamage) is poison counters to a player and
// -1/-1 counters to a creature, never life loss or marked damage.
func TestInfectDamageTo(t *testing.T) {
	sc, a, b := infectScen(t, true, 0)
	sc.s.res.e.ctrl, sc.s.res.e.obj, sc.s.res.e.card = 0, a, sc.s.objs[a].card
	life := sc.s.pl[1].life
	sc.s.damageTo(sc.k, TCode(PlayerTarget1), 3)
	if sc.s.Poison(1) != 3 || sc.s.pl[1].life != life {
		t.Fatalf("player: poison %d life %d, want 3 and %d", sc.s.Poison(1), sc.s.pl[1].life, life)
	}
	bs := sc.slot(b)
	sc.s.damageTo(sc.k, TCode(b), 1)
	if p := &sc.s.perms[bs]; p.damage != 0 || p.ctrLow(int(kop.CtrM1M1)) != 1 {
		t.Fatalf("creature: damage %d, -1/-1 counters %d, want 0 and 1", p.damage, p.ctrLow(int(kop.CtrM1M1)))
	}
	sc.s.infectBetween(sc.k, sc.slot(a), bs, 1, 0)
	if p := &sc.s.perms[bs]; p.damage != 0 || p.ctrLow(int(kop.CtrM1M1)) != 2 {
		t.Fatalf("fight: damage %d, -1/-1 counters %d, want 0 and 2", p.damage, p.ctrLow(int(kop.CtrM1M1)))
	}
}

// TestPoisonSBA: ten poison counters lose the game (CR 704.5c).
func TestPoisonSBA(t *testing.T) {
	sc, _, _ := infectScen(t, true, 0)
	sc.s.addPoison(1, 9)
	if sc.s.sba(sc.k) {
		t.Fatal("nine poison counters ended the game")
	}
	sc.s.addPoison(1, 1)
	if !sc.s.sba(sc.k) || !sc.s.pl[1].lost {
		t.Fatal("ten poison counters did not lose the game")
	}
}

// combatHit has a attack player 1 unblocked and deals the regular combat
// damage step.
func combatHit(sc *scen, a Obj) {
	p := &sc.s.perms[sc.slot(a)]
	p.flags |= pfAttacking
	p.atk = TCode(PlayerTarget1)
	sc.s.active, sc.s.nAttackers = 0, 1
	sc.s.combatDamage(sc.k, false)
}

// TestInfectToxicCombat: unblocked combat damage from an infect attacker is
// poison only; from a toxic attacker it is life loss plus toxic N poison
// (gorge runCombatAssignments, ToxicValue).
func TestInfectToxicCombat(t *testing.T) {
	sc, a, _ := infectScen(t, true, 0)
	pw := sc.s.power(sc.k, sc.slot(a))
	if pw <= 0 {
		t.Fatal("the scenario attacker has no power")
	}
	life := sc.s.pl[1].life
	combatHit(sc, a)
	if sc.s.Poison(1) != pw || sc.s.pl[1].life != life {
		t.Fatalf("infect: poison %d life %d, want %d and %d", sc.s.Poison(1), sc.s.pl[1].life, pw, life)
	}
	sc, a, _ = infectScen(t, false, 2)
	combatHit(sc, a)
	if sc.s.Poison(1) != 2 || int(sc.s.pl[1].life) != int(life)-pw {
		t.Fatalf("toxic 2: poison %d life %d, want 2 and %d", sc.s.Poison(1), sc.s.pl[1].life, int(life)-pw)
	}
}

// TestInfectStall: infect with a counter-added trigger in the table stalls
// (prepInfect); proliferate no longer does (the kernel proliferates
// players' poison as gorge does).
func TestInfectStall(t *testing.T) {
	tbl := &kop.Table{Cards: []kop.CardDef{{Infect: true}, {Abilities: []kop.Ability{{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigCounterAddedOnce}}}}}}
	var g genPrep
	prepInfect(tbl, &g)
	if !g.anyInfect || !g.infectStall {
		t.Fatalf("infect + counter-added trigger: any %v stall %v", g.anyInfect, g.infectStall)
	}
	tbl.Cards[1] = kop.CardDef{Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{{Op: kop.OpProliferate}}}}}
	g = genPrep{}
	prepInfect(tbl, &g)
	if !g.anyInfect || g.infectStall {
		t.Fatalf("infect + proliferate: any %v stall %v", g.anyInfect, g.infectStall)
	}
}

// TestPoisonOp: OpPoison gives the opponent poison counters and turns on
// the ten-poison state-based action (prepPoison sets anyInfect).
func TestPoisonOp(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	po := kop.Instr{Op: kop.OpPoison, Obj: kop.RefOpponent, A: kop.Const(4)}
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Viper", Types: kop.TSorcery, Abilities: []kop.Ability{
		{Kind: kop.AbSpell, Prog: []kop.Instr{po}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if !sc.k.gen.anyInfect || sc.k.gen.infectStall {
		t.Fatalf("anyInfect %v infectStall %v, want true false", sc.k.gen.anyInfect, sc.k.gen.infectStall)
	}
	s := sc.s
	s.res.e.ctrl = 0
	for i := 0; i < 3; i++ {
		s.exec(sc.k, &po)
	}
	if s.Poison(1) != 12 || s.Poison(0) != 0 {
		t.Fatalf("poison %d / %d, want 0 / 12", s.Poison(0), s.Poison(1))
	}
	s.poisonSBA(sc.k)
	if !s.pl[1].lost {
		t.Fatal("twelve poison counters: the opponent did not lose")
	}
}
