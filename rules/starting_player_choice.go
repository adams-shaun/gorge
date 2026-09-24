package rules

import (
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// CR 103.1's second half: the winner of the pre-game toss chooses who takes
// the first turn. rules.NewStartingPlayerChoice draws the toss (one rng IntN,
// before any shuffle), announces it, folds the RESOLVED toss into genesis
// unconditionally (G.StartingPlayer is the toss winner from the moment the
// constructor returns -- the same state the pre-choice engine left), and --
// once the opening deal has fixed the survivors -- leaves the CHOICE
// available (tossChoice.active), with only startPostDealSetup deferred so
// the pregame rounds open in the CHOSEN seat's turn order. A harness that
// can answer calls AskStartingPlayer; any caller that poses no ask and
// advances gets the deterministic R-9 fallback at Advance's head: the toss
// winner takes the first turn, the pre-choice seat. Plain rules.New never
// offers the ask at all -- it is the R-9 no-host fallback, byte-identical to
// the pre-choice engine.
//
// The answer is the authoritative starting seat: handleStartingPlayer folds
// StartingPlayerChange (a re-fold of the same no-append genesis Apply when
// the winner names themselves) and opens the pregame rounds exactly as the
// pre-choice genesis did, and beginTurn records the chosen seat in its
// ordinary TurnChange.
//
// The state is plain data (tossChoice), never a closure, so Engine.Clone and
// replay reproduce the pending choice. A game with no chooser (a terminal
// deal, a sole survivor, or a toss winner the deal eliminated) never sets it
// and is byte-identical to the pre-choice engine.

// tossChoice is the CR 103.1 choice's plain-value state. active is true from
// the end of New's deal until the choice is posed-and-answered or defaulted;
// winner is the toss winner -- which is ALSO G.StartingPlayer for the whole
// pending window, because New folds the resolved toss unconditionally. The
// candidate seats travel in the Decision's own option list (Option.Player),
// so replay rebuilds them from the live survivors without a second copy here.
type tossChoice struct {
	active bool
	winner state.PlayerID
}

// AskStartingPlayer poses the CR 103.1 winner-chooses decision when one is
// available and returns it, or nil when there is nothing to choose (a
// terminal deal, a sole survivor, an eliminated winner, a choice already
// resolved, or an engine no longer at genesis). It is the "host conveyance":
// a harness that can answer a seat calls it immediately after New and before
// Advance, then parks the returned decision for its winner. A harness that
// does not call it never sees the ask; Advance applies the deterministic
// fallback instead. The genesis guard (G.Turn == 0) keeps a hand-built
// mid-game engine that happens to carry an unoffered choice from opening the
// pregame rounds mid-scenario: the choice belongs to the moment before turn
// one, and step() and its callers must never resolve it.
func (e *Engine) AskStartingPlayer() *decision.Decision {
	if e == nil || !e.tossChoice.active || e.G.Over || e.G.Turn != 0 || e.pending != nil {
		return nil
	}
	alive := e.G.AliveFrom(0)
	if len(alive) < 2 {
		return nil
	}
	opts := make([]decision.Option, len(alive))
	for i, p := range alive {
		opts[i] = decision.Option{Index: i, Kind: "player",
			Label: seatFacingName(e.G, p), Player: p}
	}
	e.ask(decision.New(e.tossChoice.winner, decision.KStartingPlayer,
		"You won the toss. Choose who takes the first turn.", 1, 1, opts))
	return e.pending
}

// resolveStartingPlayer resolves the CR 103.1 choice to the given seat and
// opens the pregame rounds. It is the shared tail of the default and the
// answered path, and the ONE place startPostDealSetup runs for a game whose
// choice was ever pending -- the London mulligan round (and the CR 903.4b
// colour round before it) opens only now, in the CHOSEN seat's turn order.
// The fold goes through events.Apply without appending a new event and only
// when the seat differs from genesis's folded toss winner, so a self-choice
// leaves the stream exactly as the pre-choice engine wrote it; beginTurn
// records the chosen seat in its ordinary TurnChange.
func (e *Engine) resolveStartingPlayer(start state.PlayerID) {
	e.tossChoice = tossChoice{}
	if start != e.G.StartingPlayer {
		events.Apply(e.G, events.Event{Kind: events.StartingPlayerChange, Player: start})
	}
	if !e.G.Over {
		e.startPostDealSetup()
	}
}

// resolveTossChoiceDefault applies the deterministic R-9 fallback for a
// choice no host answered: the toss winner takes the first turn. It runs
// from Advance's loop head (rules/engine.go) -- never from step(), which the
// resolution machinery calls mid-game -- and only on an engine still at
// genesis (G.Turn == 0, i.e. no beginTurn has run): a hand-built mid-game
// engine that happens to carry the unoffered choice must never find the
// pregame rounds opening underneath it.
func (e *Engine) resolveTossChoiceDefault() {
	if !e.tossChoice.active || e.G.Turn != 0 {
		return
	}
	e.resolveStartingPlayer(e.tossChoice.winner)
}

// handleStartingPlayer applies the answered CR 103.1 choice. The chosen
// option's Player field names the starting seat; the offered option list
// already holds only living seats, so Validate has checked it. A decision
// that somehow carried nothing falls back to the toss winner (the same R-9
// default).
func (e *Engine) handleStartingPlayer(d *decision.Decision, in decision.Intent) {
	start := e.tossChoice.winner
	if chosen := d.Chosen(in); len(chosen) == 1 {
		start = chosen[0].Player
	}
	e.resolveStartingPlayer(start)
}
