package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// flipBatchScen is flipScen with a three-coin batch: Repeat 3 over a batch
// flip and a 1-life gain, then the Mark that closes the batch (kgen's
// lowering of FlipCoin Amount$ 3).
func flipBatchScen(t *testing.T) (*scen, Obj, Obj) {
	t.Helper()
	tbl, _, ids := newModeTable(
		kop.CardDef{Name: "Test Edgar", Types: kop.TCreature, Power: 1, Toughness: 1,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatFlipCoinMod}}}},
		kop.CardDef{Name: "Test Batch Flipper", Types: kop.TCreature, Power: 1, Toughness: 1,
			Abilities: []kop.Ability{{Kind: kop.AbActivated, Prog: []kop.Instr{
				{Op: kop.OpRepeat, A: kop.Const(3), B: kop.Const(2)},
				{Op: kop.OpRollDie, A: kop.Const(2), B: kop.Const(0), C: kop.RollDieCoin | kop.RollDieCoinBatch},
				{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)},
				{Op: kop.OpRollDie, A: kop.Const(2), B: kop.Const(0), C: kop.RollDieCoin | kop.RollDieCoinMark},
			}}}},
	)
	sc := newScenTable(t, tbl, Config{NoElide: true})
	return sc, sc.perm(ids[0], 0), sc.perm(ids[1], 0)
}

// TestTurnFlipBatchForced: gorge effFlipCoin reads FlipCoinForced once per
// Amount$ batch, so with a FlipCoinMod permanent every coin of the
// controller's first batch this turn is heads with no rng draw; the batch
// flips leave the turn bit alone and the closing Mark sets it, so the next
// flip draws.
func TestTurnFlipBatchForced(t *testing.T) {
	t.Run("instructions", func(t *testing.T) {
		sc, edgar, _ := flipBatchScen(t)
		if sc.k.gen == nil || !sc.k.gen.anyTurnF || !sc.onBF(edgar) || sc.s.turnFlipBit(0) {
			t.Fatal("precondition: turn facts armed, Edgar on the battlefield, no flip this turn")
		}
		sc.s.res.e.ctrl = 0
		batch := kop.Instr{Op: kop.OpRollDie, A: kop.Const(2), C: kop.RollDieCoin | kop.RollDieCoinBatch}
		mark := kop.Instr{Op: kop.OpRollDie, A: kop.Const(2), C: kop.RollDieCoin | kop.RollDieCoinMark}
		draws := sc.s.rng.Draws
		for i := 0; i < 3; i++ {
			sc.s.exec(sc.k, &batch)
			if sc.s.g.roll != 1 || sc.s.rng.Draws != draws {
				t.Fatalf("batch flip %d: roll %d, draws %d -> %d; want forced heads, no draw", i, sc.s.g.roll, draws, sc.s.rng.Draws)
			}
			if sc.s.turnFlipBit(0) {
				t.Fatalf("batch flip %d set the turn bit before the Mark", i)
			}
		}
		sc.s.exec(sc.k, &mark)
		if sc.s.g.roll != 1 || sc.s.rng.Draws != draws || !sc.s.turnFlipBit(0) {
			t.Fatalf("Mark: roll %d, draws %d -> %d, bit %v; want roll kept, no draw, bit set", sc.s.g.roll, draws, sc.s.rng.Draws, sc.s.turnFlipBit(0))
		}
		sc.s.exec(sc.k, &batch)
		if sc.s.rng.Draws != draws+1 {
			t.Fatalf("a batch after the Mark did not draw: draws %d -> %d", draws, sc.s.rng.Draws)
		}
	})
	t.Run("program", func(t *testing.T) {
		sc, edgar, flipper := flipBatchScen(t)
		if !sc.onBF(edgar) || !sc.onBF(flipper) || sc.s.turnFlipBit(0) {
			t.Fatal("precondition: both permanents on the battlefield, no flip this turn")
		}
		draws, life := sc.s.rng.Draws, sc.s.pl[0].life
		if !sc.activateAb(flipper, 0) {
			t.Fatal("the batch ability was not offered")
		}
		sc.resolveAll(t)
		if sc.s.pl[0].life != life+3 {
			t.Fatalf("life %d -> %d, want three passes (+3)", life, sc.s.pl[0].life)
		}
		if sc.s.rng.Draws != draws || sc.s.g.roll != 1 {
			t.Fatalf("draws %d -> %d, roll %d; want three forced heads and no draw", draws, sc.s.rng.Draws, sc.s.g.roll)
		}
		if !sc.s.turnFlipBit(0) {
			t.Fatal("the Mark did not set the controller's turn bit")
		}
	})
}
