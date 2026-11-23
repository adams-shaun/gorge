package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// emitAbilityResolved records that ability object o (stack id id) began to
// resolve, for Mode$ AbilityResolves (events.AbilityResolved) and Mode$
// BecomesCrewed (CR 702.122e). It runs beside the ability's Resolve event and
// emits nothing unless a battlefield permanent carries one of those two
// modes, so every other game keeps its chain. triggered is the engine's
// trigger-line lookup (an ability is a triggered one when it has a line); tc
// is the ability's captured trigger context, whose Chapter/ChapterLast flags
// stamp the Saga chapter bits.
func emitAbilityResolved(g *state.Game, emit func(events.Event) events.Event, id state.ObjID, o *state.Object,
	tc effects.TriggerContext, triggered func(state.ObjID, *state.Object) (cards.Trigger, bool)) {
	// The `K:Crew` expansion's minted Animate SA carries `Keyword$ Crew`
	// (engine/cards/kw_crew.go); the tag -- not any card name -- marks a crew
	// ability, the same read engine/rules/pay's EmitChoiceCosts makes.
	crew := pay.SaHasKeyword(o.Ability, "Crew")
	abilityResolves := effects.AnyBattlefieldTriggerMode(g, cards.TriggerAbilityResolves)
	becomesCrewed := crew && effects.AnyBattlefieldTriggerMode(g, cards.TriggerBecomesCrewed)
	if !abilityResolves && !becomesCrewed {
		return
	}
	var flags int32
	if crew {
		flags |= events.AbilityResolvedCrew
	}
	if _, ok := triggered(id, o); ok {
		flags |= events.AbilityResolvedTriggered
	}
	if tc.Chapter {
		flags |= events.AbilityResolvedChapter
		if tc.ChapterLast {
			flags |= events.AbilityResolvedLastChapter
		}
	}
	emit(events.Event{Kind: events.AbilityResolved, Obj: o.Source, Player: o.Controller, Amount: flags})
}
