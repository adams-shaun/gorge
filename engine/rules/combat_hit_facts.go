package rules

import (
	"slices"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// combatHitThisTurn reports whether p dealt combat damage to a player this
// turn with a source match accepts: the turn fact prowl (CR 702.76a) and
// freerunning (CR 702.173a) gate their alternative costs on. It reads the
// per-object DamageDealtThisTurn ledger the DamageProvenance fold fills
// (engine/events/apply_combat.go foldDamageProvenance) and TurnChange clears,
// so a hit counts after its source has left the battlefield. match sees the
// source's types frozen at the hit (Changeling already expanded to every
// creature type by recordDamageProvenance), never its live ones; a record
// from a log that predates the snapshot falls back to the printed face.
// Objs is a dense arena walked by index, so the answer never depends on map
// order.
func combatHitThisTurn(g *state.Game, p state.PlayerID, match func(types []string, src state.ObjID) bool) bool {
	for i := range g.Objs {
		src := &g.Objs[i]
		for _, hit := range src.DamageDealtThisTurn {
			if !hit.Combat || hit.SourceControl != p {
				continue
			}
			if _, ok := hit.Recipient.PlayerRef(); !ok {
				continue
			}
			types := hit.SourceTypes
			if !hit.HasSourceZone {
				if f := src.Face(); f != nil {
					types = f.Types
				}
			}
			if match(types, src.ID) {
				return true
			}
		}
	}
	return false
}

// prowlHit is prowl's match (CR 702.76a): the source shared a creature type
// with the spell, whose own types (spellTypes, layer-4 and Changeling
// included) are read as the offer is made.
func prowlHit(spellTypes []string) func([]string, state.ObjID) bool {
	return func(types []string, _ state.ObjID) bool {
		for _, t := range types {
			if effects.CreatureTypeWords(t) && slices.Contains(spellTypes, t) {
				return true
			}
		}
		return false
	}
}

// freerunningHit is freerunning's match (CR 702.173a): the source was an
// Assassin, or a commander ("an Assassin or commander you control" — the
// control half is combatHitThisTurn's SourceControl filter, so the
// commander half is any seat's commander card, Game.CommanderOf, which
// holds after the source leaves the battlefield).
func freerunningHit(g *state.Game, p state.PlayerID) func([]string, state.ObjID) bool {
	return func(types []string, src state.ObjID) bool {
		return slices.Contains(types, "Assassin") || g.CommanderOf(src) != 0
	}
}

// opponentLostLifeThisTurn reports whether some living opponent of p lost
// life this turn: spectacle's gate (CR 702.137a). It reads the
// Player.LifeLostThisTurn fold, walking AliveFrom(0) in seat order.
func opponentLostLifeThisTurn(g *state.Game, p state.PlayerID) bool {
	for _, i := range g.AliveFrom(0) {
		if i != p && g.Players[i].LifeLostThisTurn > 0 {
			return true
		}
	}
	return false
}
