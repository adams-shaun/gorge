package kgen

import (
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
