package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/state"
)

// effectDrawKeys are the Event$ Draw line's own parameters the two live
// registrations read: the matcher (Event$, ValidPlayer$), the body pointer
// and the prose. A line carrying any other parameter (Urabrask, Heretic
// Praetor's IsPresent$/PresentZone$/PresentCompare$ remembered-set gate) is
// refused registration outright and keeps its loud Note.
var effectDrawKeys = map[string]bool{
	"Event": true, "ValidPlayer": true, "ReplaceWith": true, "Description": true,
}

// effectDrawBodyLive reports whether an Effect-delivered Event$ Draw
// replacement is one of the two shapes this build's Draw dispatch resolves
// live. Admission is by shape, never by name, and the line may carry no
// parameter outside effectDrawKeys.
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
//     controller draws instead. An open-ended "every draw" hook whose
//     remembered set must GROW while the body runs (Urabrask's IsPresent$
//     Card.IsRemembered gate, whose remembered list the registration cannot
//     grow, and whose DB$ Dig body is not the redirect) keeps its loud Note
//     until the effect-scoped remembered-set ticket lands.
func effectDrawBodyLive(hasTriggers bool, params map[string]string, body string, svars map[string]string, rememberedPlayers []state.PlayerID) bool {
	if hasTriggers || body == "" {
		return false
	}
	for k := range params {
		if !effectDrawKeys[k] {
			return false
		}
	}
	switch strings.TrimSpace(params["ValidPlayer"]) {
	case "You":
		return effectBodyEndsInSelfExile(body, svars)
	case "Player.IsRemembered":
		return len(rememberedPlayers) > 0 && effectDrawBodyRedirectsToYou(body)
	}
	return false
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
