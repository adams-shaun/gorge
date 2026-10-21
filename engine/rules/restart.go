package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// restart.go is CR 727, restarting the game (Karn Liberated's [-14]).
//
// effRestartGame (engine/effects/game.go) calls RequestRestart and stops its
// chain. The resolving caller (resolveTop's ability and spell arms,
// resumeResolution) sees e.restart once effects.Resolve returns and calls
// finishRestart instead of finishing the resolution, the api:EndTurn
// finishEndTurn shape: the GameRestart event folds the board into the new
// game's (state.Game.Restart), resetForRestart returns every per-game engine
// field to its genesis value, and the new game is dealt and set up exactly
// as genesis does (commanders to the command zone, shuffle, seven cards, the
// pregame rounds, turn 1) with the restarting player as its starting player
// (CR 727.1a).

// restartRequest is a restart effRestartGame asked for and the resolving
// caller has not yet performed.
type restartRequest struct {
	// player controls the restarting effect: the new game's starting player.
	player state.PlayerID
	// keep is the CR 727.5 exempted set, left where it is.
	keep []state.ObjID
}

// RequestRestart implements effects' restartHost: it records the restart for
// the resolving caller. A second request in the same resolution (none in the
// corpus) keeps the first. rest, the chain's remainder CR 727.4 runs just
// before the new game's first untap step, is not yet followed (cr-g33b):
// Karn's exempted cards stay in exile.
func (e *Engine) RequestRestart(c *effects.Ctx, keep []state.ObjID, rest *cards.SA) {
	if e.restart != nil || c == nil {
		return
	}
	e.restart = &restartRequest{player: c.Controller, keep: append([]state.ObjID(nil), keep...)}
}

// finishRestart performs the pending restart (see the file comment).
func (e *Engine) finishRestart() {
	req := e.restart
	e.restart = nil
	e.emit(events.Event{Kind: events.GameRestart, Player: req.player, Amount: e.startingLife, IDs: req.keep})
	e.resetForRestart()
	// The opening deal is one genesis operation, exactly as New's: a loss
	// from an empty library is real immediately, but the terminal GameOver
	// waits until every remaining short seat has been accounted for, so an
	// all-undersized restart reaches CR 104.4a's draw instead of crowning an
	// undealt short deck (T22-c).
	e.deferGameOver = true
	alive := e.G.AliveFrom(0)
	for i, p := range alive {
		pl := &e.G.Players[p]
		// The commanders begin the new game in the command zone (CR 903.6),
		// moved before the shuffle exactly as genesis moves them. A commander
		// the restart exempted (CR 727.5) keeps the zone it stayed in, so only
		// one that Restart put in the library moves.
		for _, id := range pl.Commanders {
			if o := e.G.Obj(id); o != nil && o.Zone == state.ZLibrary {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZCommand})
			}
		}
		order := append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, p)...)
		order = e.ShuffleLibrary(p, order)
		e.emit(events.Event{Kind: events.Shuffle, Player: p, IDs: order, Secret: true})
		for j := 0; j < openingHand; j++ {
			e.drawCard(p)
		}
		if e.G.AliveCount() <= 1 {
			// Genesis's T22-c boundary: once at most one seat remains, no
			// later hand is dealt. Account for any later seat whose library
			// could not supply seven cards, then finish the terminal deal.
			for _, next := range alive[i+1:] {
				available := len(e.G.Zone(state.ZLibrary, next))
				for _, id := range e.G.Players[next].Commanders {
					if o := e.G.Obj(id); o != nil && o.Zone == state.ZLibrary {
						available--
					}
				}
				if available < openingHand && !e.G.Players[next].Lost {
					e.playerLoses(next, loseReasonMilled, "drew from an empty library")
				}
			}
			if e.finishTerminalGenesis() {
				return
			}
		}
	}
	if e.finishTerminalGenesis() {
		return
	}
	e.deferGameOver = false
	e.emit(events.Event{Kind: events.StartingPlayerChange, Player: req.player})
	e.startPostDealSetup()
}

// resetForRestart returns every per-game engine field to its zero (genesis)
// value, keeping only the match's configuration and host wiring: the board
// and log (already folded by GameRestart), the rng stream, the compiled card
// text and deck manifests, the format and Config knobs newEngineShell copied,
// the host hooks and the decision arena. Every field NOT named here is a
// per-game field by construction; TestCR727ResetForRestartKeepsOnlyConfig
// pins the kept set.
func (e *Engine) resetForRestart() {
	keep := *e
	*e = Engine{}
	e.G, e.L, e.rng = keep.G, keep.L, keep.rng
	e.deckManifests, e.compiledText, e.landTypeWords = keep.deckManifests, keep.compiledText, keep.landTypeWords
	e.ManaAbilityHook = keep.ManaAbilityHook
	e.format = keep.format
	e.mulligans, e.windowDiagnostics, e.paidManaCensus = keep.mulligans, keep.windowDiagnostics, keep.paidManaCensus
	e.startingLife = keep.startingLife
	e.setNameInPool, e.layer4InPool, e.controlStaticInPool = keep.setNameInPool, keep.layer4InPool, keep.controlStaticInPool
	e.skipPass = keep.skipPass
	e.loop = keep.loop
	e.tapeHeldHook, e.tapeHeldStats = keep.tapeHeldHook, keep.tapeHeldStats
	e.PaymentStats = keep.PaymentStats
	e.decArena = keep.decArena
	// newEngineShell's per-seat ledgers and proof flags. The event log is
	// carried across the restart, so the turn ledgers start counting at the
	// current log length: the new game's turn 1 is the first TurnChange after
	// it, and the old game's turns are not folded into Count$YourTurns or the
	// delayed next-turn boundary.
	e.turnLedgerBase = len(e.L.Events)
	e.turnsTaken = make([]int32, len(e.G.Players))
	e.turnsTakenEpoch = e.turnLedgerBase
	e.turnStartTurns = make([][]int32, len(e.G.Players))
	e.turnStartEpoch = e.turnLedgerBase
	e.manaExpended = make([]int32, len(e.G.Players))
	e.manaExpendedTurn = e.G.Turn
	e.trigGrant.free = true
	e.trigGrant.replFree = true
	e.staticEpoch, e.activeEpoch = -1, -1
}
