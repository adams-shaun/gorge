package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// entryCloneTemplateFace is the copiable face of the permanent an entering
// as-enters clone (f carries the DB$ Clone | Keyword$ ETBReplacement line,
// matched against the entry ev) answered its election with, or nil: no
// election, a decline, or a template that is no longer in the zone the body
// chooses from (the Clone body then copies nothing, so nothing of the
// template's applies). CR 707.5: the copied text's "enters with" replacement
// takes effect, so the entry plan reads the template's counter bodies next to
// the entering face's own (Clone copying Kalonian Hydra enters with its four
// +1/+1 counters).
func (e *Engine) entryCloneTemplateFace(entrant state.ObjID, f *cards.Face, ev events.Event) *cards.Face {
	tpl := e.cloneRiderTemplate(entrant)
	if tpl == nil {
		return nil
	}
	for i := range f.Repls {
		r := &f.Repls[i]
		if r.Event != "Moved" || r.With == nil || r.With.API != "Clone" ||
			r.ParamStr(cards.PKKeyword) != "ETBReplacement" ||
			!e.replacementMatches(*r, entrant, ev) {
			continue
		}
		if tpl.Zone != effects.CloneOf(r.With).ChoiceZoneKind {
			return nil
		}
		return tpl.CopiableFace()
	}
	return nil
}

// entryCounterBody reports whether r is a printed enters-with-counters line
// (a Moved PutCounter | ETB$ True body) the entry fold can encode.
func entryCounterBody(r *cards.Repl) bool {
	return r.Event == "Moved" && r.With != nil && r.With.API == "PutCounter" &&
		effects.PutCounterOf(r.With).ETB && entryBodyKindEncodable(r.With)
}
