package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// flipCoinModStatic lowers the canonical FlipCoinMod static (Edgar, King of
// Figaro's Two-Headed Coin): ValidPlayer$ You, CheckSVar$
// Count$YouFlipThisTurn, SVarCompare$ EQ0 and Result$ True. The kernel's
// flip site reads the controller and the card, and the turn bit it keeps
// replaces the CheckSVar (dirac/turn_facts.go flipForced). Any other shape
// refuses by name rather than force a flip the card does not grant.
func (lc *lowerCtx) flipCoinModStatic(s cards.Static) error {
	sp := newParams(s.Params)
	sp.consume("Mode")
	if v := strings.TrimSpace(sp.str("ValidPlayer")); v != "You" {
		return refuse("FlipCoinMod ValidPlayer %q (only You is lowered)", v)
	}
	if v := strings.TrimSpace(sp.str("CheckSVar")); v != "Count$YouFlipThisTurn" {
		return refuse("FlipCoinMod CheckSVar %q (only Count$YouFlipThisTurn is lowered)", v)
	}
	if v := strings.TrimSpace(sp.str("SVarCompare")); v != "EQ0" {
		return refuse("FlipCoinMod SVarCompare %q (only EQ0 is lowered)", v)
	}
	if v := strings.TrimSpace(sp.str("Result")); v != "True" {
		return refuse("FlipCoinMod Result %q (only True is lowered)", v)
	}
	sp.consume("ValidPlayer", "CheckSVar", "SVarCompare", "Result", "Description")
	if left := sp.unread(); len(left) > 0 {
		return refuse("FlipCoinMod unread params %v", left)
	}
	lc.out.Statics = append(lc.out.Statics, lStatic{Kind: "FlipCoinMod"})
	return nil
}
