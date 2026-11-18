package trigmatch

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// counteredMatches implements Mode$ Countered (trig:Countered; Baral, Chief
// of Compliance: "whenever a spell or ability you control counters a spell").
// It reads the events.Countered marker api:Counter emits after the countered
// object's move (CR 701.6a): Obj the countered object, Player the countering
// ability's controller, Amount 1 for a countered spell.
//
//   - ValidCause$ is the COUNTERING spell or ability: "SpellAbility" with the
//     YouCtrl / OppCtrl qualifiers, read against ev.Player (the controller of
//     the ability that countered). Any other qualifier fails closed.
//   - ValidSA$ is the COUNTERED object: "Spell" (Amount 1) or "Ability"
//     (Amount 0); a "Spell.<quals>" form also holds the qualifiers against
//     the countered card (Spell.Creature+wasCastByYou). Anything else fails
//     closed.
//   - ValidCard$ is the countered card's filter (Card.wasCastByYou).
//
// The marker is emitted only while a battlefield permanent carries a
// Mode$ Countered line, so the trigger zones are the battlefield's.
func counteredMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
	if ev.Kind != events.Countered {
		return false
	}
	ctrl := e.ControllerOf(source)
	if v := t.ParamStr(cards.PKValidCause); len(v) > 0 && !causeMatches(v, ev.Player, ctrl) {
		return false
	}
	if v := t.ParamStr(cards.PKValidSA); len(v) > 0 {
		sa := cards.ParseTrigSASpec(v)
		switch sa.Head {
		case cards.TrigSAHeadSpell:
			if ev.Amount != 1 || len(sa.Rest) > 0 && !e.MatchesSpec("Card."+sa.Rest, ev.Obj, source, ctrl, SpecOpts{}) {
				return false
			}
		case cards.TrigSAHeadAbility:
			if ev.Amount != 0 || len(sa.Rest) > 0 {
				return false
			}
		default:
			return false
		}
	}
	if v := t.ParamStr(cards.PKValidCard); len(v) > 0 && !e.MatchesSpec(v, ev.Obj, source, ctrl, SpecOpts{}) {
		return false
	}
	return true
}

// causeMatches reads a "SpellAbility[.YouCtrl|.OppCtrl]" cause spec against
// the countering controller; any other base or qualifier is false.
func causeMatches(spec string, causer, ctrl state.PlayerID) bool {
	c := cards.ParseTrigSASpec(spec)
	const known = cards.TrigSAQualYouCtrl | cards.TrigSAQualOppCtrl
	switch {
	case c.Head != cards.TrigSAHeadSpellAbility, c.Quals&^known != 0:
		return false
	case c.Quals&cards.TrigSAQualYouCtrl != 0 && causer != ctrl, c.Quals&cards.TrigSAQualOppCtrl != 0 && causer == ctrl:
		return false
	}
	return true
}

func init() {
	registerTrigMatcher(func(e Board, t cards.Trigger, source state.ObjID, ev events.Event, lki *state.Object) bool {
		return counteredMatches(e, t, source, ev, lki)
	}, "Countered")
	effects.RegisterNonAPI("trig:Countered")
}
