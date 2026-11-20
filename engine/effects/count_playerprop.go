package effects

import (
	"slices"

	"github.com/adams-shaun/gorge/engine/state"
)

// count_playerprop.go holds the PlayerCount<Group>$<Property> properties of
// the you/defined/remembered player (the Commander-deck census class: Master
// of Waves, Simulacrum, Tainted Sigil, Invasion of Fiora, Celestial
// Convergence) and the player-group selectors that name a referent
// (RememberedOwner, RememberedPlayer, DefinedTriggeredPlayer). Every read is
// of state the engine already folds, so it is replay-derived.

// playerPropGroupMembers is the member set of the referent-named
// PlayerCount groups. An absent referent is the empty group, so a count over
// it reads a real zero (the resolution simply has no such player).
//
//   - PlayerCountRememberedOwner: the distinct OWNERS of the remembered
//     objects (Tempting Contract's "for each opponent who does": the owners
//     of the Treasure tokens the per-opponent bodies remembered).
//   - PlayerCountRememberedPlayer: the remembered PLAYER entries (Garland,
//     Royal Kidnapper's remembered monarch).
//   - PlayerCountDefinedTriggeredPlayer: the trigger's player (Faramir's
//     delayed end-step trigger's active player).
func playerPropGroupMembers(g *state.Game, c *Ctx, group cov3PlayerGroupCode) []state.PlayerID {
	switch group {
	case cov3PlayerGroupPlayerCountRememberedOwner:
		return cov3RememberedOwners(g, c)
	case cov3PlayerGroupPlayerCountRememberedPlayer:
		var out []state.PlayerID
		for _, t := range c.Remembered {
			if t.IsPlayer && t.Player >= 0 && int(t.Player) < len(g.Players) {
				out = append(out, t.Player)
			}
		}
		return out
	case cov3PlayerGroupPlayerCountDefinedTriggeredPlay:
		if c.TriggerPlayer.IsPlayer && c.TriggerPlayer.Player >= 0 && int(c.TriggerPlayer.Player) < len(g.Players) {
			return []state.PlayerID{c.TriggerPlayer.Player}
		}
	case cov3PlayerGroupPlayerCountDefinedTriggeredSourceController:
		if p, ok := TriggeredSourceController(g, c.TriggerContext, c.Remembered); ok && p >= 0 && int(p) < len(g.Players) {
			return []state.PlayerID{p}
		}
	case cov3PlayerGroupPlayerCountDefinedTriggeredCardOwner:
		// The triggering card's immutable owner (CR 108.3), never the source's.
		if o := g.Obj(c.TriggerCard); o != nil && o.Owner >= 0 && int(o.Owner) < len(g.Players) {
			return []state.PlayerID{o.Owner}
		}
	case cov3PlayerGroupPlayerCountDefinedTriggeredAttackedTargetAndYou:
		// The attacked player and the resolving controller: one member when
		// they are the same seat.
		var out []state.PlayerID
		if t := c.AttackedTarget; t.IsPlayer && t.Player >= 0 && int(t.Player) < len(g.Players) {
			out = append(out, t.Player)
		}
		if c.Controller >= 0 && int(c.Controller) < len(g.Players) && !slices.Contains(out, c.Controller) {
			out = append(out, c.Controller)
		}
		return out
	}
	return nil
}

// evalPlayerPropScalar answers the scalar properties of
// evalCov3PlayerScalar's dispatch that the census class added, dispatched on
// the shared cov3PlayerPropCodes vocabulary (its Valid and LifeLostThisTurn
// keys already lived there) and the cov3PlayerGroupCodes group code:
//
//   - Valid <spec> (PlayerCountPropertyYou only): the CARD count of the
//     Count$Valid fold, Master of Waves' "Elemental creatures you control"
//     AI counts; the same filter-property reading PlayerCountRemembered$Valid
//     takes.
//   - LifeLostThisTurn: the counted players' life lost this turn, summed
//     (Tainted Sigil's "total life lost by all players this turn").
//   - DamageThisTurn: the damage dealt to the counted players this turn,
//     summed (Simulacrum's "damage dealt to you this turn").
//   - BeenDealtCombatDamageSinceLastTurn (PlayerCountPropertyYou only): 1
//     when the resolving controller was dealt combat damage since their last
//     turn ended, else 0 (Invasion of Fiora's Marchesa upkeep gate); read
//     through HostCombatLedger, the same role the this-turn combat hits read.
//   - TiedForHighestLife (every-player group only): how many living players
//     hold the highest life total (Celestial Convergence's draw check: two
//     or more).
func evalPlayerPropScalar(h Host, c *Ctx, g *state.Game, group cov3PlayerGroupCode, players []state.PlayerID, prop, arg string, depth int) (int32, bool) {
	switch cov3PlayerPropCodes.Code(string(prop)) {
	case cov3PlayerPropValid:
		if group != cov3PlayerGroupPlayerCountPropertyYou || arg == "" {
			return 0, false
		}
		return evalCountBody(h, c, "Valid "+arg, depth+1)
	case cov3PlayerPropLifeLostThisTurn:
		var n int32
		for _, p := range players {
			n += h.LifeLostThisTurn(p)
		}
		return n, true
	case cov3PlayerPropDamageThisTurn:
		var n int32
		for _, p := range players {
			n += h.DamageTakenThisTurn(p)
		}
		return n, true
	case cov3PlayerPropBeenDealtCombatDamageSinceLastTurn:
		if group != cov3PlayerGroupPlayerCountPropertyYou {
			return 0, false
		}
		if h.WasDealtCombatDamageSinceLastTurn(c.Controller) {
			return 1, true
		}
		return 0, true
	case cov3PlayerPropTiedForHighestLife:
		if group != cov3PlayerGroupPlayerCount {
			return 0, false
		}
		var best, n int32
		for i, p := range players {
			life := g.Players[p].Life
			switch {
			case i == 0 || life > best:
				best, n = life, 1
			case life == best:
				n++
			}
		}
		return n, true
	}
	return 0, false
}
