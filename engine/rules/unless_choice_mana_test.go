package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// choiceLand is a land whose one mana ability names the colour on activation.
func choiceLand(produced string) string {
	return "Name:Choice Land\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ " + produced +
		" | SpellDescription$ Tap for one mana of a named choice.\nOracle:x\n"
}

// TestUnlessCostReachesChoiceMana pins CR 605.3a at the offer gate: a mana
// ability whose colour is chosen on activation can pay an unless cost, one tap
// for one of its colours.
func TestUnlessCostReachesChoiceMana(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, produced, chosen string
		payable, unpayable     []string
	}{
		{"a combination", "Combo W B", "", []string{"1", "W", "B"}, []string{"2", "U", "W B"}},
		{"any colour", "Any", "", []string{"1", "U", "G"}, []string{"2", "U G"}},
		{"a chosen colour", "Combo R Chosen", "G", []string{"1", "R", "G"}, []string{"2", "U"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := handEngine(t)
			id := onBoard(t, e, 0, choiceLand(tc.produced))
			e.G.Obj(id).ChosenColor = tc.chosen
			for _, c := range tc.payable {
				if !e.UnlessCostPayable(0, c) {
					t.Errorf("{%s} is not payable", c)
				}
			}
			for _, c := range tc.unpayable {
				if e.UnlessCostPayable(0, c) {
					t.Errorf("{%s} is payable from one tap", c)
				}
			}
		})
	}
}

// TestCounterUnlessPaidFromChoiceMana runs the payment: the window offers the
// land, the tap asks for the colour, and the spell is not countered.
func TestCounterUnlessPaidFromChoiceMana(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _, bear := counterFixture(t, reg, "Daze", "Grizzly Bears")
	e.G.Players[0].Pool = state.Mana{}
	id := onBoard(t, e, 0, choiceLand("Combo W B"))

	d := drainUntilUnlessPay(t, e, 30)
	if d == nil || len(d.Options) != 2 {
		t.Fatalf("no payable election: %+v", d)
	}
	submitChoices(t, e, d.Options[0].Index)
	d = e.Pending()
	if d == nil || d.ResumeKind != "unless_mana" {
		t.Fatalf("no mana window after choosing to pay: %+v", d)
	}
	submitChoices(t, e, optionForObj(d, id))
	if !e.G.Obj(id).Tapped {
		t.Fatal("the land did not tap")
	}
	submitChoices(t, e, optionForKind(t, e, "mana"))
	submitChoices(t, e, optionForKind(t, e, "done"))
	passUntilStackEmpty(t, e, 20)
	if z := e.G.Obj(bear).Zone; z != state.ZBattlefield {
		t.Fatalf("the paid-for spell is in zone %v", z)
	}
}
