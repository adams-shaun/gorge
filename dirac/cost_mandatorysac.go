package dirac

// payMandatorySacSelf settles a trigger's Cost$ Mandatory Sac<1/CARDNAME>
// (kop.PayBSubSacSelfMust; gorge cumulative.go advanceTriggeredMandatory):
// there is no "may", so no pay/decline ask is posed. The source, the one
// candidate, is sacrificed unasked when it can be (still the payer's, not
// barred from being sacrificed); a cost that cannot be paid skips the body,
// the instructions the OpIfPaid covers (gorge triggeredCostDecline).
//
//go:noinline
func (s *State) payMandatorySacSelf(k *prep, p uint8, skip uint8) {
	sl := s.resSrcSlot(k)
	if sl != NoSlot && s.perms[sl].ctrl == p && !s.sacBlocked(k, sl) {
		s.paySacSelf(k)
		return
	}
	s.res.pc += skip
}
