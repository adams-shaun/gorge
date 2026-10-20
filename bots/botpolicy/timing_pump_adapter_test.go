package botpolicy_test

import (
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// TestPumpAdaptersAgree pins the A6a card fact both halves must fill: the
// game-shaped BoardFromGame and the view-shaped BoardFromView read the same
// literal pump bonus off the same printed card. The view half is built
// through view.Project -- the REAL projection a client receives -- so a
// reverted projection line fails here, not only in the whole-game parity
// tests. A missing fill on either half makes the timing rule see a different
// board depending on the host.
func TestPumpAdaptersAgree(t *testing.T) {
	for _, tc := range []struct {
		name  string
		src   string
		power int32
		tough int32
	}{
		{
			name:  "literal pump",
			src:   "Name:Giant Growth\nManaCost:G\nTypes:Instant\nOracle:Target creature gets +3/+3 until end of turn.\nA:SP$ Pump | ValidTgts$ Creature | NumAtt$ +3 | NumDef$ +3 | SpellDescription$ Target creature gets +3/+3 until end of turn.\n",
			power: 3,
			tough: 3,
		},
		{
			name:  "power only",
			src:   "Name:Rush of Adrenaline\nManaCost:R\nTypes:Instant\nOracle:Target creature gets +2/+0 and gains haste until end of turn.\nA:SP$ Pump | ValidTgts$ Creature | NumAtt$ +2 | SpellDescription$ Target creature gets +2/+0 and gains haste until end of turn.\n",
			power: 2,
		},
		{
			name: "negative bonus",
			src:  "Name:Shrink\nManaCost:U\nTypes:Instant\nOracle:Target creature gets -2/-2 until end of turn.\nA:SP$ Pump | ValidTgts$ Creature | NumAtt$ -2 | NumDef$ -2 | SpellDescription$ Target creature gets -2/-2 until end of turn.\n",
		},
		{
			name: "dynamic amount",
			src:  "Name:Untamed Might\nManaCost:X G\nTypes:Instant\nOracle:Target creature gets +X/+X until end of turn.\nA:SP$ Pump | ValidTgts$ Creature | NumAtt$ +X | NumDef$ +X | SpellDescription$ Target creature gets +X/+X until end of turn.\n",
		},
		{
			name: "not a creature",
			src:  "Name:Twiddle\nManaCost:U\nTypes:Instant\nOracle:Tap or untap target artifact.\nA:SP$ Pump | ValidTgts$ Artifact | NumAtt$ +1 | SpellDescription$ Tap or untap target artifact.\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			card, diags := cards.ParseBytes("timing_pump_adapter_test.go", []byte(tc.src))
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
			if cv.PumpPower != tc.power || cv.PumpToughness != tc.tough {
				t.Fatalf("projected pump = %d/%d, want %d/%d", cv.PumpPower, cv.PumpToughness, tc.power, tc.tough)
			}

			gameCard := botpolicy.BoardFromGame(g, printedChars{}, 0).Cards.Get(o.ID)
			viewCard := heuristic.BoardFromView(v).Cards.Get(o.ID)
			if gameCard.PumpPower != tc.power || gameCard.PumpToughness != tc.tough {
				t.Fatalf("game facts pump = %d/%d, want %d/%d", gameCard.PumpPower, gameCard.PumpToughness, tc.power, tc.tough)
			}
			if viewCard.PumpPower != tc.power || viewCard.PumpToughness != tc.tough {
				t.Fatalf("view facts pump = %d/%d, want %d/%d", viewCard.PumpPower, viewCard.PumpToughness, tc.power, tc.tough)
			}
		})
	}
}
