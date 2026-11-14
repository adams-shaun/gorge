package dirac

// execSacSelfMandatory settles a trigger body's `Cost$ Mandatory Sac<1/CARDNAME>`
// (Promise of Bunrei) as gorge's triggered-cost window does (rules
// advanceTriggeredMandatory): unasked, the source being its only candidate. A
// source still on the battlefield under the payer's control, and not barred from
// being sacrificed, is sacrificed and the body runs on; otherwise the part cannot
// be paid and the body is skipped (triggeredCostDecline).
//
//go:noinline
func (s *State) execSacSelfMandatory(k *prep, p uint8, skip uint16) bool {
	if sl := s.resSrcSlot(k); sl != NoSlot && s.perms[sl].ctrl == p && !s.sacBlocked(k, sl) {
		s.paySacSelf(k)
		return true
	}
	if skip&pfIfPaid != 0 {
		s.res.pc += uint8(skip)
	}
	return true
}
