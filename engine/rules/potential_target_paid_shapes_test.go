package rules

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// A spell's targets are chosen at CR 601.2c, before the mana abilities that
// pay for it (601.2g) and its costs (601.2h), so a spell whose only legal
// target is the gem its own payment sacrifices is castable and then resolves
// with that target gone (608.2b). Each case below is one cast shape the plain
// payment planner withholds; every one is still offered and cast.
func TestOnlyTargetPaidShapesAreCastable(t *testing.T) {
	t.Parallel()
	head := "Name:Gem Breaker %s\nManaCost:1 R\nTypes:Instant\n"
	cases := []struct {
		name  string
		spell string
		board []string
	}{
		{"additional_cost", head + "A:SP$ Destroy | Cost$ 1 R PayLife<1> | ValidTgts$ Artifact.Treasure | TgtPrompt$ Choose a gem\nOracle:x\n", nil},
		{"optional_cost", head + "K:Multikicker:2\nA:SP$ Destroy | Cost$ 1 R | ValidTgts$ Artifact.Treasure | TgtPrompt$ Choose a gem\nOracle:x\n", nil},
		{"contribution", head + "K:Convoke\nA:SP$ Destroy | Cost$ 1 R | ValidTgts$ Artifact.Treasure | TgtPrompt$ Choose a gem\nOracle:x\n", nil},
		{"mana_spent_reader", head + "A:SP$ Destroy | Cost$ 1 R | ValidTgts$ Artifact.Treasure | TgtPrompt$ Choose a gem | SubAbility$ DBHeal\n" +
			"SVar:DBHeal:DB$ GainLife | LifeAmount$ X\nSVar:X:Count$Converge\nOracle:x\n", nil},
		{"target_dependent_cost", head + "A:SP$ Destroy | Cost$ 1 R | ValidTgts$ Artifact.Treasure | TgtPrompt$ Choose a gem\nOracle:x\n",
			[]string{"Name:Gem Discount\nTypes:Enchantment\nS:Mode$ ReduceCost | ValidCard$ Card | Type$ Spell | Activator$ You | ValidTarget$ Artifact | Amount$ 1 | Description$ Spells you cast that target an artifact cost less.\nOracle:x\n"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e, _, _ := newFixtureDeck(t, 9412, ppFiller)
			spell := handCard(e, card(t, fmt.Sprintf(tc.spell, tc.name)), 0)
			gem := onBoard(t, e, 0, tpTreasure)
			onBoard(t, e, 0, lrForest)
			onBoard(t, e, 0, lrForest)
			for _, src := range tc.board {
				onBoard(t, e, 0, src)
			}
			life := e.G.Players[0].Life
			e.paidManaCensus = true
			ppPriority(t, e)
			e.EnsurePaymentActions()
			d := e.Pending()
			if !ppHasAction(d, spell) {
				t.Fatalf("no planned cast for the spell whose only target is the gem it pays with")
			}
			submitPaymentPlan(t, e, d, ppAction(t, d, spell))
			for guard := 0; guard < 20; guard++ {
				d := e.Pending()
				if d == nil || d.Kind == decision.KPriority {
					break
				}
				in := decision.Intent{Seq: d.Seq, Player: d.Player}
				if d.Kind == decision.KChoose {
					for _, o := range d.Options {
						in.Choices = append(in.Choices, o.Index)
						if len(in.Choices) >= max(d.Min, 1) {
							break
						}
					}
				} else {
					in.Choices = []int{d.Options[0].Index}
				}
				if err := e.Submit(in); err != nil {
					t.Fatal(err)
				}
			}
			if len(e.G.Stack) != 1 || e.G.Obj(gem).Zone == state.ZBattlefield {
				t.Fatalf("stack %v, gem in zone %v: want the spell cast and the gem sacrificed to pay", e.G.Stack, e.G.Obj(gem).Zone)
			}
			if tc.name == "additional_cost" && e.G.Players[0].Life != life-1 {
				t.Fatalf("life %d, want %d: the cast's life payment was skipped", e.G.Players[0].Life, life-1)
			}
			for guard := 0; guard < 4 && len(e.G.Stack) > 0; guard++ {
				ppChoose(t, e, func(o decision.Option) bool { return o.Kind == "pass" })
			}
			if len(e.G.Stack) != 0 || e.G.Obj(spell).Zone != state.ZGraveyard {
				t.Fatalf("stack %v, spell in %v: want it resolved to the graveyard", e.G.Stack, e.G.Obj(spell).Zone)
			}
		})
	}
}
