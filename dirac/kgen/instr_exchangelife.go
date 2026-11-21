package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

func init() { instrOp("ExchangeLife", (*instrCtx).opExchangeLife) }

// opExchangeLife is the kop form of api:ExchangeLife (api_exchangelife.go):
// OpExt ExtExchangeLife over the one targeted player.
func (ic *instrCtx) opExchangeLife() error {
	ic.ki.Op, ic.ki.A = kop.OpExt, kop.Const(int(kop.ExtExchangeLife))
	return nil
}
