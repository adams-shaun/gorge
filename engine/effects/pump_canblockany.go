package effects

import (
	"github.com/adams-shaun/gorge/engine/state"
)

// ModeCanBlockAny names the until-end-of-turn restriction a Pump's
// CanBlockAny$ True registers on each pumped creature ("can block any number
// of creatures this turn", CR 509.1a; Give No Ground, Valor Made Real). Rules'
// combat.BlockCapacity reads it beside the printed CanBlockAny$ Continuous
// static, so the offer, the validator and the bot top-up share one rule.
const ModeCanBlockAny = "CanBlockAny"

// registerPumpCanBlockAny registers CanBlockAny$ True's grant on pumped
// object id with the pump's own lifetime (Duration$ absent is this turn,
// CR 611.2a). The scope is the Remembered object, so the grant names exactly
// the creature the pump affected.
func registerPumpCanBlockAny(h Host, c *Ctx, id state.ObjID, g *PumpGrant) {
	permanent, untilEOT := durationTiming(g.Duration)
	effectContinuous(h, state.ContinuousEffect{
		Source: c.Source, Controller: c.Controller, Name: "PumpCanBlockAny",
		Duration: g.Duration, Permanent: permanent, UntilEOT: untilEOT,
		Restriction:    ModeCanBlockAny,
		RestrictParams: map[string]string{"ValidCard": "Card.IsRemembered"},
		Remembered:     []state.ObjID{id},
	})
}
