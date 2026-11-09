package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// charmRepeatTable is vanilla's Test Charm made repeatable: mode 0 deals 2
// damage to its creature target, mode 1 makes its player target lose 2 life.
// Each mode's block is lowered once per instance it can be chosen
// (kgen/lower_charmrepeat.go); budget and costs price the modes in pawprints.
func charmRepeatTable(pick, few, budget uint8, costs [2]uint8) *kop.Table {
	t := *v.Table
	t.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	a := t.Cards[v.TestCharm].Abilities[0]
	a.NModes, a.ModesPick, a.ModesFew = 2, pick, few
	a.RepBudget, a.RepCost = budget, [4]uint8{costs[0], costs[1]}
	for m := 0; m < 2; m++ {
		a.RepMax[m] = pick
		if budget != 0 && costs[m] != 0 {
			a.RepMax[m] = min(pick, budget/costs[m])
		}
	}
	shift, width, _ := kop.RepeatLayout(a.RepMax)
	a.Prog = nil
	for m := 0; m < 2; m++ {
		for r := 0; r < int(a.RepMax[m]); r++ {
			c := kop.RepeatCharmC(int(shift[m]), int(width[m]), r, m)
			body := kop.Instr{Op: kop.OpDealDamage, Obj: kop.RefTarget0, A: kop.Const(2)}
			if m == 1 {
				body = kop.Instr{Op: kop.OpLoseLife, Obj: kop.RefTarget1, A: kop.Const(2)}
			}
			a.Prog = append(a.Prog, kop.Instr{Op: kop.OpCharm, A: kop.Const(1), C: c}, body)
		}
	}
	t.Cards[v.TestCharm].Abilities = []kop.Ability{a}
	return &t
}

// charmRepeatPose is the next modes decision: the modes offered and whether
// Done is.
func charmRepeatPose(t *testing.T, sc *scen) (modes uint8, done bool, n int) {
	t.Helper()
	d := sc.s.Step()
	if d.Kind != DkModes {
		t.Fatalf("decision kind %d, want the modes ask: %+v", d.Kind, d.Options[:d.N])
	}
	modes, done = modeOpts(&d)
	return modes, done, int(d.N)
}

// A mode chosen twice resolves twice, each instance on its own target; Done
// is offered at the minimum; a pawprint budget hides a mode its remainder
// cannot pay.
func TestCharmRepeatModes(t *testing.T) {
	start := func(t *testing.T, tbl *kop.Table) (*scen, Obj, Obj) {
		sc := newScenTable(t, tbl, Config{NoElide: true})
		sc.lands(0, v.Mountain, 1)
		a := sc.perm(v.GrizzlyBears, 1)
		b := sc.perm(v.GrizzlyBears, 1)
		sc.do(t, AkCast, uint8(sc.hand(v.TestCharm, 0)), 0xff)
		return sc, a, b
	}
	t.Run("twice resolves twice with two targets", func(t *testing.T) {
		sc, a, b := start(t, charmRepeatTable(3, 2, 0, [2]uint8{}))
		if m, done, _ := charmRepeatPose(t, sc); m != 3 || done {
			t.Fatalf("first pose: modes %b done %v, want both modes and no Done (minimum 1)", m, done)
		}
		charmnumPickMode(t, sc, 0)
		if m, done, _ := charmRepeatPose(t, sc); m != 3 || !done {
			t.Fatalf("after one pick: modes %b done %v, want both modes again and Done at the minimum", m, done)
		}
		charmnumPickMode(t, sc, 0)
		sc.do(t, AkDone, 0xff, 0xff)
		if got := sc.s.BuildModes(); sc.s.buildAbility(sc.k).ModeCount(got, 0) != 2 {
			t.Fatalf("chosen modes %b: mode 0 not counted twice", got)
		}
		sc.do(t, AkTarget, 0xff, uint8(a))
		sc.do(t, AkTarget, 0xff, uint8(b))
		sc.resolveAll(t)
		if sc.onBF(a) || sc.onBF(b) {
			t.Fatalf("bears on the battlefield: %v %v; each instance of the repeated mode must hit its own target", sc.onBF(a), sc.onBF(b))
		}
	})
	t.Run("answer order is not resolution order", func(t *testing.T) {
		sc, a, b := start(t, charmRepeatTable(3, 0, 0, [2]uint8{}))
		charmnumPickMode(t, sc, 1)
		charmnumPickMode(t, sc, 0)
		if _, done, _ := charmRepeatPose(t, sc); done {
			t.Fatal("Done offered below the minimum of three")
		}
		charmnumPickMode(t, sc, 0)
		// the extra instance of mode 0 owns the overflow slot after the template slots
		sc.do(t, AkTarget, 0xff, uint8(a))        // mode 0, instance 0 (slot 0)
		sc.do(t, AkTarget, 0xff, PlayerTarget0+1) // mode 1 (slot 1)
		sc.do(t, AkTarget, 0xff, uint8(b))        // mode 0, instance 1 (slot 2)
		sc.resolveAll(t)
		if sc.onBF(a) || sc.onBF(b) || sc.s.pl[1].life != 18 {
			t.Fatalf("a on %v, b on %v, life %d: want both bears dead and 18 life", sc.onBF(a), sc.onBF(b), sc.s.pl[1].life)
		}
	})
	t.Run("combined ask groups the instances in printed order", func(t *testing.T) {
		sc, _, b := start(t, charmRepeatTable(3, 0, 0, [2]uint8{}))
		charmnumPickMode(t, sc, 1)
		charmnumPickMode(t, sc, 0)
		charmnumPickMode(t, sc, 0)
		ab := sc.s.buildAbility(sc.k)
		order, n := charmAskOrder(ab, sc.s.bld().e.mode)
		if n != 3 || order[0] != 0 || order[1] != 2 || order[2] != 1 {
			t.Fatalf("ask order %v (%d): want mode 0's template, its extra instance, then mode 1: slots 0 2 1", order, n)
		}
		var d Decision
		sc.s.charmTargetsStep(sc.k, &d)
		// two groups of the two bears, then the players
		var occ [2]uint8
		for i := 0; i < int(d.N); i++ {
			if x := d.Options[i]; x.Kind() == AkTarget && x.Target() == uint8(b) {
				occ[x.Mode()]++
			}
		}
		if occ[0] != 1 || occ[1] != 1 {
			t.Fatalf("second bear listed %v times by occurrence, want once in each of the two mode 0 groups", occ)
		}
	})
	t.Run("a pawprint budget hides an unaffordable mode", func(t *testing.T) {
		// budget 3, mode 0 costs 1, mode 1 costs 2: mode 1 fits once
		sc, a, _ := start(t, charmRepeatTable(5, 5, 3, [2]uint8{1, 2}))
		if m, done, _ := charmRepeatPose(t, sc); m != 3 || !done {
			t.Fatalf("first pose: modes %b done %v, want both modes and Done (up to five)", m, done)
		}
		charmnumPickMode(t, sc, 1)
		if m, done, _ := charmRepeatPose(t, sc); m != 1 || !done {
			t.Fatalf("after mode 1 (2 of 3 pawprints): modes %b done %v, want only mode 0 and Done", m, done)
		}
		charmnumPickMode(t, sc, 0)
		if m, done, n := charmRepeatPose(t, sc); m != 0 || !done || n != 1 {
			t.Fatalf("budget spent: modes %b done %v options %d, want Done alone", m, done, n)
		}
		sc.do(t, AkDone, 0xff, 0xff)
		sc.do(t, AkTarget, 0xff, uint8(a))
		sc.do(t, AkTarget, 0xff, PlayerTarget0+1)
		sc.resolveAll(t)
		if sc.onBF(a) || sc.s.pl[1].life != 18 {
			t.Fatalf("a on %v, life %d: want the bear dead and 18 life", sc.onBF(a), sc.s.pl[1].life)
		}
	})
}
