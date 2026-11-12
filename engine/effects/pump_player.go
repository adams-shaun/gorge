package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// registerPumpPlayerGrant gives the resolving ability's controller the
// player-keyword half of a Pump's grant ("You and permanents you control gain
// hexproof from blue and from black until end of turn": Veil of Summer, Dawn's
// Truce, Lazotep Plating, Surge of Salvation; CR 702.11, 702.18, 702.16). A
// player is not an object, so the per-object loop skips them; the keywords a
// player can carry (hexproof, shroud, protection: rules' playerKeywords reads
// exactly those) ride one layer-6 effect over the controller, with the pump's
// own lifetime. Another player is not carried: no corpus Pump names one with
// such a keyword, and "You" is the one player spec a registration can name.
func registerPumpPlayerGrant(h Host, c *Ctx, pl state.PlayerID, g *PumpGrant, chosenKW []string) {
	if pl != c.Controller {
		return
	}
	var kws []string
	for _, kw := range append(g.KW[:len(g.KW):len(g.KW)], chosenKW...) {
		switch strings.ToLower(cards.KeywordHead(kw)) {
		case "hexproof", "shroud", "protection":
			kws = append(kws, kw)
		}
	}
	if len(kws) == 0 {
		return
	}
	permanent, untilEOT := durationTiming(g.Duration)
	h.AddContinuous(state.ContinuousEffect{
		Source: c.Source, Affects: "You", Controller: c.Controller,
		Layer: state.LAbilities, AddKeywords: kws,
		Duration: g.Duration, Permanent: permanent, UntilEOT: untilEOT,
	})
}
