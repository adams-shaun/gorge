package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// electionRevealPrefix is the Note gorge's effChooseNumberElection emits once
// every chooser has answered a Secretly$ election.
const electionRevealPrefix = "ChooseNumber election revealed: "

// safeElectionMatchedAndMissed: over the whole game an Expert-Level Safe
// election both matched (the reveal names two equal picks and a card then
// went from exile to its owner's hand: the opened safe) and missed (two
// different picks, after which the controller's top card went to exile).
func safeElectionMatchedAndMissed(g *GorgeSide) bool {
	matched, missed := false, false
	pending := 0 // 1 after a matched reveal, 2 after a missed one
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		switch {
		case e.Kind == events.Note && strings.HasPrefix(e.Text, electionRevealPrefix):
			picks := strings.Split(strings.TrimPrefix(e.Text, electionRevealPrefix), ", ")
			pending = 2
			if len(picks) == 2 && picks[0] == picks[1] {
				pending = 1
			}
		case e.Kind == events.MoveZone && pending == 1 && e.From == state.ZExile && e.To == state.ZHand:
			matched, pending = true, 0
		case e.Kind == events.MoveZone && pending == 2 && e.From == state.ZLibrary && e.To == state.ZExile:
			missed, pending = true, 0
		}
	}
	return matched && missed
}

// TestHostedCmdrSecretElectionLockstep plays Expert-Level Safe's secret
// ChooseNumber election decision by decision against gorge: seat 0 casts and
// activates Safes, targeting seat 1; both seats answer the 1-2-3 election
// from the seed's random stream. The case FAILS unless some seed sees an
// election both match (the Safe opened) and miss (another card filed).
func TestHostedCmdrSecretElectionLockstep(t *testing.T) {
	safeDeck := append(deckOf(12, "Expert-Level Safe"), deckOf(28, "Swamp")...)
	oppDeck := append(deckOf(6, "Grizzly Bears"), deckOf(34, "Forest")...)
	runHostedCmdrShape(t, hostedCmdrShape{
		hostedCmdrCase: hostedCmdrCase{
			name:    "expert_level_safe",
			carrier: "Expert-Level Safe",
			seats:   [2]string{"hcs-safe-a", "hcs-safe-b"},
			decks:   [2][]string{safeDeck, oppDeck},
		},
		reached: safeElectionMatchedAndMissed,
	})
}
