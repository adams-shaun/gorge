package kgen

import "strings"

// Gemstone Caverns stays refused, by name, on its opening-hand node; the reason
// below says what the kernel lacks (a ticket of its own).
//
// Its opening-hand keyword (CR 103.6a: "if this is in your opening hand and
// you're not the starting player, you may begin the game with it on the
// battlefield with a luck counter on it; if you do, exile a card from your
// hand") is gorge's Gemstone arm of rules/opening_hand.go: the !PlayFirst gate
// skips the starting player (newOpeningRound), the luck counter is put by the
// pregame PutCounter sub, and the mandatory hand exile is a KChoose over the
// hand (opening_exile). The kernel's Leyline ask (turn_gen.go leylineStart,
// CardDef.Leyline) only moves a card from hand to battlefield, for every
// player: no gate, no counter, no follow-up card choice.
//
// Its mana ability lowers (lower_condsplit.go: a Condition-split mana source,
// as River of Tears'); only the opening-hand node below stays refused.

// refuseOpeningGated is the refusal of a MayEffectFromOpeningHand keyword
// whose action is not the plain FromHand self-to-battlefield move.
func refuseOpeningGated(param string) string {
	if _, req, _ := strings.Cut(param, ":"); strings.Contains(req, "!PlayFirst") {
		return "opening-hand effect gated on not being the starting player (!PlayFirst; Gemstone Caverns' luck counter and mandatory hand exile): the kernel's Leyline ask only moves the card, for every player"
	}
	return "opening-hand effect " + strings.TrimSpace(param) + ": the kernel's Leyline ask models only the plain FromHand self-to-battlefield move"
}
