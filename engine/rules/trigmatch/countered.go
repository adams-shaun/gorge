package trigmatch

import (
	"strings"

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
	if v := t.ParamStr(cards.PKValidCause); v != "" && !causeMatches(v, ev.Player, ctrl) {
		return false
	}
	if v := t.ParamStr(cards.PKValidSA); v != "" {
		kind, quals, _ := strings.Cut(v, ".")
		switch kind {
		case "Spell":
			if ev.Amount != 1 {
				return false
			}
			if quals != "" && !e.MatchesSpec("Card."+quals, ev.Obj, source, ctrl, SpecOpts{}) {
				return false
			}
		case "Ability":
			if ev.Amount != 0 || quals != "" {
				return false
			}
		default:
			return false
		}
	}
	if v := t.ParamStr(cards.PKValidCard); v != "" && !e.MatchesSpec(v, ev.Obj, source, ctrl, SpecOpts{}) {
		return false
	}
	return true
}

// causeMatches reads a "SpellAbility[.YouCtrl|.OppCtrl]" cause spec against
// the countering controller; any other base or qualifier is false.
func causeMatches(spec string, causer, ctrl state.PlayerID) bool {
	base, quals, _ := strings.Cut(spec, ".")
	if base != "SpellAbility" {
		return false
	}
	if quals == "" {
		return true
	}
	for q := range strings.SplitSeq(quals, "+") {
		switch q {
		case "YouCtrl":
			if causer != ctrl {
				return false
			}
		case "OppCtrl":
			if causer == ctrl {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func init() {
	registerTrigMatcher(func(e Board, t cards.Trigger, source state.ObjID, ev events.Event, lki *state.Object) bool {
		return counteredMatches(e, t, source, ev, lki)
	}, "Countered")
	effects.RegisterNonAPI("trig:Countered")
}
