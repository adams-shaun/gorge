package kgen

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

func init() { instrOp("ChangeCombatants", (*instrCtx).opChangeCombatants) }

// opChangeCombatants is the kop form of api:ChangeCombatants's lowered shape
// (lower_changecombatants.go): OpExt ExtChangeCombatants over the one target
// object, or over every battlefield permanent matching the Defined$ Valid
// filter (RefNone with F).
func (ic *instrCtx) opChangeCombatants() error {
	ki := &ic.ki
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtChangeCombatants))
	if ic.in.F != nil {
		return ic.objFilter(kop.ZoneBattlefield)
	}
	return nil
}
