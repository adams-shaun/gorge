package effects

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// targetPlayerKindScope resolves a player-kind ValidTgts$ (Player/Opponent) on
// an *All sweep to the seats that sweep is restricted to, through the same
// referent the TargetedPlayerCtrl filter predicate uses so the two cannot
// drift.  A non-nil scope that resolves to no player leaves an EMPTY map: the
// caller sees a non-nil scope and fails CLOSED (it sweeps nothing), matching
// the Defined/TargetedPlayerCtrl direction rather than falling back to every
// seat.
//
// A sub-ability that carries no ValidTgts$ of its own can still inherit the
// parent chain's player target: Sleep's `DB$ PumpAll | Defined$ Targeted`
// follows a `TapAll | ValidTgts$ Player`, and Defined$ Targeted (or
// TargetedPlayer) names the player the parent chose. Without this branch the
// helper returned nil for the sub and the sweep covered every seat, stamping
// the caster's own creatures. An inherited scope with no player target in the
// resolution list (a Defined$ Targeted sub behind an object-only parent) is
// NOT the "each <object> target player controls" shape, so it stays nil and
// the filter-only sweep stands.
//
// A non-player ValidTgts$ (e.g. Creature) and an absent one both return nil,
// so the filter-only sweep stands.  It is the ONE implementation shared by
// every *All primitive that carries the "each <object> target player controls"
// shape (DamageAll's Aggravate/Simoon/Chandra, Bold Pyromancer; AnimateAll's
// Curious Colossus), so a new carrier cannot silently sweep every seat because
// only one of them grew the branch.
func targetPlayerKindScope(h Host, c *Ctx, sa *cards.SA) map[state.PlayerID]bool {
	tg := TargetsOf(sa).ValidTgts
	ownPlayerKind := tg != "" && playerSpecBaseKnown(tg)
	inheritsPlayer := definedScopesToTargetedPlayer(sa)
	if !ownPlayerKind && !inheritsPlayer {
		return nil
	}
	targets := targetedGroup(c)
	if !ownPlayerKind && !hasPlayerTarget(targets) {
		return nil
	}
	sc := c.SpecContext(c.Controller)
	sc.ResolutionTargets = targets
	players, _ := controlReferentPlayers(h.Game(), sc, "ControlledBy", "TargetedPlayer")
	scope := make(map[state.PlayerID]bool, len(players))
	for _, p := range players {
		scope[p] = true
	}
	return scope
}

// definedScopesToTargetedPlayer reports whether sa names the parent chain's
// chosen player target as its scope rather than declaring a ValidTgts$ of its
// own. Defined$ Targeted is the whole-chain target union; TargetedPlayer is
// its player half. Both resolve through the resolution's target list, the
// same binding controlReferentPlayers' TargetedPlayer arm reads, so the
// inheritance and the filter predicate cannot drift.
func definedScopesToTargetedPlayer(sa *cards.SA) bool {
	switch definedSpecCodes.Code(DefinedRefOf(sa).Text) {
	case definedSpecTargeted, definedSpecTargetedPlayer:
		return true
	}
	return false
}

// hasPlayerTarget reports whether ts contains a direct player target (an
// object target is not one, even though a different referent may read its
// controller).
func hasPlayerTarget(ts []state.Target) bool {
	for _, t := range ts {
		if t.IsPlayer {
			return true
		}
	}
	return false
}

// inSweepScope reports whether an object at id is inside a target-player-kind
// scope.  A nil scope admits every object; a non-nil one admits only objects
// whose controller is in the set (an object that has left the game is
// excluded).
func inSweepScope(g *state.Game, id state.ObjID, scope map[state.PlayerID]bool) bool {
	if scope == nil {
		return true
	}
	o := g.Obj(id)
	return o != nil && scope[o.Controller]
}
