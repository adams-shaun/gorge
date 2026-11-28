package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

func init() { instrOp("EnterDevour", (*instrCtx).opEnterDevour) }

// opEnterDevour is the kop form of api:Sacrifice's Devour chain
// (lower_devour.go): OpExt ExtEnterDevour over the sacrificed permanents'
// filter (F), B the counters per sacrificed permanent.
func (ic *instrCtx) opEnterDevour() error {
	if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
		return err
	}
	buildEnterDevour(&ic.ki, ic.in.A.N, ic.ki.F)
	return nil
}
