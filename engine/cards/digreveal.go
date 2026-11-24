package cards

import "strings"

// DigRevealsPick reports whether a Dig reveals the cards it moves from the
// window (CR 701.20a: a revealed card is shown to all players).
//
// Forge's DigEffect (forge-game DigEffect.java, the pick arm) ends the pick
// with
//
//	if (!changeValid.isEmpty() && !sa.hasParam("ExileFaceDown") && !sa.hasParam("NoReveal")) {
//	    game.getAction().reveal(movedCards, chooser, true, ...lblPlayerPickedCardFrom...);
//	}
//
// and the old corpus spelled the same fact on the script as
// ForceRevealToController$ True. The new corpus dropped that spelling from
// about 118 scripts ("You may reveal a land card from among them"), so the
// rule is read from the ability's own shape and words: a ChangeValid$ pick,
// neither NoReveal$ True nor ExileFaceDown$, whose card's Oracle text has a
// sentence that reveals "from among" the window. Impulse-style digs, which
// Forge's quirk also reveals but whose text never does, stay private. An
// explicit ForceRevealToController$ True is read separately by the caller.
func DigRevealsPick(params map[string]string, oracle string) bool {
	if params["ChangeValid"] == "" || params["NoReveal"] == "True" {
		return false
	}
	if _, fd := params["ExileFaceDown"]; fd {
		return false
	}
	return OracleRevealsFromAmong(oracle)
}

// OracleRevealsFromAmong reports whether some sentence of oracle both reveals
// and names "from among" (the dug window): "You may reveal a land card from
// among them and put it into your hand."
func OracleRevealsFromAmong(oracle string) bool {
	low := strings.ToLower(oracle)
	for _, s := range strings.Split(low, ". ") {
		if strings.Contains(s, "reveal") && strings.Contains(s, "from among") {
			return true
		}
	}
	return false
}
