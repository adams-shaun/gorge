package rules

import (
	"testing"
)

const (
	fpbIsland   = "Name:Test Island\nTypes:Land Island\nA:AB$ Mana | Cost$ T | Produced$ U | SpellDescription$ Blue mana.\nOracle:x\n"
	fpbMountain = "Name:Test Mountain\nTypes:Land Mountain\nA:AB$ Mana | Cost$ T | Produced$ R | SpellDescription$ Red mana.\nOracle:x\n"
	fpbScare    = "Name:Test Signpost\nManaCost:4\nTypes:Artifact Creature Scarecrow\nPT:2/4\nK:Vigilance\n" +
		"A:AB$ Mana | Cost$ 2 | Produced$ Any | SpellDescription$ Filter.\nOracle:x\n"
	fpbBouncer = "Name:Test Bouncer\nManaCost:5 U\nTypes:Creature Shapeshifter\nPT:2/2\n" +
		"A:AB$ ChangeZone | Cost$ 2 U U | Defined$ Self | Origin$ Battlefield | Destination$ Hand | SpellDescription$ Return.\nOracle:x\n"
)

// fpbPlay prices the bouncer's activation beside one Island, the given
// Mountains and a Signpost ("{2}: add one mana of any color", no {T}).
func fpbPlay(t *testing.T, mountains int) PotentialPlan {
	t.Helper()
	e, _, _ := newFixtureDeck(t, 9711, ppFiller)
	toMain1(t, e)
	e.paidManaCensus = true
	bouncer := onBoardReady(t, e, 0, fpbBouncer)
	onBoardReady(t, e, 0, fpbScare)
	onBoardReady(t, e, 0, fpbIsland)
	for i := 0; i < mountains; i++ {
		onBoardReady(t, e, 0, fpbMountain)
	}
	ppPriority(t, e)
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Obj == bouncer {
			return pp
		}
	}
	t.Fatal("the bouncer's activation is not among the potential plays")
	return PotentialPlan{}
}

// CR 605.3a, 602.2b: the Signpost's repeatable fee-only filter makes the
// second {U} of {2}{U}{U} beside the one Island: two Mountains pay its {2} and
// two more the generic. The ask prune's free-tap-colour cut called the
// filter's {U} dominated by the Island and dropped it, though the Island
// fills only one of the two blue pips, so no prefix funded the activation
// (mercurial pretender's bounce, pool tail-45, seed 2 pair 220).
func TestPotentialScriptFeeFilterMakesSecondBluePip(t *testing.T) {
	t.Parallel()
	// Precondition: four Mountains pay the fee and the generic; three cannot.
	if pp := fpbPlay(t, 3); pp.Reason == "" && len(pp.Script) != 0 {
		t.Fatalf("precondition: three Mountains must not fund {2}{U}{U}, got script %+v", pp.Script)
	}
	pp := fpbPlay(t, 4)
	if pp.Reason != "" || len(pp.Script) == 0 {
		t.Fatalf("plan = reason %q detail %q script %d, want a prefix script through the Signpost's filter", pp.Reason, pp.Detail, len(pp.Script))
	}
	if pp.Action.Kind != "ability" {
		t.Fatalf("play kind = %q, want an ability", pp.Action.Kind)
	}
}
