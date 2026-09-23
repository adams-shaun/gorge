package rules

import (
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
)

// EmitSecretVoteFinished gives the synchronous trigger scan the privately
// answered ballot while recording only the ballot-free completion marker.
// A replay rebuilds this scratch from the recorded voter intents, not the log.
func (e *Engine) EmitSecretVoteFinished(public events.Event, ballots []effects.VoteBallot) {
	previous := e.secretVoteBallots
	e.secretVoteBallots = ballots
	defer func() { e.secretVoteBallots = previous }()
	e.emit(public)
}
