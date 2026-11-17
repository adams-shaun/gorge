package kgen

func init() {
	instrOp("Clash", (*instrCtx).opClash)
}

func (ic *instrCtx) opClash() error {
	buildClash(&ic.ki) // lower_clash.go
	return nil
}
