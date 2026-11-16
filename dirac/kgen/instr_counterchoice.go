package kgen

func init() {
	instrOp("PutCounterChoice", (*instrCtx).opPutCounterChoice)
}

func (ic *instrCtx) opPutCounterChoice() error {
	return buildPutCounterChoice(&ic.ki, ic.in.CS) // lower_counterchoice.go
}
