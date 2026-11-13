package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// CastNameKeyedSelfMill reports whether f's spell moves cards off the top of
// its caster's own library into exile or the graveyard until a card matches
// a NAME, with no other bound: the dig stops at one named card, or at a
// second card sharing a name with one already exiled, and otherwise runs
// the library out. Two shapes, read off the spell's ability chain:
//
//   - a DigUntil over the caster's own library whose Valid$ is a chosen
//     name (Card.NamedCard) and whose revealed cards go to exile or the
//     graveyard (Demonic Consultation, Spoils of the Vault);
//   - a Repeat (not RepeatEach, not RepeatOptional, no MaxRepeat$) whose
//     body digs the caster's own library to exile or the graveyard and
//     whose stop reads a shared name (Tainted Pact's sharesNameWith).
//
// A dig that stops at a card TYPE (Primal Surge's nonpermanent, Rally the
// Horde's nonland) is not in the class: an ordinary deck holds that card a
// few cards down, so the library is not at stake. Nil-safe; false for a face
// with no spell ability.
func CastNameKeyedSelfMill(f *cards.Face) bool {
	if f == nil {
		return false
	}
	sa := f.SpellAbility()
	for n, depth := sa, 0; n != nil && depth < 16; n, depth = n.Sub, depth+1 {
		switch n.API {
		case "DigUntil":
			if ownLibraryDig(n) && strings.Contains(n.Params["Valid"], "NamedCard") &&
				libraryLossZone(n.Params["RevealedDestination"], "Graveyard") {
				return true
			}
		case "Repeat":
			if n.Params["RepeatOptional"] != "" || n.Params["MaxRepeat"] != "" {
				continue
			}
			body := cards.ResolveSVar(f.SVars, n.Params["RepeatSubAbility"])
			if body == nil || !chainDigsOwnLibraryAway(body) {
				continue
			}
			if nameKeyedStop(f, n, body) {
				return true
			}
		}
	}
	return false
}

// ownLibraryDig reports whether a Dig or DigUntil reads its caster's own
// library: no target, and Defined$ empty or You.
func ownLibraryDig(sa *cards.SA) bool {
	if sa.Params["ValidTgts"] != "" || sa.Params["TgtPrompt"] != "" {
		return false
	}
	d := sa.Params["Defined"]
	return d == "" || d == "You"
}

// libraryLossZone reports whether zone (or def when the param is unset) is
// exile or the graveyard: a card sent there has left the library for good.
func libraryLossZone(zone, def string) bool {
	if zone == "" {
		zone = def
	}
	return zone == "Exile" || zone == "Graveyard"
}

// chainDigsOwnLibraryAway reports whether body's chain holds a Dig of the
// caster's own library whose cards go to exile or the graveyard.
func chainDigsOwnLibraryAway(body *cards.SA) bool {
	for n, depth := body, 0; n != nil && depth < 16; n, depth = n.Sub, depth+1 {
		if n.API == "Dig" && ownLibraryDig(n) && libraryLossZone(n.Params["DestinationZone"], "") {
			return true
		}
	}
	return false
}

// nameKeyedStop reports whether a Repeat's stop reads a shared name: its
// own RepeatPresent$, or a body step's ConditionCheckSVar$ whose SVar does.
func nameKeyedStop(f *cards.Face, rep, body *cards.SA) bool {
	if strings.Contains(rep.Params["RepeatPresent"], "sharesNameWith") {
		return true
	}
	for n, depth := body, 0; n != nil && depth < 16; n, depth = n.Sub, depth+1 {
		if v := n.Params["ConditionCheckSVar"]; v != "" && strings.Contains(f.SVars[v], "sharesNameWith") {
			return true
		}
	}
	return false
}

// Empty-library win kinds (EmptyLibraryWin).
const (
	// EmptyLibraryWinNone: the face never wins on an empty library.
	EmptyLibraryWinNone uint8 = iota
	// EmptyLibraryWinETB: a triggered WinsGame whose condition counts the
	// controller's library (Thassa's Oracle's enters trigger): it wins when
	// the card is cast and enters, not while it sits on the battlefield.
	EmptyLibraryWinETB
	// EmptyLibraryWinDraw: a draw replacement that wins instead (Laboratory
	// Maniac, Jace, Wielder of Mysteries): it wins from the battlefield at
	// the next draw from an empty library.
	EmptyLibraryWinDraw
)

// EmptyLibraryWin classifies how f wins the game for a player whose library
// is empty (the kinds above), from its IR: a Draw replacement whose body
// WinsGame, or a trigger whose chain WinsGame on a condition that counts a
// library (Count$ValidLibrary). Nil-safe.
func EmptyLibraryWin(f *cards.Face) uint8 {
	if f == nil {
		return EmptyLibraryWinNone
	}
	for i := range f.Repls {
		r := &f.Repls[i]
		if r.Event == "Draw" && chainWins(f, r.With, false) {
			return EmptyLibraryWinDraw
		}
	}
	for i := range f.Triggers {
		if chainWins(f, f.Triggers[i].Effect, true) {
			return EmptyLibraryWinETB
		}
	}
	return EmptyLibraryWinNone
}

// chainWins reports whether sa's chain holds a WinsGame for its controller;
// with libraryCond, only one whose ConditionCheckSVar$ counts a library.
func chainWins(f *cards.Face, sa *cards.SA, libraryCond bool) bool {
	for n, depth := sa, 0; n != nil && depth < 16; n, depth = n.Sub, depth+1 {
		if n.API != "WinsGame" {
			continue
		}
		if d := n.Params["Defined"]; d != "" && d != "You" {
			continue
		}
		if !libraryCond {
			return true
		}
		if v := n.Params["ConditionCheckSVar"]; v != "" && strings.Contains(f.SVars[v], "ValidLibrary") {
			return true
		}
	}
	return false
}
