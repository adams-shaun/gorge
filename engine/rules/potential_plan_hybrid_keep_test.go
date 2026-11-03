package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPotentialHybridPlanKeepsSourcesInEveryColouring: a hybrid cast whose
// first colouring ({B/R}{B/R} as two black) can only be paid by sacrificing a
// Treasure is still payable from the untapped lands alone as two red (CR
// 107.4e), and the sacrifice would also fire a drain trigger that ends the game
// before the cast. The witness must come from the colouring that keeps
// every source, not be withheld as unreachable.
func TestPotentialHybridPlanKeepsSourcesInEveryColouring(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9997, "Name:Gem Sorcery\nManaCost:1 BR BR\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 1\nOracle:x\n")
	mountain := "Name:Hill\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ R | SpellDescription$ x\nOracle:x\n"
	dual := "Name:Dual Hill\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ Combo B R | SpellDescription$ x\nOracle:x\n"
	treasure := "Name:Loot Gem\nTypes:Artifact\nA:AB$ Mana | Cost$ T Sac<1/CARDNAME> | Produced$ Any | SpellDescription$ x\nOracle:x\n"
	watcher := "Name:Gem Watcher\nTypes:Creature Imp\nPT:1/1\n" +
		"T:Mode$ Sacrificed | ValidPlayer$ You | ValidCard$ Permanent.token+YouCtrl | Origin$ Any | Destination$ Battlefield | TriggerZones$ Battlefield | Execute$ TrigDrain | TriggerDescription$ x\n" +
		"SVar:TrigDrain:DB$ LoseLife | ValidTgts$ Opponent | LifeAmount$ 1\nOracle:x\n"
	onBoard(t, e, 0, mountain)
	onBoard(t, e, 0, mountain)
	onBoard(t, e, 0, dual)
	putToken(t, e, 0, treasure, state.ZBattlefield)
	onBoard(t, e, 0, watcher)
	toMain1(t, e)
	moveToHand(t, e, spell)
	// The opponent is at one life, so the gem's sacrifice would end the game
	// on the clone the witness is replayed on.
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: 1 - e.G.Players[1].Life})
	ppPriority(t, e)
	got := faceCastPlan(t, e.PotentialPaymentPlans(0), spell, "")
	if got.Reason != "" || got.Plan == nil {
		t.Fatalf("potential cast: reason %q detail %q plan %v, want a plan from the lands", got.Reason, got.Detail, got.Plan)
	}
	for _, a := range got.Plan.Activations {
		if a.Consequence != nil && a.Consequence.Sacrifice {
			t.Fatalf("witness sacrifices %d though the lands pay as {R}{R}", a.Source)
		}
	}
}

// TestPotentialWitnessReachesAWinningDrain: the same cast with only the two
// Mountains beside the gem needs the gem's mana, and its sacrifice drains the
// opponent's last life. The drain waits until the spell is cast (CR 603.3b),
// so the cast is payable; a game that ends in the caster's favour is not an
// unreachable witness.
func TestPotentialWitnessReachesAWinningDrain(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9998, "Name:Gem Sorcery\nManaCost:1 BR BR\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 1\nOracle:x\n")
	mountain := "Name:Hill\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ R | SpellDescription$ x\nOracle:x\n"
	treasure := "Name:Loot Gem\nTypes:Artifact\nA:AB$ Mana | Cost$ T Sac<1/CARDNAME> | Produced$ Any | SpellDescription$ x\nOracle:x\n"
	watcher := "Name:Gem Watcher\nTypes:Creature Imp\nPT:1/1\n" +
		"T:Mode$ Sacrificed | ValidPlayer$ You | ValidCard$ Permanent.token+YouCtrl | Origin$ Any | Destination$ Battlefield | TriggerZones$ Battlefield | Execute$ TrigDrain | TriggerDescription$ x\n" +
		"SVar:TrigDrain:DB$ LoseLife | ValidTgts$ Opponent | LifeAmount$ 1\nOracle:x\n"
	onBoard(t, e, 0, mountain)
	onBoard(t, e, 0, mountain)
	putToken(t, e, 0, treasure, state.ZBattlefield)
	onBoard(t, e, 0, watcher)
	toMain1(t, e)
	moveToHand(t, e, spell)
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: 1 - e.G.Players[1].Life})
	ppPriority(t, e)
	got := faceCastPlan(t, e.PotentialPaymentPlans(0), spell, "")
	if got.Reason != "" || got.Plan == nil {
		t.Fatalf("potential cast: reason %q detail %q, want a plan through the gem", got.Reason, got.Detail)
	}
}
