package events

import "github.com/adams-shaun/gorge/engine/state"

// apply_turnwatch.go holds the per-seat turn-watcher tallies on state.Player
// (SpellsCastThisTurn, LifeGainedThisTurn, LifeLostThisTurn,
// TokensCreatedThisTurn). They are written only here and from the folds that
// already own the underlying state change (foldLifeChange, foldDamage, the
// token mints), and cleared in foldTurnChange, so a log-only replay derives
// them and no direct state write exists outside Apply.

// resetTurnWatchers clears one seat's watchers at TurnChange.
func resetTurnWatchers(p *state.Player) {
	p.SpellsCastThisTurn = 0
	p.LifeGainedThisTurn = 0
	p.LifeLostThisTurn = 0
	p.TokensCreatedThisTurn = 0
	p.AttackedPlayersThisTurn = 0
	p.AttackedPlaneswalkersThisTurn = 0
}

// tallyAttack records, for the controller of each declared attacker, that it
// attacked defending seat e.Player this turn: the player itself (e.Obj zero)
// or a planeswalker that seat controls (e.Obj a planeswalker). A battle
// attack (CR 310.7) is neither and records nothing.
func tallyAttack(g *state.Game, e *Event) {
	if !validPlayer(g, e.Player) || e.Player < 0 || e.Player >= 64 {
		return
	}
	bit := uint64(1) << uint(e.Player)
	walker := false
	if e.Obj != 0 {
		target := g.Obj(e.Obj)
		if target == nil || target.Face() == nil || !target.Face().IsPlaneswalker() {
			return
		}
		walker = true
	}
	for _, id := range e.IDs {
		o := g.Obj(id)
		if o == nil || !validPlayer(g, o.Controller) {
			continue
		}
		if walker {
			g.Players[o.Controller].AttackedPlaneswalkersThisTurn |= bit
		} else {
			g.Players[o.Controller].AttackedPlayersThisTurn |= bit
		}
	}
}

// tallySpellCast counts one spell put on the stack by e.Player (the same
// PutOnStack classification Engine.SpellsCastThisTurnBy scans the log for).
func tallySpellCast(g *state.Game, e *Event) {
	if validPlayer(g, e.Player) {
		g.Players[e.Player].SpellsCastThisTurn++
	}
}

// tallyTokenCreated counts one token minted under p's control by a
// TokenCreate, CardToken, CopyToken or MyriadCopy fold.
func tallyTokenCreated(g *state.Game, p state.PlayerID) {
	if validPlayer(g, p) {
		g.Players[p].TokensCreatedThisTurn++
	}
}

// foldSpellCastReversed undoes one PutOnStack tally for a CR 733.1 reversed
// cast (rules' abortCast). The guard at zero keeps a log that reverses
// without a preceding cast this turn from underflowing the counter.
func foldSpellCastReversed(g *state.Game, e *Event) {
	if validPlayer(g, e.Player) && g.Players[e.Player].SpellsCastThisTurn > 0 {
		g.Players[e.Player].SpellsCastThisTurn--
	}
}

// foldActivationReversed undoes one activation-census tally for a CR 733.1
// reversed activated ability (rules' abortCast). The mint increments
// Object.ActivatedThisTurn only while its source is on the battlefield, so
// the reversal decrements under the same condition; the guard at zero keeps
// a log that reverses without a preceding mint from underflowing the count.
func foldActivationReversed(g *state.Game, e *Event) {
	if o := g.Obj(e.Obj); o != nil && o.Zone == state.ZBattlefield && o.ActivatedThisTurn > 0 {
		o.ActivatedThisTurn--
	}
}
