package trigmatch

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// destroyedText is the Text every destroy effect stamps on its MoveZone.
const destroyedText = "destroyed"

// destroyedMatches implements Mode$ Destroyed ("whenever a spell or ability an
// opponent controls destroys a noncreature permanent you control", Karmic
// Justice; Cobra Trap's tracker; "whenever a noncreature artifact is ...
// destroyed", Jaws). The triggering event is the battlefield-to-graveyard
// MoveZone every destroy effect emits with Text "destroyed" (effDestroy,
// DestroyAll, regeneration's Aura-instead destroy). The lethal-damage and
// zero-toughness state-based actions use their own Text, so they never fire it
// (CR 704.5g/704.5f are destruction by rule, not by a spell or ability).
// ValidCard$ is read against the permanent as it last existed on the
// battlefield (the LKI the walk passes), like Sacrificed's.
//
// ValidCauser$ names the PLAYER controlling the spell or ability that
// destroyed it: the resolving stack object ActionCause names, matched as a
// player spec from the trigger source controller's seat. A destroy with no
// resolving cause (the lines print no cause) fails a ValidCauser$ line closed;
// a line without ValidCauser$ (Jaws) needs none.
func destroyedMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event, lki *state.Object) bool {
	if ev.Kind != events.MoveZone || ev.Text != destroyedText ||
		ev.From != state.ZBattlefield || ev.To != state.ZGraveyard {
		return false
	}
	ctrl := e.ControllerOf(source)
	if v := t.ParamStr(cards.PKValidCard); v != "" {
		if lki == nil || !e.MatchesObject(v, lki, source, ctrl, SpecOpts{}) {
			return false
		}
	}
	if v := t.ParamStr(cards.PKValidCauser); v != "" {
		causer, ok := DestroyCauser(e)
		if !ok || !effects.MatchesPlayerSpec(e.Game(), v, causer, ctrl) {
			return false
		}
	}
	return true
}

// DestroyCauser is the player controlling the spell or ability whose
// resolution is destroying a permanent right now (the TriggeredCauser a
// Mode$ Destroyed body binds). ok is false outside a resolution.
func DestroyCauser(e Board) (state.PlayerID, bool) {
	cause := e.ActionCause()
	if !e.InResolution() || cause == 0 {
		return 0, false
	}
	co := e.Game().Obj(cause)
	if co == nil {
		return 0, false
	}
	return co.Controller, true
}

func init() {
	registerTrigMatcher(destroyedMatches, "Destroyed")
	effects.RegisterNonAPI("trig:Destroyed")
}
