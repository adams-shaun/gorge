package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// A pair filter: pay one and tap for one white and one blue at once.
const pairFilterFixtureSrc = `Name:Fixture Pair Filter
ManaCost:no cost
Types:Land
A:AB$ Mana | Cost$ 1 T | Produced$ W U | SpellDescription$ Pay one, tap: white and blue.
Oracle:{1}, {T}: Add {W}{U}.
`

// A land making one mana of the commander's identity: nothing without one.
const identityLandFixtureSrc = `Name:Fixture Identity Land
ManaCost:no cost
Types:Land
A:AB$ Mana | Cost$ T | Produced$ Combo ColorIdentity | SpellDescription$ One mana of your commander's colours.
Oracle:{T}: Add one mana of any color in your commander's color identity.
`

// An artifact making one mana for each of its charge counters.
const counterRockFixtureSrc = `Name:Fixture Counter Rock
ManaCost:2
Types:Artifact
A:AB$ Mana | Cost$ T | Produced$ Any | Amount$ Stored | SpellDescription$ One mana of a colour per charge counter.
SVar:Stored:Count$CardCounters.CHARGE
Oracle:{T}: Add one mana of any one color for each charge counter on this artifact.
`

const plainsFixtureSrc = "Name:Fixture Plains\nManaCost:no cost\nTypes:Basic Land Plains\nOracle:x\n"

const whiteThreeDropFixtureSrc = "Name:Fixture White Three\nManaCost:2 W\nTypes:Creature Cat\nPT:3/2\nOracle:x\n"

// TestSourceMakingNothingLeavesTheCensusComplete: {2}{W} from an Island, a
// Plains and a pair filter is payable (the Island funds the filter's {1}). A
// source that makes nothing -- an identity land without a commander (CR
// 903.4f), a counter rock with no counters -- beside them is no census gap,
// so the census still funds the filter's fee and the cast has a prefix.
func TestSourceMakingNothingLeavesTheCensusComplete(t *testing.T) {
	t.Parallel()
	for _, extra := range []string{identityLandFixtureSrc, counterRockFixtureSrc} {
		e, _, three := newFixtureDeck(t, 6305, whiteThreeDropFixtureSrc)
		onBoard(t, e, 0, pairFilterFixtureSrc)
		onBoard(t, e, 0, islandFixtureSrc)
		onBoard(t, e, 0, extra)
		onBoard(t, e, 0, plainsFixtureSrc)
		driveToStep(t, e, 1, 0, state.StepMain1)
		found := false
		for _, pp := range e.PotentialPaymentPlans(0) {
			if pp.Action.Kind != "cast" || pp.Action.Obj != three {
				continue
			}
			found = true
			if pp.Reason != "" || len(pp.Script) == 0 {
				t.Errorf("%s beside the filter: verdict %q %q, script %d steps; want a funded prefix",
					card(t, extra).Faces[0].Name, pp.Reason, pp.Detail, len(pp.Script))
			}
		}
		if !found {
			t.Fatalf("the {2}{W} cast is not a potential play")
		}
	}
}
