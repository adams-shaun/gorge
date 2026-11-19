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
// Probe). The triggering event is the Secret events.Shuffle. "Caused by a
// spell or ability" -- Forge's ShuffleFromEffect$ True, the only form both
// corpus carriers print -- is read off the engine, not the logged event: the
// shuffle must happen while a stack object resolves (InResolution) with a
// resolving cause (ActionCause). The game-rule shuffles (mulligan, restart,
// the genesis deal) never happen in resolution, so they do not fire it; an
// Omen spell's CR 720.3d resolution shuffle does, since its owner shuffles
// it into their library as part of that spell's resolution. ValidPlayer$
// names the player whose library was shuffled (ev.Player).
// ShuffleBySelfControlled$ True is Widespread Panic's "causes ITS CONTROLLER
// to shuffle": the resolving stack object that caused the shuffle (the same
// ActionCause the ValidCause$ readers use) must be controlled by the player
// whose library was shuffled, so an opponent-targeted shuffle (Spellshift,
// Knowledge Exploitation) does not fire it while Psychogenic Probe, which
// omits the param, still sees every effect-caused shuffle.
func shuffledMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
	if ev.Kind != events.Shuffle {
		return false
	}
	cause := e.ActionCause()
	if !e.InResolution() || cause == 0 {
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
