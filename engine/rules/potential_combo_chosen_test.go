package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// comboChosenElection attacks with Frenzied Goblin ("you may pay {R}") while
// the seat's only untapped source is a Thriving-style land producing
// `produced` with the given as-enters colour, and returns the kinds of the
// pay election's options.
func comboChosenElection(t *testing.T, produced, chosen string) []string {
	t.Helper()
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Frenzied Goblin")
	goblin := searchMoveByName(t, e, "Frenzied Goblin", state.ZBattlefield)
	e.G.Obj(goblin).SummonSick = false
	land := xFoldLand(t, e, "Test Thriving", produced)
	e.G.Obj(land).ChosenColor = chosen
	target := onBoard(t, e, 1, "Name:Test Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")

	driveToStep(t, e, e.G.Turn, e.G.Active, state.StepDeclareAttackers)
	submitAttackersOnly(t, e, goblin)
	d := passUntilNonPriority(t, e, 40)
	if d.Kind != decision.KTarget {
		t.Fatalf("ask 1 = %s %q, want the target ask", d.Kind, d.Prompt)
	}
	for _, o := range d.Options {
		if o.Obj == target {
			submitChoices(t, e, o.Index)
			break
		}
	}
	d = passUntilNonPriority(t, e, 40)
	if d.Kind != decision.KChoose {
		t.Fatalf("ask 2 = %s %q, want the pay election", d.Kind, d.Prompt)
	}
	var kinds []string
	for _, o := range d.Options {
		kinds = append(kinds, o.Kind)
	}
	return kinds
}

// TestPotentialComboChosenPaymentBound pins CR 106.1a / 118.3 on a payment's
// potential bound: a "Combo W Chosen" source (a Thriving land) with chosen
// colour G makes W or G, so a trigger's "you may pay {R}" is not offered as a
// pay; with chosen colour R it is.
func TestPotentialComboChosenPaymentBound(t *testing.T) {
	t.Parallel()
	got := comboChosenElection(t, "Combo W Chosen", "G")
	if want := []string{"trigger_cost_decline"}; !slices.Equal(got, want) {
		t.Fatalf("W-or-chosen-G land under a pay {R} trigger: options %v, want %v", got, want)
	}
	got = comboChosenElection(t, "Combo W Chosen", "R")
	if !slices.Contains(got, "trigger_cost_pay") {
		t.Fatalf("control: W-or-chosen-R land under a pay {R} trigger: options %v, want a pay option", got)
	}
}
