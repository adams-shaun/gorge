package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// oppCastCondValue lowers PlayerCountOpponents$ConditionGE<n>
// SpellsCastThisTurn: the number of the controller's living opponents who
// cast n or more spells this turn (gorge playerCountCondition over
// opponentGroup, each member's SpellsCastThisTurnBy, the per-caster
// Player.SpellsCastThisTurn tally). Mindbreak Trap's "if an opponent cast
// three or more spells this turn". The count is seat-exact: the kernel's
// group count walks every living opponent and compares that opponent's own
// per-caster tally (ExOppExtreme with OppExtCountCmp|OppExtSpellsCast), so a
// pod reaches when any one opponent has cast n. Any other Condition head
// (another operator, an SVar right-hand side, another member property)
// refuses by name.
func (lc *lowerCtx) oppCastCondValue(rest string) (lValue, error) {
	cond, ok := strings.CutPrefix(strings.TrimSpace(rest), "ConditionGE")
	lit, prop, ok2 := strings.Cut(cond, " ")
	n, err := strconv.Atoi(lit)
	if !ok || !ok2 || err != nil || n < 1 || n > 255 || strings.TrimSpace(prop) != "SpellsCastThisTurn" {
		return lValue{}, refuse("PlayerCountOpponents$%s: only ConditionGE<n> SpellsCastThisTurn is read", rest)
	}
	return lValue{Kind: vOppExtreme, N: kop.OppExtCountCmp | kop.OppExtSpellsCast, Op: opGE, Sub: []lValue{cst(n)}}, nil
}
