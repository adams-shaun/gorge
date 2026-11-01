package rules

// A RemoveCounter over "any number" of chosen permanents (Garnet's shape:
// Choices$ | ChoiceOptional$ | RememberAmount$) removes one counter from each
// answered permanent and remembers one entry per counter removed, which the
// chained Count$RememberedNumber payoff reads; a decline removes and pays
// nothing; an exact-count pick (Dyadrine's ChoiceNum$ 2) remembers both; and
// a line without RememberAmount$ remembers nothing.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

const sagaSrc = "Types:Enchantment Saga\nK:Chapter:5:C1,C2,C3,C4,C5\n" +
	"SVar:C1:DB$ GainLife | LifeAmount$ 1\nSVar:C2:DB$ GainLife | LifeAmount$ 1\nSVar:C3:DB$ GainLife | LifeAmount$ 1\n" +
	"SVar:C4:DB$ GainLife | LifeAmount$ 1\nSVar:C5:DB$ GainLife | LifeAmount$ 1\nOracle:x\n"

func TestAnyNumberRemoveCounterPaysOffPerCounter(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		picks []int // indices into [sagaA, sagaB]
	}{{"both", []int{0, 1}}, {"one", []int{1}}, {"decline", nil}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := kernelEngine(t, 2)
			garnet := putCardIn(t, e, 0, cardOfSrc(t, "Name:Garnet Fixture\nManaCost:G W\nTypes:Creature Human\nPT:2/2\n"+
				"A:AB$ RemoveCounter | Cost$ 0 | Choices$ Saga.YouCtrl | CounterType$ LORE | CounterNum$ 1 | ChoiceOptional$ True | RememberAmount$ True | SubAbility$ DBPut\n"+
				"SVar:DBPut:DB$ PutCounter | Defined$ Self | CounterType$ P1P1 | CounterNum$ X | SubAbility$ DBCleanup\n"+
				"SVar:DBCleanup:DB$ Cleanup | ClearRemembered$ True\nSVar:X:Count$RememberedNumber\nOracle:x\n"), state.ZBattlefield, false)
			sagas := []state.ObjID{
				putCardIn(t, e, 0, cardOfSrc(t, "Name:Saga A\n"+sagaSrc), state.ZBattlefield, false),
				putCardIn(t, e, 0, cardOfSrc(t, "Name:Saga B\n"+sagaSrc), state.ZBattlefield, false),
			}
			e.G.Obj(sagas[0]).AddCounter("LORE", 2)
			e.G.Obj(sagas[1]).AddCounter("LORE", 3)
			d := requireResumeAsk(t, activatePermanentAbility(t, e, 0, garnet), "counter_pick")
			if d.Min != 0 || len(d.Options) != 2 {
				t.Fatalf("pick = %+v, want a Min 0 pick over both Sagas", d)
			}
			var choices []int
			for _, p := range tc.picks {
				choices = append(choices, objOptionIndex(t, d, sagas[p]))
			}
			if d = answerDecision(t, e, d, choices...); d != nil {
				t.Fatalf("unexpected ask: %+v", d)
			}
			wantA, wantB := int32(2), int32(3)
			for _, p := range tc.picks {
				if p == 0 {
					wantA--
				} else {
					wantB--
				}
			}
			if a, b := e.G.Obj(sagas[0]).Counter("LORE"), e.G.Obj(sagas[1]).Counter("LORE"); a != wantA || b != wantB {
				t.Fatalf("LORE = %d/%d, want %d/%d", a, b, wantA, wantB)
			}
			if got := e.G.Obj(garnet).Counter("P1P1"); got != int32(len(tc.picks)) {
				t.Fatalf("Garnet P1P1 = %d, want %d (one per lore counter removed)", got, len(tc.picks))
			}
			if hasNote(e, "RemoveCounter") {
				t.Fatal("a RemoveCounter degradation Note was emitted")
			}
		})
	}
}

func TestExactCountRemoveCounterRememberAmountFeedsThePayoff(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		remember string
		gain     int32
	}{{"RememberAmount", " | RememberAmount$ True", 2}, {"no RememberAmount", "", 0}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := kernelEngine(t, 2)
			src := putCardIn(t, e, 0, cardOfSrc(t, "Name:Amalgam\nManaCost:G W\nTypes:Artifact\n"+
				"A:AB$ RemoveCounter | Cost$ 0 | CounterType$ P1P1 | Choices$ Creature.YouCtrl+counters_GE1_P1P1 | ChoiceNum$ 2"+tc.remember+" | SubAbility$ DBGain\n"+
				"SVar:DBGain:DB$ GainLife | LifeAmount$ Z | SubAbility$ DBCleanup\n"+
				"SVar:DBCleanup:DB$ Cleanup | ClearRemembered$ True\nSVar:Z:Count$RememberedNumber\nOracle:x\n"), state.ZBattlefield, false)
			var bears []state.ObjID
			for _, n := range []string{"Bear One", "Bear Two"} {
				id := putCardIn(t, e, 0, cardOfSrc(t, "Name:"+n+"\n"+creatureKernelSrc), state.ZBattlefield, false)
				e.G.Obj(id).AddCounter("P1P1", 1)
				bears = append(bears, id)
			}
			life := e.G.Players[0].Life
			// Exactly two eligible creatures for ChoiceNum$ 2: the pick may be
			// forced (no ask); when it is posed, answer both.
			if d := activatePermanentAbility(t, e, 0, src); d != nil {
				requireResumeAsk(t, d, "counter_pick")
				if d = answerDecision(t, e, d, objOptionIndex(t, d, bears[0]), objOptionIndex(t, d, bears[1])); d != nil {
					t.Fatalf("unexpected ask: %+v", d)
				}
			}
			for _, id := range bears {
				if got := e.G.Obj(id).Counter("P1P1"); got != 0 {
					t.Fatalf("creature %d P1P1 = %d, want 0", id, got)
				}
			}
			if got := e.G.Players[0].Life; got != life+tc.gain {
				t.Fatalf("life = %d, want %d (Count$RememberedNumber = %d)", got, life+tc.gain, tc.gain)
			}
		})
	}
}
