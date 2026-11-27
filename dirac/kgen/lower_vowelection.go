package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// Promise of Loyalty's election (RepeatEach over every player): the repeated
// player picks one creature they control, which takes the vow counter and is
// remembered (a PutCounter with Choices$ and Chooser$ / Placer$
// Player.IsRemembered, gorge counters_put.go putCounterChooseTitled), and
// sacrifices the rest (a SacrificeAll of the creatures the pick did not
// remember, controlled by the repeated player). The two nodes lower inside a
// RepeatEach player body only in these exact spellings; the pick reads its
// own parameters (putCounterChoices) and the sacrifice's filter names the
// repeated player as RememberedPlayerCtrl does.

// vowChoicesSpec and vowSacrificeSpec are the two spellings.
const (
	vowChoicesSpec   = "Creature.ControlledBy Player.IsRemembered"
	vowSacrificeSpec = "Creature.!IsRemembered+ControlledBy Player.IsRemembered"
)

// vowElectionNode reports whether n is one of the election's two nodes.
func vowElectionNode(n *cards.SA) bool {
	switch n.API {
	case "PutCounter":
		return strings.TrimSpace(n.Params["Choices"]) == vowChoicesSpec &&
			strings.TrimSpace(n.Params["Chooser"]) == "Player.IsRemembered" &&
			strings.TrimSpace(n.Params["Placer"]) == "Player.IsRemembered"
	case "SacrificeAll":
		return strings.TrimSpace(n.Params["ValidCards"]) == vowSacrificeSpec
	}
	return false
}
