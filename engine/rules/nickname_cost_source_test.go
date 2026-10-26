package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestNicknameCostPaysAsSource pins that every `<N/NICKNAME>` cost kind the
// corpus carries (Exile: Melira, the Living Cure; Sac: Kagemaro, Arek, ...;
// Discard: Mjolnir, the channel lands; Exert: Basri) is offered and, once
// paid, takes the source itself -- NICKNAME is Forge's alias of CARDNAME.
func TestNicknameCostPaysAsSource(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		src    string
		inHand bool
		bear   bool
		check  func(t *testing.T, e *Engine, id state.ObjID)
	}{
		{
			name: "Exile",
			src: "Name:Nick Exiler\nManaCost:1 W\nTypes:Creature Human\nPT:1/1\n" +
				"A:AB$ GainLife | Cost$ Exile<1/NICKNAME> | LifeAmount$ 3 | SpellDescription$ Gain 3 life.\nOracle:\n",
			check: func(t *testing.T, e *Engine, id state.ObjID) {
				if z := e.G.Obj(id).Zone; z != state.ZExile {
					t.Fatalf("source zone after Exile<1/NICKNAME> = %v, want exile", z)
				}
			},
		},
		{
			// Melira, the Living Cure's shape: a targeted ability whose only
			// cost is "Exile Melira".
			name: "ExileTargeted",
			src: "Name:Nick Melira\nManaCost:1 W\nTypes:Creature Human\nPT:1/1\n" +
				"A:AB$ Pump | Cost$ Exile<1/NICKNAME> | ValidTgts$ Creature.Other | NumAtt$ 1 | SpellDescription$ Target other creature gets +1/+0.\nOracle:\n",
			bear: true,
			check: func(t *testing.T, e *Engine, id state.ObjID) {
				if z := e.G.Obj(id).Zone; z != state.ZExile {
					t.Fatalf("source zone after Exile<1/NICKNAME> = %v, want exile", z)
				}
			},
		},
		{
			name: "Sac",
			src: "Name:Nick Sacrificer\nManaCost:1 W\nTypes:Creature Human\nPT:1/1\n" +
				"A:AB$ GainLife | Cost$ Sac<1/NICKNAME> | LifeAmount$ 3 | SpellDescription$ Gain 3 life.\nOracle:\n",
			check: func(t *testing.T, e *Engine, id state.ObjID) {
				if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
					t.Fatalf("source zone after Sac<1/NICKNAME> = %v, want graveyard", z)
				}
			},
		},
		{
			name:   "Discard",
			inHand: true,
			src: "Name:Nick Discarder\nManaCost:1 W\nTypes:Creature Human\nPT:1/1\n" +
				"A:AB$ GainLife | Cost$ Discard<1/NICKNAME> | ActivationZone$ Hand | LifeAmount$ 3 | SpellDescription$ Gain 3 life.\nOracle:\n",
			check: func(t *testing.T, e *Engine, id state.ObjID) {
				if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
					t.Fatalf("source zone after Discard<1/NICKNAME> = %v, want graveyard", z)
				}
			},
		},
		{
			name: "Exert",
			src: "Name:Nick Exerter\nManaCost:1 W\nTypes:Creature Human\nPT:1/1\n" +
				"A:AB$ GainLife | Cost$ Exert<1/NICKNAME> | LifeAmount$ 3 | SpellDescription$ Gain 3 life.\nOracle:\n",
			check: func(t *testing.T, e *Engine, id state.ObjID) {
				if !e.G.Obj(id).ExertedThisTurn {
					t.Fatal("source is not exerted after Exert<1/NICKNAME>")
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := combatEngine(t)
			var id state.ObjID
			if tc.inHand {
				id = onHand(t, e, 0, tc.src)
			} else {
				id = onBoard(t, e, 0, tc.src)
				e.G.Obj(id).SummonSick = false
			}
			if tc.bear {
				onBoard(t, e, 0, "Name:Nick Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:\n")
			}
			e.G.Active = 0
			e.priorityRound()
			opt, ok := findAbilityOption(e, id, 0)
			if !ok {
				t.Fatalf("%s<1/NICKNAME> activation is not offered: %+v", tc.name, e.Pending())
			}
			before := e.G.Players[0].Life
			if err := e.Submit(decision.Intent{Seq: e.Pending().Seq, Player: 0, Choices: []int{opt.Index}}); err != nil {
				t.Fatalf("activate: %v", err)
			}
			for i := 0; i < 8; i++ {
				d := e.Pending()
				if d == nil || d.Kind == decision.KPriority {
					break
				}
				answerFirst(t, e)
			}
			tc.check(t, e, id)
			passUntilStackEmpty(t, e, 80)
			if tc.bear {
				return // Pump has no life to observe
			}
			if got := e.G.Players[0].Life; got != before+3 {
				t.Fatalf("life after resolution = %d, want %d", got, before+3)
			}
		})
	}
}
