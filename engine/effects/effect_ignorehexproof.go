package effects

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// effectUnsupportedStatic handles a statically unknown Effect body. Detection
// Tower's targeting exception and Hall of Echoes' legend-rule exemption are
// two supported modes delivered this way.
func effectUnsupportedStatic(h Host, c *Ctx, mode string, params staticLineParams, name, rawDur, dur, what string, remembered []state.ObjID, sa *cards.SA) bool {
	if mode == ModeIgnoreHexproof && len(params["ValidEntity"]) > 0 &&
		staticKeysReadable(params, "Mode", "Activator", "ValidEntity", "Description") {
		effectContinuous(h, state.ContinuousEffect{
			Source: c.Source, Controller: c.Controller, Name: name,
			UntilEOT: effectUntilEOT(h, c.Source, rawDur), Duration: dur,
			Restriction: mode, RestrictParams: params,
		})
		return true
	}
	if mode == ModeAssignCombatDamageAsUnblocked &&
		staticKeysReadable(params, "Mode", "ValidCard", "Optional", "Description") {
		// CR 510.1c-d's "assign combat damage as though it weren't blocked",
		// delivered by a resolved ability (Predatory Focus's Creature.YouCtrl
		// this-turn grant, Outmaneuver's remembered blocked creatures,
		// Gurzigost's Card.EffectSource self-grant). Registered as a
		// restriction rules' asUnblockedStaticMatches reads beside the
		// printed S: static route; Optional$ True keeps the controller
		// election, an absent Optional$ is mandatory. A body carrying any
		// other key fails closed to the unimplemented Note.
		effectContinuous(h, state.ContinuousEffect{
			Source: c.Source, Controller: c.Controller, Name: name,
			UntilEOT: effectUntilEOT(h, c.Source, rawDur), Duration: dur,
			Restriction: mode, RestrictParams: params,
			Remembered: remembered,
		})
		return true
	}
	if mode == ModeIgnoreLegendRule &&
		staticKeysReadable(params, "Mode", "ValidCard", "Description") {
		// CR 704.5j's exemption delivered by a resolved ability (Hall of
		// Echoes' "The 'legend rule' doesn't apply to permanents you control
		// this turn."): registered as a restriction rules' legendGroups and
		// sbaQuietKindsSafe read through legendExemptions, beside the printed
		// S:Mode$ static route they already collect. An absent ValidCard$ is
		// Mirror Gallery's unconditional "The legend rule doesn't apply."
		// spelling, so it is not required. The lifetime is the Effect's own:
		// an absent Duration$ is Forge's end-of-turn default (the body's
		// oracle "this turn"), so cleanup drops the registration with it.
		effectContinuous(h, state.ContinuousEffect{
			Source: c.Source, Controller: c.Controller, Name: name,
			UntilEOT: effectUntilEOT(h, c.Source, rawDur), Duration: dur,
			Restriction: mode, RestrictParams: params,
			RestrictSVars: c.SVars, Remembered: remembered,
		})
		return true
	}
	if mode == ModeCantLoseLife &&
		staticKeysReadable(params, "Mode", "ValidPlayer", "Description") {
		// CR 119.8 "can't lose life" delivered by a resolved ability
		// (Everybody Lives!, Courageous Resolve), read by rules'
		// playerLifeLossForbidden. An absent Duration$ is this turn.
		effectContinuous(h, state.ContinuousEffect{
			Source: c.Source, Controller: c.Controller, Name: name,
			UntilEOT: effectUntilEOT(h, c.Source, rawDur) || rawDur == "", Duration: dur,
			Restriction: mode, RestrictParams: params,
			RememberedPlayers: effectRememberedPlayers(h, c, sa),
		})
		return true
	}
	if (mode == ModeCantPhaseIn || mode == ModeCantPhaseOut) &&
		staticKeysReadable(params, "Mode", "ValidCard", "Description") {
		// CR 702.26's "can't phase in" / "can't phase out" delivered by a
		// resolved ability (Ertai's Familiar, Spatial Binding, The Phasing of
		// Zhalfir, Teferi, Timeless Voyager): registered as a restriction
		// rules' phaseBlocked reads at the emit choke point, so neither the
		// untap-step phasing nor an api:Phases effect phases the permanent.
		// The IsPresent$-gated bodies (The Pandorica, The Doctor's Childhood
		// Barn) are refused here: a registered restriction has no present
		// gate, and a blanket reading would hold the permanent forever.
		//
		// Duration$ UntilYourNextUpkeep is handled by the generic duration
		// machinery (IsUntilYourNextUpkeepDuration).
		effectContinuous(h, state.ContinuousEffect{
			Source: c.Source, Controller: c.Controller, Name: name,
			UntilEOT: effectUntilEOT(h, c.Source, rawDur), Duration: dur,
			Restriction: mode, RestrictParams: params,
			RestrictSVars: c.SVars, Remembered: remembered,
		})
		return true
	}
	if len(mode) > 0 {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "continuous effect " + mode + " unimplemented (" + what + ")"})
	}
	return false
}
