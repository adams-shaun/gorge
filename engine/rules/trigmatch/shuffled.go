package trigmatch

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// shuffledMatches implements Mode$ Shuffled ("Whenever a spell or ability
// causes its controller to shuffle their library", Widespread Panic;
// "Whenever a spell or ability causes a player to shuffle", Psychogenic
// Probe). The triggering event is the Secret events.Shuffle a resolving
// effect emits; every effect-caused shuffle carries the
// events.ShuffleFromEffectMarker in Text, while the game-rule shuffles
// (mulligan, restart, the genesis deal) carry none, so requiring the marker
// is the ShuffleFromEffect$ True filter -- the only form both corpus carriers
// print -- without a new cards param key. ValidPlayer$ names the player whose
// library was shuffled (ev.Player, the same seat the marker's emitters name).
// ShuffleBySelfControlled$ True is Widespread Panic's "causes ITS CONTROLLER
// to shuffle": the resolving stack object that caused the shuffle (the same
// ActionCause the ValidCause$ readers use) must be controlled by the player
// whose library was shuffled, so an opponent-targeted shuffle (Spellshift,
// Knowledge Exploitation) does not fire it while Psychogenic Probe, which
// omits the param, still sees every effect-caused shuffle.
func shuffledMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
	if ev.Kind != events.Shuffle || ev.Text != events.ShuffleFromEffectMarker {
		return false
	}
	p := ev.Player
	if int(p) < 0 || int(p) >= len(e.Game().Players) {
		return false
	}
	if v, ok := t.Param(cards.PKValidPlayer); ok &&
		!effects.MatchesPlayerSpec(e.Game(), v, p, e.ControllerOf(source)) {
		return false
	}
	if t.ParamStr(cards.PKShuffleBySelfControlled) == "True" {
		cause := e.ActionCause()
		if !e.InResolution() || cause == 0 {
			return false
		}
		co := e.Game().Obj(cause)
		if co == nil || co.Controller != p {
			return false
		}
	}
	return true
}

func init() {
	registerTrigMatcher(shuffledMatches, "Shuffled")
	effects.RegisterNonAPI("trig:Shuffled")
}
