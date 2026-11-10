package kgen

import (
	"sort"
	"strconv"
	"strings"
)

// the two `Affected$ You` player statics beyond
// Unlimited hand size, hexproof and extra land plays.
//
//   - SetMaxHandSize$ <N> (The Ten Rings 10, Doctor Octopus 8): gorge's
//     maxHandSizeFor reads the value through effects.HandSizeValueOK, whose
//     integer arm is a plain non-negative decimal. The static lowers to
//     NoMaxHandSize with Amount N (build_ab.go: Static.A = N+1, 0 stays
//     "unlimited"). An SVar-named value is the printed route's own pricing
//     (setMaxHandSizeSVar) and stays refused.
//   - AddKeyword$ Protection:Player.Opponent:each of your opponents
//     (Absolute Virtue): gorge's playerProtectedFrom holds against a source
//     whose controller is an opponent of the protected player. In two
//     players that is the set playerHexproofBlocksTarget withholds, so the
//     static lowers to PlayerHexproof with Amount 1 (the protection bit
//     the kernel's replDamage reads: static_playerstats.go).

// protOpponent is the one Protection spelling the kernel prices.
const protOpponent = "Protection:Player.Opponent:each of your opponents"

// oppMaxHand lowers an `Affected$ Opponent` SetMaxHandSize$ naming an SVar
// (Winter, Misanthropic Guide: Y = Number$7/Minus.X, X the controller's
// graveyard card types) to NoMaxHandSize with a dynamic amount and the
// "HandOpp" flag (kop.StHandOpp: it binds the controller's opponents, not the
// controller). gorge setMaxHandSizeSVar prices the SVar from the source's
// controller; the Condition$ gate rides st.Cond (Static.Cond2). A literal or
// Unlimited value, or any other param beside it, stays refused.
func (lc *lowerCtx) oppMaxHand(sp *params, st *lStatic) error {
	// the other Continuous readers (AddKeyword$ and kin) read before this
	// point, so a param beside the hand size would be dropped silently
	var extra []string
	for k := range sp.m {
		switch k {
		case "Mode", "Affected", "SetMaxHandSize", "Condition", "Description":
		default:
			extra = append(extra, k)
		}
	}
	if sort.Strings(extra); len(extra) > 0 {
		return refuse("SetMaxHandSize for opponents beside %s$", extra[0])
	}
	raw := strings.TrimSpace(sp.str("SetMaxHandSize"))
	if _, err := strconv.Atoi(raw); err == nil || raw == "Unlimited" || raw == "" {
		return refuse("SetMaxHandSize %q for opponents", raw)
	}
	v, err := lc.value(raw)
	if err != nil {
		return refuse("SetMaxHandSize %q for opponents: %v", raw, err)
	}
	if v.isConst() {
		return refuse("SetMaxHandSize %q for opponents: constant", raw)
	}
	st.Kind = "NoMaxHandSize"
	st.Amount = v
	st.Flags = append(st.Flags, "HandOpp")
	return nil
}

// playerStatic lowers the numeric hand size and the opponent protection.
// It reports false when sp carries neither, leaving the caller's refusal.
func (lc *lowerCtx) playerStatic(sp *params, st *lStatic) (bool, error) {
	if raw, ok := sp.get("SetMaxHandSize"); ok {
		raw = strings.TrimSpace(raw)
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 || n > 255 || strconv.Itoa(n) != raw {
			return false, refuse("SetMaxHandSize %q", raw)
		}
		st.Kind = "NoMaxHandSize"
		st.Amount = cst(n)
		return true, nil
	}
	if sp.is("AddKeyword", protOpponent) {
		st.Kind = "PlayerHexproof"
		st.Amount = cst(1)
		return true, nil
	}
	return false, nil
}
