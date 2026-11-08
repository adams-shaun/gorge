package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestPotentialWitnessReachesPastAnOpponentsWaitingTrigger: a sorcery whose
// only {W} is a Treasure-like token's, beside an opponent's permanent whose
// optional trigger hears the token's sacrifice (Zoetic Glyph's discover after
// its Treasure paid for Guardian of the Great Door). The trigger waits until
// the spell is cast (CR 601.2g, 603.3b), so the cast is payable: the witness
// walk, which lets the trigger resolve first, takes any answer its controller
// may give it rather than calling the witness unreachable.
func TestPotentialWitnessReachesPastAnOpponentsWaitingTrigger(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9996, "Name:Gem Sorcery\nManaCost:1 W\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 1\nOracle:x\n")
	mountain := "Name:Hill\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ R | SpellDescription$ x\nOracle:x\n"
	treasure := "Name:Loot Gem\nTypes:Artifact\nA:AB$ Mana | Cost$ T Sac<1/CARDNAME> | Produced$ Any | SpellDescription$ x\nOracle:x\n"
	watcher := "Name:Gem Mourner\nTypes:Creature Imp\nPT:1/1\n" +
		"T:Mode$ Sacrificed | ValidCard$ Permanent.token | Origin$ Any | Destination$ Battlefield | TriggerZones$ Battlefield | OptionalDecider$ You | Execute$ TrigMournerDraw | TriggerDescription$ x\n" +
		"SVar:TrigMournerDraw:DB$ Draw | NumCards$ 1 | Defined$ You\nOracle:x\n"
	onBoard(t, e, 0, mountain)
	putToken(t, e, 0, treasure, state.ZBattlefield)
	onBoard(t, e, 1, watcher)
	toMain1(t, e)
	moveToHand(t, e, spell)
	ppPriority(t, e)
	got := faceCastPlan(t, e.PotentialPaymentPlans(0), spell, "")
	if got.Reason != "" || got.Plan == nil {
		t.Fatalf("potential cast: reason %q detail %q, want a plan through the gem", got.Reason, got.Detail)
	}
	if !paymentPlanConsumes(*got.Plan) {
		t.Fatalf("witness %v does not sacrifice the gem: the walk under test never ran", got.Plan)
	}
}
