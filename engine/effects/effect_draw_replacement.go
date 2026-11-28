package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/state"
)

// effectDrawKeys are the Event$ Draw line's own parameters the live
// registrations read: the matcher (Event$, ValidPlayer$), the body pointer
// and the prose. The remembered-set gate family (IsPresent$/PresentZone$/
// PresentCompare$) is admitted only in the one arm that reads the effect-
// scoped group list (effectDrawGroupKeys below); any other parameter keeps
// the line refused outright with its loud Note.
var effectDrawKeys = map[string]bool{
	"Event": true, "ValidPlayer": true, "ReplaceWith": true, "Description": true,
}

// effectDrawGroupKeys is effectDrawKeys widened by exactly the
// IsPresent$/PresentZone$/PresentCompare$ gate family, for the
// effect-scoped-remembered-set arm whose gate reads the ACTIVATION's
// remembered list (rules' RememberOnEffect grows it), not the source
// object's.
var effectDrawGroupKeys = map[string]bool{
	"Event": true, "ValidPlayer": true, "ReplaceWith": true, "Description": true,
	"IsPresent": true, "PresentZone": true, "PresentCompare": true,
}

// effectDrawBodyLive reports whether an Effect-delivered Event$ Draw
// replacement is one of the three shapes this build's Draw dispatch resolves
// live. Admission is by shape, never by name, and the line may carry no
// parameter outside its arm's whitelist.
//
//   - ValidPlayer$ You: the one-shot "the next time you would draw a card
//     this turn, <body> instead" family (Words of Worship/War/Waste/Wilding/
//     Wind, Aladdin's Lamp, Ring of Ma'ruf). The Effect carries no Triggers$
//     of its own (Mangara's Tome's leave-play bookkeeping and RememberedFirst
//     pile are unmodelled, so its draw would be swallowed with no card
//     moved), and the body's own self-exile tail (a ChangeZone of the Effect
//     from the command zone, ends the registration in zone.go) retires it
//     after one use.
//   - ValidPlayer$ Player.IsRemembered: Plagiarize's redirect -- the
//     registration must already carry remembered players (the DB$ Effect's
//     RememberObjects$ capture, which the Draw matcher's ValidPlayer$ gate
//     reads), and the body is a single DB$ Draw | Defined$ You: the
//     remembered player's draw is prevented and the registration's frozen
//     controller draws instead.
//   - ValidPlayer$ Player.IsRemembered with the IsPresent$ Card.IsRemembered
//     | PresentZone$ Exile | PresentCompare$ EQ0 gate and a single
//     DB$ Dig | Defined$ ReplacedPlayer | DestinationZone$ Exile |
//     RememberChanged$ True body: Urabrask, Heretic Praetor's open-ended
//     "the next time they would draw a card this turn, instead they exile
//     the top card of their library". The EQ0 gate reads the activation's
//     own remembered list (state.ContinuousEffect.EffectGroup), which the
//     body grows in place through Host.RememberOnEffect -- the gate flips
//     the moment the card is exiled, spending the one-shot inside the
//     registration's this-turn duration.
func effectDrawBodyLive(hasTriggers bool, params map[string]string, body string, svars map[string]string, rememberedPlayers []state.PlayerID) bool {
	if hasTriggers || body == "" {
		return false
	}
	switch strings.TrimSpace(params["ValidPlayer"]) {
	case "You":
		return drawParamsIn(params, effectDrawKeys) && effectBodyEndsInSelfExile(body, svars)
	case "Player.IsRemembered":
		if len(rememberedPlayers) == 0 {
			return false
		}
		if effectDrawBodyRedirectsToYou(body) {
			return drawParamsIn(params, effectDrawKeys)
		}
		return effectDrawBodyExilesRememberedPlayerTopcard(params, body)
	}
	return false
}

// drawParamsIn reports whether every parameter the line carries is on the
// arm's whitelist.
func drawParamsIn(params map[string]string, allowed map[string]bool) bool {
	for k := range params {
		if !allowed[k] {
			return false
		}
	}
	return true
}

// effectDrawBodyExilesRememberedPlayerTopcard admits the effect-scoped
// remembered-set shape: the gate must spell the one-shot exactly (an
// IsPresent$ Card.IsRemembered over the exile zone, compared EQ0 -- any
// other spelling would read a list this mechanism does not grow), and the
// body must be the single DB$ Dig that exiles the would-be drawer's library
// top and remembers it (RememberChanged$ True) into that same list. The
// remembered-players capture (ValidPlayer$ gate + the body's Condition-
// Defined$ Remembered reading it) is the caller's precondition.
func effectDrawBodyExilesRememberedPlayerTopcard(params map[string]string, body string) bool {
	if !drawParamsIn(params, effectDrawGroupKeys) {
		return false
	}
	if strings.TrimSpace(params["IsPresent"]) != "Card.IsRemembered" ||
		strings.TrimSpace(params["PresentZone"]) != "Exile" ||
		strings.TrimSpace(params["PresentCompare"]) != "EQ0" {
		return false
	}
	if replacementBodyAPI(body) != "Dig" {
		return false
	}
	p := replacementBodyParams(body)
	return p["Defined"] == "ReplacedPlayer" && p["DestinationZone"] == "Exile" &&
		p["RememberChanged"] == "True" && p["SubAbility"] == ""
}

// effectDrawBodyRedirectsToYou reports whether an Event$ Draw body is the
// Plagiarize redirect: a single DB$ Draw | Defined$ You, no SubAbility chain.
// The Draw dispatch prevents the remembered player's draw, and this body's
// own draw (run under the registration's frozen controller, replCtx's
// frozenController) is the only card move: nothing can be lost -- the
// replaced draw never takes a card, and the body draws from its own
// controller's library.
func effectDrawBodyRedirectsToYou(body string) bool {
	if replacementBodyAPI(body) != "Draw" {
		return false
	}
	p := replacementBodyParams(body)
	return p["Defined"] == "You" && p["SubAbility"] == ""
}

// effectBodyEndsInSelfExile walks a replacement body's SubAbility$ chain and
// reports whether it reaches the one-shot idiom DB$ ChangeZone | Defined$ Self
// | Origin$ Command | Destination$ Exile. The walk is bounded so a cyclic
// script cannot hang a registration.
func effectBodyEndsInSelfExile(body string, svars map[string]string) bool {
	for range 8 {
		p := replacementBodyParams(body)
		if replacementBodyAPI(body) == "ChangeZone" && p["Defined"] == "Self" &&
			p["Origin"] == "Command" && p["Destination"] == "Exile" {
			return true
		}
		next := p["SubAbility"]
		if next == "" || svars[next] == "" {
			return false
		}
		body = svars[next]
	}
	return false
}
