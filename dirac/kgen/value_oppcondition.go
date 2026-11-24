package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// oppCastCondValue lowers PlayerCountOpponents$ConditionGE<n> <property>: the
// number of the controller's living opponents whose own figure is n or more
// (gorge playerCountCondition over opponentGroup, each member's property read
// from that member's side). The count is seat-exact: the kernel's group count
// walks every living opponent (ExOppExtreme with OppExtCountCmp), so a pod
// reaches when any one opponent does.
//
//   - SpellsCastThisTurn: the member's per-caster tally (OppExtSpellsCast;
//     Mindbreak Trap's "if an opponent cast three or more spells this turn").
//   - ThisTurnEntered_<spec>: the entered-ledger count read with the member as
//     the controller (OppExtMemberExpr; Whiplash Trap's "an opponent had two
//     or more creatures enter the battlefield under their control"). The
//     property is the ordinary Count$ThisTurnEntered_ head, so its spec
//     grammar and refusals are that head's; a $<property> sum tail refuses
//     (the member's figure is a count).
//
// Any other Condition head (another operator, an SVar right-hand side,
// another member property) refuses by name.
func (lc *lowerCtx) oppCastCondValue(rest string) (lValue, error) {
	cond, ok := strings.CutPrefix(strings.TrimSpace(rest), "ConditionGE")
	lit, prop, ok2 := strings.Cut(cond, " ")
	n, err := strconv.Atoi(lit)
	prop = strings.TrimSpace(prop)
	if !ok || !ok2 || err != nil || n < 1 || n > 255 {
		return lValue{}, refuse("PlayerCountOpponents$%s: only ConditionGE<n> SpellsCastThisTurn and ThisTurnEntered_ are read", rest)
	}
	switch {
	case prop == "SpellsCastThisTurn":
		return lValue{Kind: vOppExtreme, N: kop.OppExtCountCmp | kop.OppExtSpellsCast, Op: opGE, Sub: []lValue{cst(n)}}, nil
	case strings.HasPrefix(prop, "ThisTurnEntered_") && !strings.Contains(prop, "$"):
		fig, err := lc.countValue(prop, 0)
		if err != nil {
			return lValue{}, err
		}
		return lValue{Kind: vOppExtreme, N: kop.OppExtCountCmp | kop.OppExtMemberExpr, Op: opGE, Sub: []lValue{cst(n), fig}}, nil
	}
	return lValue{}, refuse("PlayerCountOpponents$%s: only ConditionGE<n> SpellsCastThisTurn and ThisTurnEntered_ are read", rest)
}
