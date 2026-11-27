package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// drawReplacementPlayerMatches is the Draw class's ValidPlayer$ gate. A
// Player.IsRemembered line reads the registration's own remembered players
// (state.ContinuousEffect.RememberedPlayers, threaded through replMatch): the
// bare MatchesPlayerSpec read carries an empty PlayerSpecCtx and no capture,
// so Plagiarize's ValidPlayer$ Player.IsRemembered could never match. This is
// the same idiom gainLifeReplacementMatches uses for a LifeChange line. Every
// other spec keeps the ordinary player filter -- and a Player.IsRemembered
// line on a PRINTED replacement (no registration capture) still fails closed
// exactly as it did: the membership loop runs over an empty set.
func drawReplacementPlayerMatches(g *state.Game, spec string, p, you state.PlayerID, rememberedPlayers []state.PlayerID) bool {
	if strings.TrimSpace(spec) == "Player.IsRemembered" {
		for _, rp := range rememberedPlayers {
			if rp == p {
				return true
			}
		}
		return false
	}
	return effects.MatchesPlayerSpec(g, spec, p, you)
}
