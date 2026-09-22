// Mode$ BecomeMonarch: "whenever a player becomes the monarch" (CR 716.2's
// designation change).
//
// Split out so tickets touching different modes stop colliding on one file.
// Registration is at the bottom; a duplicate mode panics (registerTrigMatcher).
package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// becomeMonarchMatches implements Mode$ BecomeMonarch: the event is the
// MonarchChange designation (the same event effBecomeMonarch emits and
// CheckDefinedPlayer$ You.isMonarch reads), and ValidPlayer$ scopes which
// becoming player the trigger fires for.
func (e *Engine) becomeMonarchMatches(t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
	if ev.Kind != events.MonarchChange {
		return false
	}
	if v := t.Params["ValidPlayer"]; v != "" && !e.monarchPlayerAdmits(v, source, ev.Player) {
		return false
	}
	return true
}

// monarchPlayerAdmits reads a BecomeMonarch trigger's ValidPlayer$ gate. The
// ordinary player grammar covers the printed carriers (Custodi Lich's
// "Opponent", Knights of the Black Rose's). The effect-owned arm adds the one
// spelling the grammar lacks: Palace Jailer's Player.OpponentOf Remembered --
// the remembered list there is the EFFECT OBJECT's own capture, which lives
// on the registration (the match identity, rules/trigger_granted.go's effect
// arm), not on any game object. The triggering player qualifies when it is an
// opponent of the controller of any remembered object (Forge evaluates the
// defined list with any-match semantics); a list with no resolvable object
// fails closed. Outside the identity the spec falls to the ordinary grammar,
// whose unknown-qualifier fail-closed keeps a printed OpponentOf spec silent.
func (e *Engine) monarchPlayerAdmits(spec string, source state.ObjID, p state.PlayerID) bool {
	if e.matchAs != nil && source == e.matchAs.source {
		if base, qualifier, _ := strings.Cut(spec, "."); (base == "Player" || base == "Any") &&
			strings.TrimSpace(qualifier) == "OpponentOf Remembered" {
			return opponentOfRemembered(e.G, e.matchAs.remembered, p)
		}
	}
	return effects.MatchesPlayerSpec(e.G, spec, p, e.controllerOf(source))
}

// opponentOfRemembered is the OpponentOf-Remembered read above: the triggering
// player p qualifies when it is an opponent of the controller of ANY object in
// the remembered list (CR 716.2's ComeBack reading: "until an opponent becomes
// the monarch" -- any player other than a remembered card's controller). An
// empty or unresolvable list matches nobody: the fail-closed direction.
func opponentOfRemembered(g *state.Game, remembered []state.ObjID, p state.PlayerID) bool {
	for _, id := range remembered {
		o := g.Obj(id)
		if o == nil {
			continue
		}
		if p != o.Controller {
			return true
		}
	}
	return false
}

func init() {
	registerTrigMatcher((*Engine).becomeMonarchMatches, "BecomeMonarch")
}
