package rules

// etb_modechoice.go poses an as-enters GenericChoice (K:ETBReplacement:Other
// with SetChosenMode$ True: the Sieges' "As CARDNAME enters, choose Khans or
// Dragons") at the entry boundary when no tape run serves the entry. A card an
// until-leaves exile returns from a state-based sweep (a seat leaving the game
// ceases the exiler, CR 800.4a; a host dying to damage) enters outside any
// resolution, so the body's own ask found no run and took its first mode. The
// owner chooses (CR 614.12, 610.3): the choice rides the entry machinery the
// other as-enters choices use (applyETBChoiceReplacement), answered as a
// Choose "mode" event the fold records on the permanent.

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// DebugETBModeNoEntryAsk is the lockstep mutation: the entry never poses the
// as-enters mode choice outside a run (the old default-answered behaviour).
var DebugETBModeNoEntryAsk bool

// etbModeOptions returns the option list of o's as-enters GenericChoice body
// with, and whether the entry boundary asks it: only outside a tape run, for
// the controller's own choice (Defined$ You) whose every choice does nothing
// beyond being recorded (the Sieges' DB$ Nothing bodies).
func (e *Engine) etbModeOptions(o *state.Object, with *cards.SA, svars map[string]string) ([]decision.Option, bool) {
	if DebugETBModeNoEntryAsk || with == nil || with.API != "GenericChoice" || e.tape.Serving() {
		return nil, false
	}
	p := effects.CharmOf(with)
	if !strings.EqualFold(with.ParamStr(cards.PKSetChosenMode), "True") || p.Defined != "You" ||
		len(p.Modes) < 2 || effects.GenericChoiceAtRandom(with) {
		return nil, false
	}
	opts := make([]decision.Option, 0, len(p.Modes))
	for i, name := range p.Modes {
		sub := cards.ResolveSVar(svars, name)
		if !doesNothing(sub) {
			return nil, false
		}
		opts = append(opts, decision.Option{Index: i, Kind: "mode",
			Label: effects.CharmModeLabel(sub, name), Obj: o.ID, Player: o.Controller})
	}
	return opts, true
}

// emitETBMode records the answered as-enters mode on the entering object: the
// Choose "mode" event the fold stores in ChosenModes, named by the chosen
// choice's SVar so Card.ChosenModeKhans reads it.
func (e *Engine) emitETBMode(obj state.ObjID, opt decision.Option) {
	o := e.G.Obj(obj)
	if o == nil || o.Face() == nil {
		return
	}
	for i := range o.Face().Repls {
		with := o.Face().Repls[i].With
		if _, ok := e.etbModeOptions(o, with, o.Face().SVars); !ok {
			continue
		}
		if names := effects.CharmOf(with).Modes; opt.Index >= 0 && opt.Index < len(names) {
			e.emit(events.Event{Kind: events.Choose, Obj: obj, Counter: "mode", Text: names[opt.Index]})
		}
		return
	}
}

// doesNothing reports whether sa is a DB$ Nothing body: the parser folds it into
// a Pump (cards.canonicalAPI) that carries no parameter but its description and
// no SubAbility$.
func doesNothing(sa *cards.SA) bool {
	if sa == nil || sa.API != "Pump" || sa.Sub != nil {
		return false
	}
	for k := range sa.Params {
		if k != "SpellDescription" {
			return false
		}
	}
	return true
}
