package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// stepInScen is Great Train Heist and a Mountain from the OTJ table.
func stepInScen(t *testing.T) (*scen, map[string]CardID) {
	t.Helper()
	return synthScen(t, "std-otj", "Great Train Heist", "Mountain")
}

// stepInCast casts Great Train Heist's first Spree mode ({2}{R}: untap your
// creatures, then, in your combat phase, an extra combat after this one).
func (sc *scen) stepInCast(t *testing.T, id map[string]CardID) {
	t.Helper()
	for i := 0; i < 4; i++ {
		sc.perm(id["Mountain"], 0)
	}
	heist := sc.hand(id["Great Train Heist"], 0)
	sc.do(t, AkCast, TCode(heist), scAny)
	charmnumPickMode(t, sc, 0)
	sc.do(t, AkDone, scAny, scAny)
	sc.resolveAll(t)
}

// stepInExtraCombat reports the queued extra combat phases (0 or 1 for this
// program) and the splice point of the first.
func stepInExtraCombats(sc *scen) int {
	n := 0
	for i := 0; i < int(sc.s.g.nxph); i++ {
		if (sc.s.g.xph[i].at>>4)&3 == kop.AddPhaseCombat {
			n += int(sc.s.g.xph[i].n)
		}
	}
	return n
}

// TestStepInCondition: ConditionPhases$ BeginCombat->EndCombat beside
// ConditionPlayerTurn$ True gates Great Train Heist's extra combat on the
// controller's own combat phase. The mutation makes the step test always true:
// a main-phase cast then queues the extra combat it must not.
func TestStepInCondition(t *testing.T) {
	t.Run("own combat adds the extra combat", func(t *testing.T) {
		sc, id := stepInScen(t)
		sc.s.step = kop.StepBlockers
		sc.s.prio = 0
		sc.s.passes = 0
		sc.s.ctl = ctlPriority
		sc.stepInCast(t, id)
		if got := stepInExtraCombats(sc); got != 1 {
			t.Fatalf("extra combats queued %d, want 1 in the controller's combat phase", got)
		}
	})
	t.Run("own main phase adds none", func(t *testing.T) {
		sc, id := stepInScen(t)
		sc.stepInCast(t, id)
		if got := stepInExtraCombats(sc); got != 0 {
			t.Fatalf("extra combats queued %d, want 0 in the controller's main phase", got)
		}
	})
	t.Run("opponent's combat adds none", func(t *testing.T) {
		sc, id := stepInScen(t)
		sc.s.active = 1
		sc.s.step = kop.StepBlockers
		sc.s.prio = 0
		sc.s.passes = 0
		sc.s.ctl = ctlPriority
		sc.stepInCast(t, id)
		if got := stepInExtraCombats(sc); got != 0 {
			t.Fatalf("extra combats queued %d, want 0 in the opponent's combat phase", got)
		}
	})
	t.Run("always-true mutation diverges in the main phase", func(t *testing.T) {
		sc, id := stepInScen(t)
		tbl := *sc.k.t
		tbl.Exprs = append([]kop.Expr(nil), tbl.Exprs...)
		found := false
		for i := range tbl.Exprs {
			if tbl.Exprs[i].Kind == kop.ExStepIn {
				tbl.Exprs[i].N = int16(1<<kop.NumSteps) - 1
				found = true
			}
		}
		if !found {
			t.Fatal("the lowered table carries no ExStepIn expression")
		}
		sc2 := newScenTable(t, &tbl, Config{NoElide: true})
		sc2.stepInCast(t, id)
		if got := stepInExtraCombats(sc2); got != 1 {
			t.Fatalf("with the step test always true the main-phase cast queued %d extra combats, want 1 (the mutation must diverge)", got)
		}
	})
}
