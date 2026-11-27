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
	// the kernel's copyEnterAttacking (copy_picked.go) attacks
	// nextSeat(ctrl); gorge asks the CR 508.4 choose:player
	// question, so the reduction is exact at two seats only.
	lc.twoSeat("RepeatEach batched copy enters attacking the next seat, not gorge's CR 508.4 defender ask")
	*flags = append(*flags, "Attacking")
	return nil
}
