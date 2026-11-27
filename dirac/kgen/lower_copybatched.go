package kgen

// copyBatchedAttacking reads TokenAttacking$ on a CopyPermanent made by a
// RepeatEach batched body (Mirror-Style Master) and appends the Attacking
// flag. It is a no-op for a picked copy or outside a batched body.
func (lc *lowerCtx) copyBatchedAttacking(ps *params, picked bool, flags *[]string) error {
	if picked || lc.repeatCard == 0 || !lc.repeatUseImp {
		return nil
	}
	v, ok := ps.get("TokenAttacking")
	if !ok {
		return nil
	}
	// the copy enters attacking the attack trigger's defender, as gorge's
	// copyAttackRider resolves TokenAttacking$ True
	if v != "True" {
		return refuse("CopyPermanent TokenAttacking %q", v)
	}
	if _, err := lc.tokenAttacking(v); err != nil {
		return err
	}
	// the kernel's copyEnterAttacking resolves TokenAttacking$ True through
	// the same CR 508.4 ask: the batched loop is a resumable frame
	// (act_repeatcards.go), so the copy's defender pick parks it before the
	// copy enters. The reading is seat-exact.
	*flags = append(*flags, "Attacking")
	return nil
}
