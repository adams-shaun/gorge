// Package rules: state-based actions (CR 704) and game end (CR 104.4).
//
// checkStateBased, checkGameOver and destroyLethalDamage all predate this
// file: Task 21 added destroyLethalDamage (combat depends on lethal-damage
// and zero-toughness destruction, and nothing in this codebase checked
// either before then), and stubs.go carried checkStateBased/checkGameOver
// as a placeholder ever since Task 3 -- a checkGameOver call and nothing
// else. This task deletes stubs.go outright and relocates all three here
// rather than losing them with it, and fills in the state-based actions M1
// actually specifies: a player at 0 or less life loses (CR 704.5a); an
// eliminated player's permanents leave the battlefield; the game ends in a
// win when exactly one seat remains and a draw (CR 104.4a) when none do --
// not, as stubs.go's own checkGameOver used to, an unconditional win for
// seat 0 when nobody survived. Drawing from an empty library (CR 704.5c) is
// already a loss as of Task 14 (effects.DrawFor emits PlayerLost directly at
// the moment of the failed draw, which checkStateBased's own permanent-
// removal pass below picks up on its next run regardless of how a player
// came to be Lost); this build has no poison-counter mechanic, so CR
// 704.5b/704.5h(poison) is not modelled and not a gap this task needs to
// close.
package rules

import (
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// maxSBAPasses bounds the fixed-point loop below. A game that needs more
// passes than this has a rules bug (a static-effect cycle where destroying
// one creature always makes another lethal, forever), and reporting a stuck
// game via a bounded loop is better than hanging the one goroutine running
// the whole match.
const maxSBAPasses = 32

// checkStateBased applies state-based actions until none apply, which is
// what CR 704.3 requires: they are checked and rechecked, not just once.
// TestSBALoopsUntilStable is the regression test for the "not just once"
// half -- a lord dying can make a creature its own static was keeping alive
// newly lethal, and that second creature must not survive to see a second,
// separate checkStateBased call before it is caught.
func (e *Engine) checkStateBased() {
	for pass := 0; pass < maxSBAPasses; pass++ {
		changed := e.checkLoseConditions()
		if e.destroyLethalDamage() {
			changed = true
		}
		if !changed {
			break
		}
	}
	e.checkGameOver()
}

// checkLoseConditions applies CR 704.5a (life 0 or less) to every player not
// already marked Lost, in fixed seat order (Players is a slice walked by
// index, never a map, so simultaneous eliminations always emit in the same
// order run to run). It then separately sweeps every Lost player -- from
// life loss just above, from an empty-library draw (effects.DrawFor, which
// emits PlayerLost directly, independent of this loop, at the moment the
// draw fails), or from any future cause -- and removes whatever they still
// control from the battlefield, which is CR 800.4a's own cleanup for a
// player who has left the game. Doing this generically, keyed only on
// p.Lost rather than on which branch just set it, is what makes a decked-out
// player's board get cleaned up exactly the same way a player who hit 0
// life does, through the one path, rather than needing its own copy of the
// same logic.
//
// Reports whether anything changed, which is what checkStateBased's own
// pass loop uses to decide whether to run again.
func (e *Engine) checkLoseConditions() bool {
	changed := false
	for i := range e.G.Players {
		p := &e.G.Players[i]
		if !p.Lost && p.Life <= 0 {
			e.emit(events.Event{Kind: events.PlayerLost, Player: p.ID, Text: "life total is 0 or less"})
			changed = true
		}
	}
	for i := range e.G.Players {
		p := &e.G.Players[i]
		if p.Lost && len(e.G.Zone(state.ZBattlefield, p.ID)) > 0 {
			e.removePermanents(p.ID)
			changed = true
		}
	}
	return changed
}

// removePermanents moves an eliminated player's permanents out of the game.
// Exile is the closest zone this build has to CR 800.4a's "leaves the
// game" -- the same approximation resolveTop already makes for a resolved
// ability with no printed card (CR 608.2m).
func (e *Engine) removePermanents(p state.PlayerID) {
	ids := append([]state.ObjID(nil), e.G.Zone(state.ZBattlefield, p)...)
	for _, id := range ids {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id,
			From: state.ZBattlefield, To: state.ZExile, Text: "controller left the game"})
	}
}

// destroyLethalDamage performs the two CR 704.5 state-based actions combat
// damage depends on: a creature with toughness 0 or less (704.5f) is
// destroyed regardless of damage or Indestructible, and a creature with
// damage marked on it greater than or equal to its toughness, or with any
// damage at all from a source with Deathtouch (704.5g, via the Deathtouched
// counter damageStep marks), is destroyed unless Indestructible.
//
// This is general state-based-action logic, not combat-specific -- a
// creature killed by an ordinary DealDamage spell dies here exactly the same
// way one killed in combat does -- which is why it lives here rather than in
// combat.go, where Task 21 first added it.
//
// The Indestructible check reads the same way effDestroy/effDestroyAll
// (effects/zone.go) already do; it is not one of the eight keywords Task 21
// registers as implemented, so a card that actually carries it still reads
// as unsupported for deck-routing purposes, but a creature that is
// Indestructible for some other already-working reason should not
// spuriously die to lethal damage.
//
// Every candidate is found in one pass before any of them actually leaves
// (dead is collected, then destroyed), so one creature's departure this same
// pass can never retroactively change whether another one here counts as
// lethal, matching CR 704.3's "state-based actions are performed
// simultaneously" for one round of this check. Reports whether it destroyed
// anything, which is what checkStateBased's pass loop uses to decide
// whether a further pass -- one where a just-removed creature's own
// continuous effects have now actually expired -- might find something new.
func (e *Engine) destroyLethalDamage() bool {
	var dead []state.ObjID
	for _, p := range e.G.AliveFrom(0) {
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			o := e.G.Obj(id)
			if o == nil {
				continue
			}
			f := o.Face()
			if f == nil || !f.IsCreature() {
				continue
			}
			if e.Toughness(id) <= 0 {
				dead = append(dead, id)
				continue
			}
			if o.Damage <= 0 {
				continue
			}
			if o.Damage < e.Toughness(id) && o.Counter("Deathtouched") == 0 {
				continue
			}
			if e.HasKeyword(id, "Indestructible") {
				continue
			}
			dead = append(dead, id)
		}
	}
	for _, id := range dead {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id,
			From: state.ZBattlefield, To: state.ZGraveyard, Text: "destroyed"})
	}
	return len(dead) > 0
}

// checkGameOver ends the game once at most one seat remains (CR 104.2a's
// "the last player left in the game wins", CR 104.4a's draw when nobody is).
// Ruling T22-b: this used to default the winner to seat 0 whenever nobody
// survived -- PlayerID's zero value is a real seat, so an unconditional
// `w := state.PlayerID(0)` silently crowned it every time elimination was
// simultaneous. There is no winner in that case; Amount: 1 tells Apply's
// GameOver case to record a draw (Game.Draw) instead of a winner.
func (e *Engine) checkGameOver() {
	if e.G.Over {
		return
	}
	alive := e.G.AliveFrom(0)
	if len(alive) > 1 {
		return
	}
	if len(alive) == 1 {
		w := alive[0]
		e.emit(events.Event{Kind: events.GameOver, Player: w, Text: e.G.Players[w].Name})
	} else {
		e.emit(events.Event{Kind: events.GameOver, Amount: 1, Text: "draw"})
	}
	e.pending = nil
}
