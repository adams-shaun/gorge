package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// hasCardsValue lowers <group>$HasPropertyHasCardsIn{Hand,Graveyard}_Card_<op><bound>
// (gorge count_player.go:459-500): the number of group members whose own
// Hand / Graveyard holds a count of cards compared true against the bound.
// The bound is an integer or a printed SVar read once from the source's
// controller (parseCountCompareOrSVar), so it is never per-member. Two
// players: PlayerCount$ / PlayerCountPlayers$ = {you, opponent},
// PlayerCountOpponents$ = {opponent}. ok = false: not this head.
func (lc *lowerCtx) hasCardsValue(group, rest string) (v lValue, ok bool, err error) {
	var graveyard bool
	switch {
	case strings.HasPrefix(rest, "HasPropertyHasCardsInHand_"):
		rest = strings.TrimPrefix(rest, "HasPropertyHasCardsInHand_")
	case strings.HasPrefix(rest, "HasPropertyHasCardsInGraveyard_"):
		rest = strings.TrimPrefix(rest, "HasPropertyHasCardsInGraveyard_")
		graveyard = true
	default:
		return lValue{}, false, nil
	}
	switch group {
	case "PlayerCountOpponents", "PlayerCount", "PlayerCountPlayers":
	default:
		return lValue{}, true, refuse("HasCardsIn group %s", group)
	}
	i := strings.LastIndex(rest, "_")
	if i < 0 {
		return lValue{}, true, refuse("HasCardsIn spec %q", rest)
	}
	spec, cmp := rest[:i], rest[i+1:]
	if spec != "Card" {
		return lValue{}, true, refuse("HasCardsIn spec %q", spec)
	}
	if len(cmp) < 3 {
		return lValue{}, true, refuse("HasCardsIn compare %q", cmp)
	}
	var op uint8
	switch cmp[:2] {
	case "GE":
		op = opGE
	case "GT":
		op = opGT
	case "LE":
		op = opLE
	default:
		return lValue{}, true, refuse("HasCardsIn op %q", cmp[:2])
	}
	var bound lValue
	if n, aerr := strconv.Atoi(cmp[2:]); aerr == nil {
		bound = cst(n)
	} else {
		body, has := lc.face.SVars[cmp[2:]]
		if !has {
			return lValue{}, true, refuse("HasCardsIn bound %q names no SVar", cmp[2:])
		}
		bv, berr := lc.svarValue(body, 1)
		if berr != nil {
			return lValue{}, true, berr
		}
		bound = bv
	}
	// each member of the group compared on its own (gorge
	// hasPropertyStateBacked: hasPropertyCount over the living group), so a
	// pod counts every opponent, not one (kop.OppExtCountCmp)
	n := kop.OppExtCountCmp | kop.OppExtHand
	if graveyard {
		n = kop.OppExtCountCmp | kop.OppExtGraveAll
	}
	if group != "PlayerCountOpponents" {
		n |= kop.OppExtAllPl
	}
	return lValue{Kind: vOppExtreme, N: n, Op: op, Sub: []lValue{bound}}, true, nil
}

// zeroOne reports whether v is provably 0 or 1: the opponents-who-lost-life
// count in a two-player game (twoSeat-at-caller: lower_keywordaction.go marks
// that arm), or a vCompare whose arms are the constants 0
// and 1, with no Mul/Div/Add/Neg applied on top.
func zeroOne(v lValue) bool {
	if v.Mul != 0 || v.Div != 0 || v.Add != 0 || v.Neg {
		return false
	}
	switch v.Kind {
	case vOppsLostLife:
		return true
	case vCompare:
		if len(v.Sub) != 2 || v.Sub[1].Kind != vBranches || len(v.Sub[1].Sub) != 2 {
			return false
		}
		a, b := v.Sub[1].Sub[0], v.Sub[1].Sub[1]
		return a.isConst() && b.isConst() && (a.N == 0 && b.N == 1 || a.N == 1 && b.N == 0)
	}
	return false
}
