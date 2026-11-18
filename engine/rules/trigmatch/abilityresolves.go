package trigmatch

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// abilityResolvesMatches implements Mode$ AbilityResolves (trig:AbilityResolves;
// Tom Bombadil's and Narci's "whenever the final chapter ability of a Saga you
// control resolves"). It reads the events.AbilityResolved marker rules emits
// beside a resolving ability's Resolve event: Obj the ability's source
// permanent, Amount the AbilityResolved* flag bits.
//
//   - ValidSource$ is filtered against the source permanent (Saga.YouCtrl).
//   - ValidSpellAbility$ is "Ability" or "Triggered" (a triggered ability
//     only) plus an optional ".LastChapter" qualifier (the Saga's final
//     chapter ability). Any other base or qualifier (SpellAbility.ManaAbility:
//     a mana ability never reaches the stack) fails closed.
func abilityResolvesMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
	if ev.Kind != events.AbilityResolved {
		return false
	}
	if v := t.ParamStr(cards.PKValidSource); len(v) > 0 &&
		!e.MatchesSpec(v, ev.Obj, source, e.ControllerOf(source), SpecOpts{}) {
		return false
	}
	if v := t.ParamStr(cards.PKValidSpellAbility); len(v) > 0 {
		sa := cards.ParseTrigSASpec(v)
		switch {
		case sa.Head == cards.TrigSAHeadOther, sa.Quals&^cards.TrigSAQualLastChapter != 0:
			return false
		case sa.Head == cards.TrigSAHeadTriggered && ev.Amount&events.AbilityResolvedTriggered == 0,
			sa.Quals&cards.TrigSAQualLastChapter != 0 && ev.Amount&events.AbilityResolvedLastChapter == 0:
			return false
		}
	}
	return true
}

func init() {
	registerTrigMatcher(func(e Board, t cards.Trigger, source state.ObjID, ev events.Event, lki *state.Object) bool {
		return abilityResolvesMatches(e, t, source, ev, lki)
	}, "AbilityResolves")
	effects.RegisterNonAPI("trig:AbilityResolves")
}
