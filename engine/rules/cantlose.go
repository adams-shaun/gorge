// The "you can't lose the game / your opponents can't win the game" class
// (task fdn-repl-cant-lose): CR 104.3's GameLoss replacement and CR 104.2a's
// GameWin replacement, both printed as bodyless `R:Event$ GameLoss|GameWin |
// Layer$ CantHappen` lines (Herald of Eternal Dawn, Platinum Angel, Darksteel
// Angel, Abyssal Persecutor, Lich's Mastery, and the narrower ValidLoseReason$
// carriers). While such a replacement is live for a player, the state-based
// loss simply does not happen: the condition (0 or less life, ten poison, an
// empty-library draw, an "you lose the game" effect) stays true and the player
// loses the moment the replacement is gone and state-based actions are next
// checked (CR 704.5a, c). An attempted empty-library draw is the exception: it
// is a one-shot condition (CR 704.5b, "since the last time state-based actions
// were checked"), consumed by the check the replacement stopped.
//
// This file is the ONE gate every loss and alternate win funnels through:
//
//   - Engine.playerLoses / Engine.playerWins are the rules-side funnels the
//     state-based-action sweep, the opening-deal deck-out and every
//     effects-emitted loss/win go through.
//   - effects reaches the same funnel through the Host.EmitPlayerLost /
//     EmitGameWin methods (the effects package cannot import rules), so the
//     deck-out draw and api:LosesGame/api:WinsGame cannot bypass the gate.
//
// Matching runs through the ordinary replacement matcher (engine/rules/replacement.go
// exposes the GameLoss/GameWin cases) so ValidPlayer$, ValidLoseReason$,
// ActiveZones$ and the shared IsPresent$/CheckSVar$ condition gate cannot drift
// from every other replacement family. A concession is deliberately NOT routed
// through this gate: CR 104.3a's "a player may concede at any time" is not a
// loss a replacement can stop, so the concede option emits PlayerLost directly.

package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Forge's GameLossReason spellings, carried as the synthetic proposal's Text.
// Only LifeReachedZero and Milled have corpus ValidLoseReason$ carriers; the
// rest name the engine's own loss sites so a future narrow line can read them.
const (
	loseReasonLifeReachedZero = "LifeReachedZero"
	loseReasonPoisoned        = "Poisoned"
	loseReasonCommanderDamage = "CommanderDamage"
	loseReasonMilled          = "Milled"
	loseReasonEffect          = "Effect"
)

// playerLoses is the one funnel for a player losing the game. It reports
// whether the loss actually happened: a live GameLoss CantHappen replacement
// for p (and, when reason is non-empty, for that cause) swallows the event,
// exactly as Forge's replacement does, and the caller must not report the
// state change. The emitted event is the ordinary PlayerLost the log,
// events.Apply and replay already carry -- the gate changes only whether it is
// proposed, never its shape.
func (e *Engine) playerLoses(p state.PlayerID, reason, text string) bool {
	if int(p) < 0 || int(p) >= len(e.G.Players) || e.G.Players[p].Lost {
		return false
	}
	if e.gameLossPrevented(p, reason) {
		return false
	}
	e.losePlayer(p, text)
	return true
}

// losePlayer makes a loss nothing prevented.
func (e *Engine) losePlayer(p state.PlayerID, text string) {
	e.emit(events.Event{Kind: events.PlayerLost, Player: p, Text: text})
	e.initiativeHandoffOnDeparture(p)
}

// initiativeHandoffOnDeparture implements CR 726.4: "If the player who has
// the initiative leaves the game, the active player takes the initiative at
// the same time that player leaves the game. If the active player is leaving
// the game or if there is no active player, the next player in turn order
// takes the initiative." It runs immediately after the seat's PlayerLost has
// been emitted (so the fold has already marked it Lost) from the ONE loss
// gate below; the concede option calls it too, because CR 104.3a's
// "a player may concede at any time" deliberately bypasses this gate while
// still being a way a player leaves the game.
//
// The new holder "takes the initiative" -- the same action CR 726.2's
// inherent ability listens for -- so the handoff also queues the CR 726.2
// venture for the new holder, exactly as the combat-damage take does
// (engine/rules/combat.go). CR 726.5 confirms the reading: a designation move by an
// already-holder still causes that trigger. When no other seat is alive the
// handoff is skipped rather than emitting a designation naming a departed
// seat. No game-state field is written here: the single-holder move is the
// ordinary InitiativeChange fold (CR 726.3).
func (e *Engine) initiativeHandoffOnDeparture(p state.PlayerID) {
	if e.G.Over || !e.G.HasInitiative || e.G.Initiative != p {
		return
	}
	next := e.G.Active
	if !(int(next) >= 0 && int(next) < len(e.G.Players) && next != p && !e.G.Players[next].Lost) {
		next = e.G.NextAlive(p) // next in turn order that is still in the game
	}
	if next == p || e.G.Players[next].Lost {
		// No other seat can take the designation; leave it unset rather
		// than hand it to a seat that has left the game.
		return
	}
	e.emit(events.Event{Kind: events.InitiativeChange, Player: next})
	e.pendingTriggers = append(e.pendingTriggers, pendingTrigger{
		Controller: next, InitiativeVenture: true})
}

// playerWins is the one funnel for an alternate win ("you win the game", the
// api:WinsGame family, CR 104.2b). A live GameWin CantHappen replacement for p
// swallows the GameOver event and reports false. The CR 104.2a
// last-player-standing win checkGameOver grants is NOT routed here: it
// overrides every effect that would preclude the win.
func (e *Engine) playerWins(p state.PlayerID, text string) bool {
	if int(p) < 0 || int(p) >= len(e.G.Players) || e.G.Players[p].Lost || e.G.Over {
		return false
	}
	if e.gameWinPrevented(p) {
		return false
	}
	e.emit(events.Event{Kind: events.GameOver, Player: p, Text: text})
	return true
}

// markDrawEmpty records p's attempted draw from an empty library (CR 121.4).
// The loss is CR 704.5b's, emitted by the next state-based check; a second
// attempt before that check adds nothing.
func (e *Engine) markDrawEmpty(p state.PlayerID) {
	if int(p) < 0 || int(p) >= len(e.G.Players) || e.G.Players[p].Lost || e.G.Players[p].DrewFromEmpty {
		return
	}
	e.emit(events.Event{Kind: events.DrawEmpty, Player: p})
}

// EmitPlayerLost implements effects.Host: effects reaches the one loss gate
// through it (the effects package cannot import rules).
func (e *Engine) EmitPlayerLost(p state.PlayerID, reason, text string) {
	e.playerLoses(p, reason, text)
}

// EmitGameWin implements effects.Host: effects reaches the one alternate-win
// gate through it.
func (e *Engine) EmitGameWin(p state.PlayerID, text string) {
	e.playerWins(p, text)
}

// gameLossPrevented reports whether a live R:Event$ GameLoss | Layer$
// CantHappen replacement would stop p from losing the game for reason. reason
// is the Forge GameLossReason spelling (empty means "any cause").
func (e *Engine) gameLossPrevented(p state.PlayerID, reason string) bool {
	return e.gameEventCantHappen("GameLoss", events.Event{
		Kind: events.PlayerLost, Player: p, Text: reason})
}

// gameWinPrevented reports whether a live R:Event$ GameWin | Layer$ CantHappen
// replacement would stop p from winning the game.
func (e *Engine) gameWinPrevented(p state.PlayerID) bool {
	return e.gameEventCantHappen("GameWin", events.Event{
		Kind: events.GameOver, Player: p})
}

// gameEventCantHappen is gameLossPrevented/gameWinPrevented's shared scan: it
// reports whether ANY live bodyless Layer$ CantHappen replacement named name
// matches the synthetic proposal ev (which carries the affected player and,
// for a loss, the reason). The shape mirrors turnFaceUpCantHappen: an
// Effect-created registration is scanned through active() (its lifetime is the
// effect's, and the active-zone gate is deliberately skipped), and every
// printed face through the deterministic forEachReplacementSource walk. The
// matching itself is the ordinary replacementMatches path, so ValidPlayer$,
// ValidLoseReason$, ActiveZones$ and the condition gate all read exactly as
// they do for every other family.
func (e *Engine) gameEventCantHappen(name string, ev events.Event) bool {
	// Effect-created replacements (a delayed "you can't lose this turn"):
	// bodyless, Layer$ CantHappen, matching through the same matcher.
	for ceI, ceL := 0, e.active(); ceI < len(ceL); ceI++ {
		ce := &ceL[ceI]
		if ce.ReplacementEvent != name || ce.ReplacementBody != "" ||
			!strings.EqualFold(strings.TrimSpace(ce.ReplacementParam(cards.PKLayer)), "CantHappen") {
			continue
		}
		r := &cards.Repl{Event: ce.ReplacementEvent, Params: ce.ReplacementParams}
		if e.replacementMatchesEffectCreated(*r, ce.Source, ev, ce.Remembered, ce.RememberedPlayers) {
			return true
		}
	}
	blocked := false
	e.forEachReplacementSource(func(source state.ObjID) {
		if blocked {
			return
		}
		f := e.replacementFace(source, ev)
		if f == nil {
			return
		}
		for i := range f.Repls {
			r := &f.Repls[i]
			if r.Event != name || r.With != nil {
				continue
			}
			if !strings.EqualFold(strings.TrimSpace(r.ParamStr(cards.PKLayer)), "CantHappen") {
				continue
			}
			if e.replacementMatches(*r, source, ev) {
				blocked = true
				return
			}
		}
	})
	return blocked
}
