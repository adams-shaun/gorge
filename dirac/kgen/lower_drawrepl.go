package kgen

import (
	"strconv"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// drawRepl lowers the controller's "if you would draw a card, draw N cards
// instead" replacement (gorge applyReplacements, a Draw event whose
// ReplaceWith$ body is exactly DB$ Draw | Defined$ You | NumCards$ N):
// Event$ Draw | ActiveZones$ Battlefield | ValidPlayer$ You, optionally
// NotFirstCardInDrawStep$ True (the turn-based draw is exempt). Every other
// Draw shape (an optional body, a prevention, a choice body, a condition
// the kernel does not read) refuses.
func (lc *lowerCtx) drawRepl(rp *params, w *cards.SA, rl *lRepl) error {
	if rp.is("Optional", "True") || w != nil && w.API == "GenericChoice" {
		// the player is asked whether to replace the draw, then which kind
		// of card to dig for: two asks the kernel does not pose at a draw
		return refuse("repl-hard: Draw replacement with an optional or choice body")
	}
	if rp.str("ActiveZones") != "Battlefield" || rp.str("ValidPlayer") != "You" || w == nil ||
		!withIs(w, "Draw", map[string]string{"Defined": "You", "NumCards": w.Params["NumCards"]}) {
		return refuse("repl-hard: Draw replacement")
	}
	n, err := strconv.Atoi(w.Params["NumCards"])
	if err != nil || n < 2 || n > 8 {
		return refuse("repl-hard: Draw replacement NumCards$ %q", w.Params["NumCards"])
	}
	rl.Kind, rl.N = "DrawMore", cst(n)
	if rp.is("NotFirstCardInDrawStep", "True") {
		rl.Bits = kop.ReplDrawNotFirst
	}
	return nil
}
