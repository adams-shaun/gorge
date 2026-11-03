package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A cast of a shape the payment planner withholds (here an optional cost) whose
// only payment sacrifices a gem is the potential walk's to price when the gem's
// sacrifice ends the game in the caster's favour: the drain it sets off waits for
// the cast (CR 603.3b), so the witness reaches the play and the cast is not one
// the planner prices at CR 601.2g on its own account.
func TestWinningDrainWitnessIsNotATargetPaidCast(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9999, "Name:Gem Sorcery\nManaCost:R R\nTypes:Sorcery\nK:Multikicker:2\nA:SP$ Draw | NumCards$ 1\nOracle:x\n")
	hill := "Name:Hill\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ R | SpellDescription$ x\nOracle:x\n"
	gem := "Name:Loot Gem\nTypes:Artifact\nA:AB$ Mana | Cost$ T Sac<1/CARDNAME> | Produced$ Any | SpellDescription$ x\nOracle:x\n"
	watcher := "Name:Gem Watcher\nTypes:Creature Imp\nPT:1/1\n" +
		"T:Mode$ Sacrificed | ValidPlayer$ You | ValidCard$ Permanent.token+YouCtrl | Origin$ Any | Destination$ Battlefield | TriggerZones$ Battlefield | Execute$ TrigDrain | TriggerDescription$ x\n" +
		"SVar:TrigDrain:DB$ LoseLife | ValidTgts$ Opponent | LifeAmount$ 1\nOracle:x\n"
	onBoard(t, e, 0, hill)
	putToken(t, e, 0, gem, state.ZBattlefield)
	onBoard(t, e, 0, watcher)
	toMain1(t, e)
	moveToHand(t, e, spell)
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: 1 - e.G.Players[1].Life})
	ppPriority(t, e)
	e.EnsurePaymentActions()
	if ppHasAction(e.Pending(), spell) {
		t.Fatal("the cast is a planned payment though the gem's sacrifice ends the game and the potential walk reaches it")
	}
	if got := faceCastPlan(t, e.PotentialPaymentPlans(0), spell, ""); got.Plan == nil || got.Reason != "" {
		t.Fatalf("potential cast: reason %q detail %q, want a plan through the gem", got.Reason, got.Detail)
	}
}
