package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// trigreferents11Forms: the MBC trigger-referents-11 spellings, one carrier
// each, checked like paramreadformsForms.
var trigreferents11Forms = []formCase{
	{card: "Shanid, Sleepers' Scourge", form: "LandPlayed ValidActivatingPlayer$ You (gorge landPlayedMatches never reads it; ValidCard$ YouCtrl names the player)", lir: "LandPlayed"},
	{card: "Xyris, the Writhing Storm", form: "Draw Defined$ You & TriggeredTarget (the controller draws, then the damaged player)", lir: "obj=You A=TriggerAmount\n     1 Draw obj=TriggeredPlayer A=TriggerAmount"},
	{card: "Throat Slitter", form: "Creature.nonBlack+ControlledBy TriggeredTarget in its own combat-damage-to-a-player trigger (PredTrigPlayerCtrl beside a colour test)", lir: "cmp(16 0 0  )"},
	{card: "Lightwielder Paladin", form: "Permanent.Black+ControlledBy TriggeredTarget,Permanent.Red+ControlledBy TriggeredTarget (one PredTrigPlayerCtrl clause per alternative)", lir: "cmp(16 0 0  )"},
	{card: "Norwood Priestess", form: "ActivationFirstCombat$ True is read (the activation condition: combats begun <= 1) beside the ActivationPhases$ window", lir: "PhaseCount"},
}

func TestTrigReferents11Forms(t *testing.T) { checkForms(t, trigreferents11Forms) }

// TestActivationFirstCombatLowers: ActivationFirstCombat$ True on an
// activated ability lowers to an activation condition on the combats begun
// this turn (gorge activationPhasesOK withholds the offer once
// CombatsThisTurn > 1); any other value, or a mana ability, refuses. Norwood
// Priestess carries it beside ActivationPhases$; the carrier here is inline
// so the False and mana-ability refusals are reachable.
func TestActivationFirstCombatLowers(t *testing.T) {
	probe := func(fc string) *lCard {
		t.Helper()
		src := "Name:Combat Probe\nManaCost:2 G\nTypes:Creature Elf\nPT:1/1\n" +
			"A:AB$ Draw | Cost$ T | Defined$ You | NumCards$ 1 | ActivationFirstCombat$ " + fc + "\n"
		card, diags := cards.ParseBytes("combat_probe.txt", []byte(src))
		if card == nil {
			t.Fatalf("parse: %v", diags)
		}
		if d := card.Link(); len(d) != 0 {
			t.Fatalf("link: %v", d)
		}
		p := &pool{names: []string{"Combat Probe"}, byName: map[string]*cards.Card{"Combat Probe": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
		return lowerPool(p).cards[0]
	}
	c := probe("True")
	if len(c.Refusals) != 0 {
		t.Fatalf("ActivationFirstCombat$ True refused: %v", c.Refusals)
	}
	var sb strings.Builder
	dumpLIR(&sb, c)
	if !strings.Contains(sb.String(), "PhaseCount") {
		t.Errorf("no combats-begun activation condition in:\n%s", sb.String())
	}
	if c := probe("False"); len(c.Refusals) == 0 {
		t.Error("ActivationFirstCombat$ False lowered")
	}
}
