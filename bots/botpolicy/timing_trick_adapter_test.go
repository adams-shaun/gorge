package botpolicy_test

import (
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

func TestCombatTrickAdaptersAgree(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want bool
	}{
		{
			name: "pump",
			src:  "Name:Giant Growth\nManaCost:G\nTypes:Instant\nOracle:x\nA:SP$ Pump | ValidTgts$ Creature | NumAtt$ +3 | NumDef$ +3 | SpellDescription$ x\n",
			want: true,
		},
		{
			name: "curse",
			src:  "Name:Stab\nManaCost:B\nTypes:Instant\nOracle:x\nA:SP$ Pump | ValidTgts$ Creature | NumAtt$ -1 | NumDef$ -1 | SpellDescription$ x\n",
		},
		{
			name: "counter",
			src:  "Name:Snakeskin Veil\nManaCost:G\nTypes:Instant\nOracle:x\nA:SP$ PutCounter | ValidTgts$ Creature.YouCtrl | CounterType$ P1P1 | CounterNum$ 1 | SpellDescription$ x\n",
			want: true,
		},
		{
			name: "pump all",
			src:  "Name:Claws Out\nManaCost:3 W W\nTypes:Instant\nOracle:x\nA:SP$ PumpAll | ValidCards$ Creature.YouCtrl | NumAtt$ +2 | NumDef$ +2 | SpellDescription$ x\n",
			want: true,
		},
		{
			name: "keyword only",
			src:  "Name:Divine Resilience\nManaCost:W\nTypes:Instant\nOracle:x\nA:SP$ Pump | ValidTgts$ Creature.YouCtrl | KW$ Indestructible | SpellDescription$ x\n",
			want: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			card, diags := cards.ParseBytes("timing_trick_adapter_test.go", []byte(tc.src))
			if len(diags) != 0 {
				t.Fatalf("parse: %v", diags)
			}
			card.Link()
			g := state.NewGame([]string{"a", "b"})
			o := g.AddObject(card, 0)
			g.SetZone(state.ZHand, 0, []state.ObjID{o.ID})

			v := view.Project(g, nil, 0, nil)
			var cv *view.CardView
			for i := range v.Players[0].Hand {
				if v.Players[0].Hand[i].ID == o.ID {
					cv = &v.Players[0].Hand[i]
				}
			}
			if cv == nil {
				t.Fatalf("projection: hand card %d missing", o.ID)
			}
			if cv.CombatTrick != tc.want {
				t.Fatalf("projected CombatTrick = %v, want %v", cv.CombatTrick, tc.want)
			}
			gameCard := botpolicy.BoardFromGame(g, printedChars{}, 0).Cards.Get(o.ID)
			viewCard := heuristic.BoardFromView(v).Cards.Get(o.ID)
			if gameCard.CombatTrick != tc.want || viewCard.CombatTrick != tc.want {
				t.Fatalf("game/view CombatTrick = %v/%v, want %v", gameCard.CombatTrick, viewCard.CombatTrick, tc.want)
			}
		})
	}
}
