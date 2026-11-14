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
// opponent recorded unasked), so the chain lowers at any seat count. The
// lowering is seat-exact: the ask and the readers hold with any number of
// players, and no lc.twoSeat call is made here. The readers:
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
func (lc *lowerCtx) chosenCtrlImplied(ps *params, origin int, ct string) string {
	if lc.chosenMidAb == nil || lc.chosenMidAb != lc.ab || origin != zGraveyard {
		return ct
	}
	switch strings.TrimSpace(ps.m["DefinedPlayer"]) {
	case "ChosenPlayer", "Player.Chosen":
	default:
		return ct
	}
	if !strings.Contains(ct, "ChosenCtrl") {
		return ct
	}
	alts := strings.Split(ct, ",")
	for i, p := range alts {
		w, prop, ok := strings.Cut(strings.TrimSpace(p), ".")
		if !ok || prop != "ChosenCtrl" || strings.Contains(w, ".") {
			return ct
		}
		alts[i] = w
	}
	return strings.Join(alts, ",")
}

// chosenMidPool decides a hidden graveyard -> hand pick's pool while this
// ability's mid-resolution ChoosePlayer is armed (Skullwinder: DefinedPlayer$
// ChosenPlayer, Chooser$ ChosenPlayer): the pool is the chosen player's own
// graveyard and they pick from it, so the pick's clauses need no owner of
// their own. It consumes DefinedPlayer$ and returns the ChooseMove's player
// Ref (the chosen player, else the controller) and whether the pool is the
// chosen player's.
func (lc *lowerCtx) chosenMidPool(ps *params, dest int) (lRef, bool, error) {
	owner := lRef{Kind: rYou}
	if lc.chosenMidAb == nil || lc.chosenMidAb != lc.ab || dest != zHand {
		return owner, false, nil
	}
	switch strings.TrimSpace(ps.m["DefinedPlayer"]) {
	case "ChosenPlayer", "Player.Chosen":
		ps.consume("DefinedPlayer")
		if ch, ok := ps.get("Chooser"); ok && strings.TrimSpace(ch) != "ChosenPlayer" {
			return owner, false, refuse("graveyard choice Chooser$ %q", ch)
		}
		return lRef{Kind: rChosenPlayer}, true, nil
	}
	return owner, false, nil
}
