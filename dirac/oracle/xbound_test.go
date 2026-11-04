package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// An offer cannot read an X not yet announced, so an activation or a spell is
// offered while some object fits the rest of the target's description (CR 601.2c,
// 602.2b), and an announced X that leaves no target reverses it (CR 733.1).
func TestXBoundTargetOffer(t *testing.T) {
	if isolate(t) {
		return
	}
	summoner := []cardset.Entry{
		{Name: "Rydia, Summoner of Mist", Count: 12}, {Name: "Summon: Titan", Count: 10},
		{Name: "Mountain", Count: 9}, {Name: "Forest", Count: 9},
	}
	lockstepRun(t, "xbound", [2][]cardset.Entry{summoner, summoner}, 80, true, nil)
	bounce := []cardset.Entry{
		{Name: "Repeal", Count: 12}, {Name: "Grizzly Bears", Count: 10},
		{Name: "Island", Count: 9}, {Name: "Forest", Count: 9},
	}
	lockstepRun(t, "xbound", [2][]cardset.Entry{bounce, bounce}, 80, true, nil)
}

// The announcement must both pay and leave a target (CR 601.2b, 601.2c, 601.2h), so
// a target only an X out of reach could name withholds the offer: lands kept short
// against a Saga in the graveyard and against a dear permanent to bounce.
func TestXBoundOfferNeedsPayableX(t *testing.T) {
	if isolate(t) {
		return
	}
	summoner := []cardset.Entry{
		{Name: "Rydia, Summoner of Mist", Count: 14}, {Name: "Summon: Titan", Count: 14},
		{Name: "Mountain", Count: 6}, {Name: "Forest", Count: 6},
	}
	discard := []cardset.Entry{{Name: "Mind Rot", Count: 16}, {Name: "Swamp", Count: 24}}
	lockstepRun(t, "xbound", [2][]cardset.Entry{summoner, discard}, 80, true, nil)
	bounce := []cardset.Entry{{Name: "Repeal", Count: 28}, {Name: "Island", Count: 12}}
	giants := []cardset.Entry{{Name: "Hill Giant", Count: 20}, {Name: "Mountain", Count: 20}}
	lockstepRun(t, "xbound", [2][]cardset.Entry{bounce, giants}, 80, true, nil)
}

// An offer reads a target's bound for whoever would cast it: Squirming Emergence
// counts the permanent cards in its caster's graveyard, not the first player's.
func TestOfferReadsItsCaster(t *testing.T) {
	if isolate(t) {
		return
	}
	descent := []cardset.Entry{
		{Name: "Squirming Emergence", Count: 12}, {Name: "Hill Giant", Count: 8},
		{Name: "Swamp", Count: 10}, {Name: "Forest", Count: 10},
	}
	discard := []cardset.Entry{{Name: "Mind Rot", Count: 16}, {Name: "Swamp", Count: 24}}
	for _, decks := range [][2][]cardset.Entry{{descent, discard}, {discard, descent}} {
		lockstepRun(t, "xbound", decks, 40, true, nil)
	}
}
