package dirac

// payPickLife charges an elected pay-life cost (execPay's kop.PayLife arm): a
// life total below the count declines the payment (CR 119.4, 118.3; gorge
// counters the ward, rules unlessflow's guarded charge). It settles the
// payment either way, so execPay's case returns instead of also finishing it
// as paid.
//
//go:noinline
func (s *State) payPickLife(k *prep) {
	if int(s.pl[s.g.pickP].life) < int(s.g.pickLeft) {
		s.payDone(k, false)
		return
	}
	s.loseLife(k, s.g.pickP, int(s.g.pickLeft))
	s.payDone(k, true)
}
