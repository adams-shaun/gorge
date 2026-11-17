package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Harsh Deceiver's {1} ability is AB$ PeekAndReveal | Cost$ 1 | NoReveal$ True
// — look at the top card of your library, no reveal ask (CR 701.20e: the look
// needs a card; with the library empty the activation stays legal and resolves
// into nothing — no look_ack ask, no note, on either engine). Codsworth, Handy
// Helper's {T}: Add {W}{W} restricted to Aura and Equipment spells sits beside
// it, so a priority's menu carries both a restricted-mana producer and the peek.
//
// Brink of Disaster is the payment-plan half: an Aura with `Mode$ Taps |
// ValidCard$ Card.AttachedBy | DB$ Destroy | Defined$ TriggeredCardLKICopy`
// on a Scrubland in the peek's mana base. Before the payment planner vetted
// that shape it deferred the watched land ("source:interference:BrinkD"), so
// an autopay priority whose payment needed that land lost the cast or
// activation while the kernel priced it. The destroy is a post-payment
// consequence, not interference: the mana is produced at the tap (CR 605.4a)
// and the trigger resolves after the payment that caused it (CR 405.6c/603.2),
// touching no payment resource.

// khpeekDecks: a forty-card white deck — Harsh Deceiver is cast and activated,
// Brink of Disaster sits on one of the sixteen Scrublands in the peek's mana
// base, and the library runs dry a few turns later, so the peek's empty-window
// activation is reached too. The green side attacks into it harmlessly (a 1/4
// blocks a 2/2).
var khpeekDecks = [2][]cardset.Entry{
	{
		{Name: "Harsh Deceiver", Count: 4}, {Name: "Codsworth, Handy Helper", Count: 4},
		{Name: "Brink of Disaster", Count: 4}, {Name: "Scrubland", Count: 14},
		{Name: "Plains", Count: 14},
	},
	{
		{Name: "Grizzly Bears", Count: 4}, {Name: "Forest", Count: 36},
	},
}

// TestMBCHarshDeceiverPeekLockstep replays the peek-activation lockstep with a
// destroy-self Taps watcher on a land in the peek's mana base (CR 605.4a/603.2
// and CR 701.20e).
func TestMBCHarshDeceiverPeekLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "hpeek", khpeekDecks, 40, "HPEEK_SEEDS")
}
