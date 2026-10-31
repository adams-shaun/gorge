package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestPumpAmountIsReadOnce pins CR 608.2h: an amount that reads the board is
// determined once, so pumping the first creature cannot grow what the next one
// gets. Greatest power among a 3/3 and a 1/1 is 3, for both.
func TestPumpAmountIsReadOnce(t *testing.T) {
	t.Parallel()
	const tail = " | NumAtt$ +Growth | NumDef$ +Growth | SpellDescription$ Grow them by the largest power.\n" +
		"SVar:Growth:Count$Valid Creature.YouCtrl$GreatestCardPower\nOracle:x\n"
	for _, tc := range []struct{ name, body string }{
		{"a sweep", "A:SP$ PumpAll | ValidCards$ Creature.YouCtrl"},
		{"a defined group", "A:SP$ Pump | Defined$ Valid Creature.YouCtrl"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := handEngine(t, card(t, "Name:Growth\nManaCost:G\nTypes:Sorcery\n"+tc.body+tail))
			big := onBoard(t, e, 0, "Name:Big\nTypes:Creature Bear\nPT:3/3\nOracle:x\n")
			small := onBoard(t, e, 0, "Name:Small\nTypes:Creature Bear\nPT:1/1\nOracle:x\n")
			e.G.Players[0].Pool[state.MG] = 1
			e.priorityRound()
			submitChoices(t, e, castModeOption(t, e, e.G.Zone(state.ZHand, 0)[0], ""))
			passUntilStackEmpty(t, e, 20)
			if p, q := e.Power(big), e.Power(small); p != 6 || q != 4 {
				t.Fatalf("powers %d and %d, want 6 and 4", p, q)
			}
		})
	}
}
