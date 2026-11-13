package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// The mid-resolution chosen opponent (Skullwinder: a battlefield trigger
// whose DB chain is `ChoosePlayer | Defined$ You | Choices$ Player.Opponent`
// and whose later links read the choice). gorge runs effChoosePlayer
// mid-resolution and the chain's readers resolve the choice while the
// resolution holds it; the kernel asks it too (OpExt ExtChoosePlayerMid, one
// opponent recorded unasked), so the chain lowers at any seat count instead
// of the two-seat path. The readers:
//
//   - Defined$ ChosenPlayer / Player.Chosen and DefinedPlayer$ the same:
//     kop.RefChosenPlayer
//   - Chooser$ ChosenPlayer on the chain's own pick whose DefinedPlayer$ is
//     ChosenPlayer: the zone's player is that player, the kernel pick's
//     default chooser, so the param lowers to nothing
//   - ChangeType$ <word>.ChosenCtrl on such a pick: every card of a player's
//     graveyard is controlled by them, so the property is implied and drops
//
// A chain whose readers are none of these keeps the two-seat path
// (lower_chooseplayer.go); their shapes are still unmodelled.

// chainReadsChosenPlayer reports whether the Sub chain after a ChoosePlayer
// node reads its choice in the spellings above.
func chainReadsChosenPlayer(n *cards.SA) bool {
	for ; n != nil; n = n.Sub {
		for _, k := range []string{"Defined", "DefinedPlayer"} {
			switch strings.TrimSpace(n.Params[k]) {
			case "ChosenPlayer", "Player.Chosen":
				return true
			}
		}
		if strings.TrimSpace(n.Params["Chooser"]) == "ChosenPlayer" {
			return true
		}
		if strings.Contains(n.Params["ChangeType"], "ChosenCtrl") {
			return true
		}
	}
	return false
}

// chosenPlayerMidDefined maps ChosenPlayer / Player.Chosen to the resolution's
// choice (RefChosenPlayer) while this ability's mid-resolution ChoosePlayer
// is armed.
func (lc *lowerCtx) chosenPlayerMidDefined(v string) (lRef, bool) {
	if lc.chosenMidAb == nil || lc.chosenMidAb != lc.ab {
		return lRef{}, false
	}
	if v == "ChosenPlayer" || v == "Player.Chosen" {
		return lRef{Kind: rChosenPlayer}, true
	}
	return lRef{}, false
}

// chosenCtrlImplied rewrites a pick's ChangeType$ while this ability's
// mid-resolution ChoosePlayer is armed and the pick's own player is the
// chosen player (DefinedPlayer$ ChosenPlayer) over Origin$ Graveyard: every
// card of a player's graveyard is controlled by them, so the ChosenCtrl
// property is implied and drops. Only the exact `<word>.ChosenCtrl`
// alternative is rewritten; anything else keeps the spec (and refuses, the
// property being otherwise unknown).
func (lc *lowerCtx) chosenCtrlImplied(ps *params, origin int, ct string) (string, bool) {
	if lc.chosenMidAb == nil || lc.chosenMidAb != lc.ab || origin != zGraveyard {
		return "", false
	}
	switch strings.TrimSpace(ps.m["DefinedPlayer"]) {
	case "ChosenPlayer", "Player.Chosen":
	default:
		return "", false
	}
	if !strings.Contains(ct, "ChosenCtrl") {
		return "", false
	}
	alts := strings.Split(ct, ",")
	for i, p := range alts {
		w, prop, ok := strings.Cut(strings.TrimSpace(p), ".")
		if !ok || prop != "ChosenCtrl" || strings.Contains(w, ".") {
			return "", false
		}
		alts[i] = w
	}
	return strings.Join(alts, ","), true
}
