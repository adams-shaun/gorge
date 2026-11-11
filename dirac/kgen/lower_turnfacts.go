package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// flipCoinModStatic lowers the canonical FlipCoinMod static (Edgar, King of
// Figaro's Two-Headed Coin): ValidPlayer$ You, CheckSVar$
// Count$YouFlipThisTurn, SVarCompare$ EQ0 and Result$ True. The kernel's
// flip site reads the controller and the card, and the turn bit it keeps
// replaces the CheckSVar (dirac/static_turnbits.go flipForced). Any other shape
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

// activationLimitStatic lowers the one Activations shape the kernel reads
// (Elvish Refueler): Exhaust abilities on the controller's own turn while the
// turn's Exhaust activations (st.Cond, already lowered from CheckSVar) are
// below one, MinLimit$ -1 (gorge legal_loyalty.go additionalActivationLimit's
// unbounded arm). The kernel reads the turn bit and the active player
// directly (dirac/static_turnbits.go exhaustOnceWaived); the lowered
// condition keeps the same expression for any other reader of the static.
// Any other shape refuses by name.
func activationLimitStatic(s cards.Static, sp *params, st *lStatic) error {
	if v := strings.TrimSpace(sp.str("ValidSA")); v != "Activated.Exhaust" {
		return refuse("Activations ValidSA %q (only Activated.Exhaust is lowered)", v)
	}
	if v := strings.TrimSpace(sp.str("ValidPlayer")); v != "You" {
		return refuse("Activations ValidPlayer %q (only You is lowered)", v)
	}
	if v := strings.TrimSpace(sp.str("MinLimit")); v != "-1" {
		return refuse("Activations MinLimit %q (only the unbounded -1 is lowered)", v)
	}
	if v := strings.TrimSpace(sp.str("PlayerTurn")); v != "You" {
		return refuse("Activations PlayerTurn %q (only You is lowered)", v)
	}
	sp.consume("ValidSA", "ValidPlayer", "MinLimit", "PlayerTurn")
	if st.Cond.Kind != cValue || st.Cond.A.Kind != vThisTurnActExhaust || st.Cond.Op != opLT ||
		!st.Cond.B.isConst() || st.Cond.B.N != 1 {
		return refuse("Activations CheckSVar %q (only Count$ThisTurnActivated_Activated.Exhaust+YouCtrl LT1 is lowered)", s.Params["CheckSVar"])
	}
	st.Cond = andCond(st.Cond, lCond{Kind: cYourTurn})
	st.Kind = "ActivationLimit"
	return nil
}
