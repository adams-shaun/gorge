package effects

import "strings"

// effectDrawKeys are the Event$ Draw line's own parameters the live "next
// time you would draw" registration reads: the matcher (Event$,
// ValidPlayer$), the body pointer and the prose.
var effectDrawKeys = map[string]bool{
	"Event": true, "ValidPlayer": true, "ReplaceWith": true, "Description": true,
}

// effectDrawBodyLive reports whether an Effect-delivered Event$ Draw
// replacement is the one-shot "the next time you would draw a card this turn,
// <body> instead" family (Words of Worship/War/Waste/Wilding/Wind, Aladdin's
// Lamp, Ring of Ma'ruf): the shared Draw matcher in engine/rules picks the
// registration up like a printed R: line, runs the body under the Effect's
// source, and the body's own self-exile tail (a ChangeZone of the Effect from
// the command zone, ends the registration in zone.go) retires it after one use.
//
// Admission is by shape, never by name: the line names only its own player
// (ValidPlayer$ You, no IsPresent$ gate), the Effect carries no Triggers$ of
// its own (Mangara's Tome's leave-play bookkeeping and RememberedFirst pile
// are unmodelled, so its draw would be swallowed with no card moved), and the
// body chain ends in the self-exile. An open-ended "every draw" body
// (Urabrask's Player.IsRemembered, IsPresent$ gate, no self-exile) keeps its
// loud Note: registered live it would replace the draw forever.
func effectDrawBodyLive(hasTriggers bool, params map[string]string, body string, svars map[string]string) bool {
	if hasTriggers || body == "" || strings.TrimSpace(params["ValidPlayer"]) != "You" {
		return false
	}
	for k := range params {
		if !effectDrawKeys[k] {
			return false
		}
	}
	return effectBodyEndsInSelfExile(body, svars)
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
