package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// flipScen builds a table with a FlipCoinMod permanent and a creature whose
// activated ability is one coin flip (OpRollDie C = RollDieCoin).
func flipScen(t *testing.T) (*scen, Obj, Obj) {
	t.Helper()
	tbl, _, ids := newModeTable(
		kop.CardDef{Name: "Test Edgar", Types: kop.TCreature, Power: 1, Toughness: 1,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatFlipCoinMod}}}},
		kop.CardDef{Name: "Test Flipper", Types: kop.TCreature, Power: 1, Toughness: 1,
			Abilities: []kop.Ability{{Kind: kop.AbActivated, Prog: []kop.Instr{{Op: kop.OpRollDie,
				A: kop.Const(2), B: kop.Const(0), C: kop.RollDieCoin}}}}},
	)
	sc := newScenTable(t, tbl, Config{NoElide: true})
	return sc, sc.perm(ids[0], 0), sc.perm(ids[1], 0)
}

// TestTurnFlipForced: the first coin flip by the controller of a FlipCoinMod
// permanent is heads (1) with no rng draw; a second flip in the same turn
// draws the rng.
func TestTurnFlipForced(t *testing.T) {
	sc, edgar, flipper := flipScen(t)
	if sc.k.gen == nil || !sc.k.gen.anyTurnF {
		t.Fatal("precondition: the table does not arm the turn facts (gen.anyTurnF)")
	}
	if !sc.onBF(edgar) || !sc.onBF(flipper) {
		t.Fatal("precondition: both permanents are not on the battlefield")
	}
	draws := sc.s.rng.Draws
	if !sc.activateAb(flipper, 0) {
		t.Fatal("the flip ability was not offered")
	}
	sc.resolveAll(t)
	if sc.s.g.roll != 1 {
		t.Fatalf("first flip result %d, want forced heads (1)", sc.s.g.roll)
	}
	if sc.s.rng.Draws != draws {
		t.Fatalf("the forced flip drew the rng: Draws %d -> %d", draws, sc.s.rng.Draws)
	}
	if !sc.s.turnFlipBit(0) {
		t.Fatal("the first flip did not set the controller's this-turn bit")
	}
	if !sc.activateAb(flipper, 0) {
		t.Fatal("the second flip ability was not offered")
	}
	sc.resolveAll(t)
	if sc.s.rng.Draws != draws+1 {
		t.Fatalf("the second flip did not draw the rng: Draws %d -> %d", draws, sc.s.rng.Draws)
	}
}

// exhaustScen builds a table with an Activations static and a creature whose
// activated ability is Exhaust (once per object).
func exhaustScen(t *testing.T) (*scen, Obj, Obj) {
	t.Helper()
	tbl, _, ids := newModeTable(
		kop.CardDef{Name: "Test Refueler", Types: kop.TCreature, Power: 1, Toughness: 1,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatActivationLimit}}}},
		kop.CardDef{Name: "Test Exhauster", Types: kop.TCreature, Power: 1, Toughness: 1,
			Abilities: []kop.Ability{{Kind: kop.AbActivated, Exhaust: true, Once: 1,
				Cost: kop.Cost{Flags: kop.CostOncePerGame},
				Prog: []kop.Instr{{Op: kop.OpGainLife, A: kop.Const(1)}}}}},
	)
	sc := newScenTable(t, tbl, Config{NoElide: true})
	return sc, sc.perm(ids[0], 0), sc.perm(ids[1], 0)
}

// TestTurnExhaustRefuel: on the turn after an Exhaust ability was already
// activated, an Activations static lets its controller activate it once more
// as though it had not been activated; that activation still marks the
// ability's own once record, and a second activation in the same turn is
// refused.
func TestTurnExhaustRefuel(t *testing.T) {
	sc, refueler, exhauster := exhaustScen(t)
	if sc.k.gen == nil || !sc.k.gen.anyTurnF {
		t.Fatal("precondition: the table does not arm the turn facts (gen.anyTurnF)")
	}
	if !sc.onBF(refueler) || !sc.onBF(exhauster) {
		t.Fatal("precondition: both permanents are not on the battlefield")
	}
	if !sc.activateAb(exhauster, 0) {
		t.Fatal("the first exhaust activation was not offered")
	}
	sc.resolveAll(t)
	sl := sc.slot(exhauster)
	if !sc.s.onceUsed(sc.k, sl, &sc.k.t.Cards[sc.s.objs[exhauster].card].Abilities[0]) {
		t.Fatal("the first exhaust activation did not mark the ability's once record")
	}
	if !sc.s.turnExhaustBit(0) {
		t.Fatal("the first exhaust activation did not set the controller's this-turn bit")
	}

	sc.nextTurn() // the ability's once record spans the game; this turn's bit does not
	if !sc.activateAb(exhauster, 0) {
		t.Fatal("the Activations static did not waive the used once record on the next turn")
	}
	sc.resolveAll(t)
	if !sc.s.onceUsed(sc.k, sl, &sc.k.t.Cards[sc.s.objs[exhauster].card].Abilities[0]) {
		t.Fatal("the waived activation cleared the ability's own once record")
	}
	if !sc.s.turnExhaustBit(0) {
		t.Fatal("the waived activation did not set the controller's this-turn bit")
	}
	if sc.activateOffered(exhauster, 0) {
		t.Fatal("a second exhaust activation in the same turn was offered")
	}
}
