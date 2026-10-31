package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestDamageAllReplaceDyingDefinedExilesInstead closes the printed clause of
// Anger of the Gods ("If a creature dealt damage this way would die this
// turn, exile it instead") end to end, through the real engine. effDamageAll
// damages the sweep and -- with RememberDamaged$ True -- remembers the hit
// creatures; registerReplaceDying then turns the state-based lethal-damage
// move into an exile. Before the fix effDamageAll never called
// registerReplaceDying, so the 2/2 went to the graveyard.
func TestDamageAllReplaceDyingDefinedExilesInstead(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	anger := mustCorpusCard(t, reg, "Anger of the Gods")
	bear := mustCorpusCard(t, reg, "Grizzly Bears")
	e, cfg := censusEngine(t, 9331, []*cards.Card{anger, bear}, nil)
	cardToHand(t, e, anger)
	bear0 := moveOwnerCard(t, e, 0, bear, state.ZBattlefield)
	addMana(t, e, 0, "GRR")
	castFirst(t, e, "cast")
	passUntilStackEmpty(t, e, 20)

	if o := e.G.Obj(bear0); o.Zone != state.ZExile {
		t.Fatalf("the dealt-damage bear should be exiled, not dying: %s", o.Zone)
	}
	if !movedToExile(e, bear0) {
		t.Fatal("no logged MoveZone into exile for the swept creature")
	}
	n := 0
	for _, ce := range e.continuous {
		if ce.ReplacementEvent == "Moved" {
			n++
			if len(ce.Remembered) != 1 || ce.Remembered[0] != bear0 {
				t.Fatalf("replacement remembered %v, want the bear %d", ce.Remembered, bear0)
			}
		}
	}
	if n != 1 {
		t.Fatalf("want exactly one Moved replacement registered, got %d", n)
	}
	replayCheck(t, e, cfg)
}

// The exile rider registers when the damage batch ends, and a batch stopped at
// a later recipient's replacement choice has not ended: nothing is logged after
// the posed ask, and the answer's re-execution registers the rider.
func TestReplaceDyingWaitsOutAMidBatchChoice(t *testing.T) {
	t.Parallel()
	const ember = "Name:Test Ember\nManaCost:R\nTypes:Sorcery\n" +
		"A:SP$ DealDamage | ValidTgts$ Creature | TargetMin$ 2 | TargetMax$ 2 | NumDmg$ 2 | ReplaceDyingDefined$ Targeted | SpellDescription$ Two creatures smoulder.\nOracle:x\n"
	const shielded = "Name:Test Shielded\nTypes:Creature\nPT:9/9\n" +
		"R:Event$ DamageDone | ActiveZones$ Battlefield | ValidTarget$ Card.Self | ReplaceWith$ More\n" +
		"SVar:More:DB$ ReplaceEffect | VarName$ DamageAmount | VarValue$ OneMore\nSVar:OneMore:ReplaceCount$DamageAmount/Plus.1\n" +
		"R:Event$ DamageDone | ActiveZones$ Battlefield | ValidTarget$ Card.Self | ReplaceWith$ Most\n" +
		"SVar:Most:DB$ ReplaceEffect | VarName$ DamageAmount | VarValue$ TwoMore\nSVar:TwoMore:ReplaceCount$DamageAmount/Plus.2\nOracle:x\n"
	e := handEngine(t, card(t, ember))
	victim := onBoard(t, e, 1, testWarden)
	onBoard(t, e, 1, shielded)
	e.G.Players[0].Pool[state.MR] = 1
	e.askPriority(0)
	castFirst(t, e, "cast")
	submitChoices(t, e, 0, 1)

	posed := false
	for len(e.G.Stack) > 0 {
		d := e.Pending()
		if d.Kind == decision.KPriority {
			submitChoices(t, e, passPriorityOption(t, d))
			continue
		}
		if d.Kind != decision.KReplacement {
			t.Fatalf("pending %s, want the shielded creature's replacement choice", d.Kind)
		}
		posed = true
		if got := e.L.Events[len(e.L.Events)-1].Kind; got != events.DecisionAsk {
			t.Fatalf("last event of a posed replacement choice is %v, want its ask", got)
		}
		submitChoices(t, e, 0)
	}
	if !posed {
		t.Fatal("the damage batch never stopped at a replacement choice")
	}
	if got := e.G.Obj(victim).Zone; got != state.ZExile {
		t.Fatalf("damaged creature zone = %s, want exile", got)
	}
}
