package kgen

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
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

// youPlayerStatic lowers a Continuous static on Affected$ You: the
// unlimited / additive hand size, the extra land play, the player hexproof
// and the player shroud (lower_continuous.go's Affected$ You arm), then
// falls through to playerStatic for the two remaining player statics. It
// reports false when neither side lowers the shape; the caller refuses with
// the standing "player static" reason.
func (lc *lowerCtx) youPlayerStatic(sp *params, st *lStatic) (bool, error) {
	switch {
	case sp.is("SetMaxHandSize", "Unlimited"):
		st.Kind = "NoMaxHandSize"
	case sp.m["AdjustLandPlays"] != "":
		n, err := strconv.Atoi(sp.str("AdjustLandPlays"))
		if err != nil || n < 1 || n > 3 {
			return false, refuse("AdjustLandPlays %q", sp.str("AdjustLandPlays"))
		}
		st.Kind = "ExtraLandPlay"
		st.Amount = cst(n)
	case sp.m["RaiseMaxHandSize"] != "":
		// CR 402.2: the additive RaiseMaxHandSize$ statics (Minamo
		// Scrollkeeper's +1, Jin-Gitaxias's -7) apply ON TOP of the set
		// maximum, never below zero (gorge maxHandSizeFor). The kernel's
		// maxHand adds each StHandDelta static's signed amount after the
		// set-maximum walk (static_playerstats.go); gorge sums every
		// matching delta the same way.
		n, err := strconv.Atoi(sp.str("RaiseMaxHandSize"))
		if err != nil || n < -255 || n > 255 {
			return false, refuse("RaiseMaxHandSize %q", sp.str("RaiseMaxHandSize"))
		}
		st.Kind = "NoMaxHandSize"
		st.Amount = cst(n)
		st.Flags = append(st.Flags, "HandDelta")
	case sp.is("AddKeyword", "Hexproof"):
		st.Kind = "PlayerHexproof"
	case sp.is("AddKeyword", "Shroud"):
		// "You have shroud" (Ivory Mask, True Believer, Solitary
		// Confinement): CR 702.18, a player with shroud is an illegal
		// target for ANY spell or ability, their controller's included --
		// shroud is symmetric, unlike hexproof (gorge
		// playerShroudBlocksTarget). The kernel reads a
		// StatPlayerHexproof with A == 2 in the two player-target gates
		// (legal.go targetOK, resolve.go targetStillLegal); seat-exact: A is
		// the shroud bit, not a seat count, and every player is covered.
		st.Kind = "PlayerShroud"
	case sp.is("AddKeyword", plKwDefenderAssigns):
		// Defensive Formation: gorge defenderAssignsAttackerDamage; the
		// kernel's bandDivider reads it (combat_band.go)
		st.Kind = "DefenderAssigns"
	default:
		return lc.playerStatic(sp, st)
	}
	return true, nil
}

// The one player keyword the kernel reads (kop.StatDefenderAssigns). The
// others a Continuous `Affected$ You` static carries (AdditionalVote,
// AdditionalOptionalVote, UntapAdjust, the echo-waiver sentence) have no
// gorge reader, so lowering them would copy a gorge gap. K'rrik's
// PayLifeInsteadOf:B has one but the kernel does not model its payment yet
// (its granted {B} pips: offer faces, the shortfall payment, the mana
// window's done): both stay refused ("player static").
const plKwDefenderAssigns = "You assign combat damage of each creature attacking you."

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

// banOppFlag reads the one flag a CantGainLife / CantDraw static carries:
// "BanOpp" (ValidPlayer$ Opponent) sets kop.StBanOpp.
func banOppFlag(flags []string, st *kop.Static) error {
	for _, fl := range flags {
		if fl != "BanOpp" {
			return fmt.Errorf("CantGainLife/CantDraw flag %q", fl)
		}
		st.StFlags |= kop.StBanOpp
	}
	return nil
}
