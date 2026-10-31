package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestTriggeredPayNotOfferedWithoutTheColour (dirac pair 589, Flamewake
// Phoenix with only Tranquil Cove untapped): a triggered "you may pay {R}"
// whose payer's one untapped source makes {W} or {U} offers decline only
// (CR 118.3). PotentialMana prices the Cove's "Combo W U" unbounded in every
// colour, which used to offer "Pay R"; answering it opened a window that could
// only fail.
func TestTriggeredPayNotOfferedWithoutTheColour(t *testing.T) {
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Frenzied Goblin")
	goblin := searchMoveByName(t, e, "Frenzied Goblin", state.ZBattlefield)
	e.G.Obj(goblin).SummonSick = false
	xFoldLand(t, e, "Test Cove", "Combo W U")
	target := onBoard(t, e, 1, "Name:Test Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")

	driveToStep(t, e, e.G.Turn, e.G.Active, state.StepDeclareAttackers)
	submitAttackersOnly(t, e, goblin)
	d := passUntilNonPriority(t, e, 40)
	if d.Kind != decision.KTarget {
		t.Fatalf("ask 1 = %s %q, want the target ask", d.Kind, d.Prompt)
	}
	pick := -1
	for _, o := range d.Options {
		if o.Obj == target {
			pick = o.Index
		}
	}
	submitChoices(t, e, pick)

	election := passUntilNonPriority(t, e, 40)
	if election.Kind != decision.KChoose {
		t.Fatalf("ask 2 = %s %q, want the pay election", election.Kind, election.Prompt)
	}
	var kinds []string
	for _, o := range election.Options {
		kinds = append(kinds, o.Kind)
	}
	if len(kinds) != 1 || kinds[0] != "trigger_cost_decline" {
		t.Fatalf("election option kinds = %v, want [trigger_cost_decline] (no red source)", kinds)
	}
}

// TestTriggeredPayNotOfferedFromRestrictedMana (dirac pairs 1638 and
// 6403, Frenzied Goblin with only Secluded Courtyard, or only a white
// Heraldic Banner and a Plains, untapped): the election offers decline only
// when no untapped source can make mana that may pay the triggered "you may
// pay {R}". Mana whose RestrictValid$ spends only on a creature spell or a
// creature's activated ability cannot (CR 106.6), and a Produced$ Chosen
// source makes only its chosen colour (CR 106.1a). resPayNeedsMana now
// prices the payer's sources through potentialManaFor, which drops the
// restricted ability and reads the chosen colour; PotentialMana's priority
// over-bound is unchanged. The control arms (unrestricted "any color", a red
// Banner) still offer the pay.
func TestTriggeredPayNotOfferedFromRestrictedMana(t *testing.T) {
	for _, tc := range []struct {
		name, produced, chosen string
		wantPay                bool
	}{
		{"any", "Any", "", true},
		{"restricted", "Any | RestrictValid$ Spell.Creature,Activated.Creature", "", false},
		{"chosen red", "Chosen", "R", true},
		{"chosen white", "Chosen", "W", false},
	} {
		reg := searchTestRegistry(t)
		e, _ := searchEngine(t, reg, "Frenzied Goblin")
		goblin := searchMoveByName(t, e, "Frenzied Goblin", state.ZBattlefield)
		e.G.Obj(goblin).SummonSick = false
		src := xFoldLand(t, e, "Test Source", tc.produced)
		e.G.Obj(src).ChosenColor = tc.chosen
		target := onBoard(t, e, 1, "Name:Test Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")

		driveToStep(t, e, e.G.Turn, e.G.Active, state.StepDeclareAttackers)
		submitAttackersOnly(t, e, goblin)
		d := passUntilNonPriority(t, e, 40)
		if d.Kind != decision.KTarget {
			t.Fatalf("%s: ask 1 = %s %q, want the target ask", tc.name, d.Kind, d.Prompt)
		}
		pick := -1
		for _, o := range d.Options {
			if o.Obj == target {
				pick = o.Index
			}
		}
		submitChoices(t, e, pick)

		election := passUntilNonPriority(t, e, 40)
		if election.Kind != decision.KChoose {
			t.Fatalf("%s: ask 2 = %s %q, want the pay election", tc.name, election.Kind, election.Prompt)
		}
		pays := false
		for _, o := range election.Options {
			pays = pays || o.Kind == "trigger_cost_pay"
		}
		if pays != tc.wantPay {
			t.Fatalf("%s: pay offered = %v, want %v (options %+v)", tc.name, pays, tc.wantPay, election.Options)
		}
	}
}
