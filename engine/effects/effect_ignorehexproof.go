package effects

import (
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// effectUnsupportedStatic handles a statically unknown Effect body. Detection
// Tower's targeting exception and Hall of Echoes' legend-rule exemption are
// two supported modes delivered this way.
func effectUnsupportedStatic(h Host, c *Ctx, mode string, params staticLineParams, name, rawDur, dur, what string, remembered []state.ObjID) bool {
	if mode == ModeIgnoreHexproof && len(params["ValidEntity"]) > 0 &&
		staticKeysReadable(params, "Mode", "Activator", "ValidEntity", "Description") {
		effectContinuous(h, state.ContinuousEffect{
			Source: c.Source, Controller: c.Controller, Name: name,
			UntilEOT: effectUntilEOT(h, c.Source, rawDur), Duration: dur,
			Restriction: mode, RestrictParams: params,
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
	if len(mode) > 0 {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "continuous effect " + mode + " unimplemented (" + what + ")"})
	}
	return false
}
