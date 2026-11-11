package kgen

import (
	"strconv"
	"strings"
)

// oppCastCondValue lowers PlayerCountOpponents$ConditionGE<n>
// SpellsCastThisTurn: the number of the controller's living opponents who
// cast n or more spells this turn (gorge playerCountCondition over
// opponentGroup, each member's SpellsCastThisTurnBy, the per-caster
// Player.SpellsCastThisTurn tally). Mindbreak Trap's "if an opponent cast
// three or more spells this turn". In a two-player game the group is the one
// opponent, so the count is 1 when that opponent's spells this turn (the
// kernel's per-caster tally, ExThisTurnCast N 0) reach n, else 0. Any other
// Condition head (another operator, an SVar right-hand side, another member
// property) refuses by name.
func (lc *lowerCtx) oppCastCondValue(rest string) (lValue, error) {
	cond, ok := strings.CutPrefix(strings.TrimSpace(rest), "ConditionGE")
	lit, prop, ok2 := strings.Cut(cond, " ")
	n, err := strconv.Atoi(lit)
	if !ok || !ok2 || err != nil || n < 1 || n > 255 || strings.TrimSpace(prop) != "SpellsCastThisTurn" {
		return lValue{}, refuse("PlayerCountOpponents$%s: only ConditionGE<n> SpellsCastThisTurn is read", rest)
	}
	lc.twoSeat("the opponents who cast n spells counted as the one opponent")
	cast := lValue{Kind: vThisTurnCast, Ref: lRef{Kind: rEachOpp}}
	return lValue{Kind: vCompare, N: n, Op: opGE, Sub: []lValue{cast, {Kind: vBranches, Sub: []lValue{cst(1), cst(0)}}}}, nil
}
