package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugOppMemberFromCtrl is a test-only mutation (TestOppMemberExprMutation):
// the member's figure reads from the controller's own side, as if every
// opponent's YouCtrl were the caster's.
var DebugOppMemberFromCtrl bool

// memberFigure is groupCountCmp's OppExtMemberExpr figure: Sub[1] evaluated
// with seat p as the controller (gorge playerCountCondition's
// playerMemberProperty, which reads the property "FROM THAT MEMBER'S OWN
// PERSPECTIVE"). Whiplash Trap's "an opponent had two or more creatures enter
// under their control" is ThisTurnEntered_Battlefield_Creature.YouCtrl read
// once per opponent.
func (s *State) memberFigure(k *prep, x *kop.Expr, ctx evalCtx, p uint8) int {
	if !DebugOppMemberFromCtrl {
		ctx.ctrl = p
	}
	return s.evalV(k, x.Sub[1], ctx)
}
