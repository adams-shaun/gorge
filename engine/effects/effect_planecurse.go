package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// DurationUntilPlaneswalk is the lifetime effectPlaneCurseEnd gives the
// plane-source curse's registration; rules ends every registration carrying
// it at the next planeswalk.
const DurationUntilPlaneswalk = "UntilPlaneswalk"

// effectPlaneCurseStatic reports whether an Effect's StaticAbilities$ names
// exactly one CantBeCast static: the only pairing whose PlaneswalkedTo
// self-exile trigger is read as the Effect's end condition (Eloren Wilds'
// plane-source curse).
func effectPlaneCurseStatic(c *Ctx, staticAbilities string) bool {
	names := strings.FieldsFunc(staticAbilities, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
	if len(names) != 1 {
		return false
	}
	m, _ := parseStaticLine(c.SVars, names[0])
	return cards.StaticModeOf(m) == cards.StaticCantBeCast
}

// effectPlaneCurseTrigger reports whether tr is the plane-source curse's own
// PlaneswalkedTo self-exile trigger on a Duration$ Permanent Effect whose one
// static is CantBeCast. The unscoped PlaneswalkedTo fires at any player's
// planeswalk, so the trigger IS the Effect's end; gorge has no effect-token
// object for its self-exile to move, so the caller skips it instead of
// registering an inert delayed trigger. Anything outside that pairing falls
// through to the fail-closed unimplemented-Note path.
func effectPlaneCurseTrigger(h Host, c *Ctx, ep *EffectParams, tr cards.Trigger) bool {
	return effectDurationPermanent(ep.Duration) &&
		effectPlaneCurseStatic(c, ep.StaticAbilities) &&
		effectSelfExileOnPlaneswalkTrigger(h, c, tr)
}

// effectPlaneCurseEnd gives the curse's CantBeCast registration its
// lifetime: "can't cast spells until a player planeswalks". The registration
// outlives its plane source (Permanent: continuousLive's battlefield rule
// reads nothing in ZPlanarDeck) until rules' checkPlaneswalkTriggers ends
// every UntilPlaneswalk registration at the next PlanarWalk -- the point the
// Effect's own PlaneswalkedTo self-exile trigger would move its token to
// exile. Any other Effect's registration is returned unchanged.
func effectPlaneCurseEnd(h Host, c *Ctx, ep *EffectParams, ce state.ContinuousEffect) state.ContinuousEffect {
	for _, name := range ep.TriggerNames {
		tr, ok := cards.ParseTriggerLine(effectTriggerBody(h, c, name))
		if !ok || !effectPlaneCurseTrigger(h, c, ep, tr) {
			continue
		}
		ce.Duration = DurationUntilPlaneswalk
		ce.Permanent = true
		ce.UntilEOT = false
		break
	}
	return ce
}
