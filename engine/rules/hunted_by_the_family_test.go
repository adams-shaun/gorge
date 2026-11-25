package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Hunted by The Family is `SP$ ChooseCard | ValidTgts$ Creature.YouDontCtrl`
// with no Defined$: Forge's getDefinedPlayersOrTargeted reads only the
// targeted PLAYERS, so with card targets nobody is asked to choose a card
// (CR 115.1: a target does not make its controller a chooser). The only ask
// the opposing creature's controller gets is the villainous choice.
func TestHuntedByTheFamilyAsksNoStrayChooseCard(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := corpusEngineCfg(t, reg,
		[]*cards.Card{lookup(t, reg, "Hunted by The Family")},
		[]*cards.Card{card(t, "Name:Test Walker\nManaCost:1\nTypes:Creature Human\nPT:0/3\nOracle:x\n")})
	walker := moveByName(t, e, 1, "Test Walker", state.ZBattlefield)
	ensureInHand(t, e, 0, "Hunted by The Family")
	addMana(t, e, 0, "UUUUUUU")
	castCardNow(t, e, "Hunted by The Family")

	var kinds []decision.Kind
	var chooseOwners []state.PlayerID
	targeted, animated := false, false
	for i := 0; i < 60 && !(animated); i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			if d = e.Pending(); d == nil {
				break
			}
		}
		kinds = append(kinds, d.Kind)
		var choices []int
		switch d.Kind {
		case decision.KTarget:
			targeted = true
			for _, o := range d.Options {
				if o.Obj == walker {
					choices = []int{o.Index}
				}
			}
			if choices == nil {
				t.Fatalf("walker not offered as a target: %+v", d.Options)
			}
		case decision.KChoose:
			chooseOwners = append(chooseOwners, d.Player)
			choices = []int{d.Options[0].Index}
		case decision.KModes:
			if d.Player != 1 {
				t.Fatalf("villainous choice asked of seat %d, want the creature's controller 1", d.Player)
			}
			choices = []int{d.Options[0].Index} // the animate branch
		case decision.KPriority:
			for _, o := range d.Options {
				if o.Kind == "pass" {
					choices = []int{o.Index}
				}
			}
		default:
			t.Fatalf("unexpected decision %+v", d)
		}
		submitChoices(t, e, choices...)
		if len(e.G.Stack) == 0 && targeted {
			animated = true
		}
	}
	if !targeted {
		t.Fatalf("precondition: the spell never asked for its targets (kinds %v)", kinds)
	}
	if len(chooseOwners) != 0 {
		t.Fatalf("a choose_n ask reached seat(s) %v before/around the villainous choice; Forge asks nobody (kinds %v)", chooseOwners, kinds)
	}
	o := e.G.Obj(walker)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("walker left the battlefield: %+v", o)
	}
	pw, tf := e.Power(walker), e.Toughness(walker)
	if pw != 1 || tf != 1 {
		t.Fatalf("walker is %d/%d after the animate branch, want 1/1", pw, tf)
	}
}
