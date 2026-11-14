package effects

import "github.com/adams-shaun/gorge/engine/state"

// playerThanActive evaluates the Oath cycle's player properties that compare
// the candidate seat with the ACTIVE player (Forge PlayerProperty
// withMoreCreaturesThanActive / withMoreLandsThanActive /
// hasMoreCardsInHandThanActive / hasMoreLifeThanActive /
// hasFewerCreaturesInYardThanActive). "Active" is the player whose turn it
// is (g.Active); each comparison is strict, so a tie names nobody. Counts are
// read after continuous effects (CR 613.1d, 205.1) through the same type
// predicate the object filters use.
func playerThanActive(g *state.Game, p state.PlayerID, code playerSpecQualifierCode, pc *PlayerSpecCtx) bool {
	if int(p) >= len(g.Players) || int(g.Active) >= len(g.Players) || g.Active < 0 {
		return false
	}
	sc := SpecContext{Layers: pc.Layers}
	count := func(pi state.PlayerID, zone state.Zone, typ string) int32 {
		var n int32
		for _, id := range g.Zone(zone, pi) {
			if o := g.Obj(id); o != nil && hasTypeCtxPtr(o, typ, &sc) {
				n++
			}
		}
		return n
	}
	switch code {
	case playerSpecQualifierMoreCreaturesThanActive:
		return count(p, state.ZBattlefield, "Creature") > count(g.Active, state.ZBattlefield, "Creature")
	case playerSpecQualifierMoreLandsThanActive:
		return count(p, state.ZBattlefield, "Land") > count(g.Active, state.ZBattlefield, "Land")
	case playerSpecQualifierMoreCardsInHandThanActive:
		return len(g.Zone(state.ZHand, p)) > len(g.Zone(state.ZHand, g.Active))
	case playerSpecQualifierMoreLifeThanActive:
		return g.Players[p].Life > g.Players[g.Active].Life
	case playerSpecQualifierFewerCreaturesInYardThanActive:
		return count(p, state.ZGraveyard, "Creature") < count(g.Active, state.ZGraveyard, "Creature")
	}
	return false
}
