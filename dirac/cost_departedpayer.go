package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// execPayer is the seat an OpUnlessPaid / OpIfPaid / OpUnlessSwitched asks,
// or 255 for none. A player who has left the game is asked nothing (CR
// 800.4a, 800.4f; gorge effects/unless.go unlessPayerTargetsFor skips a
// Lost seat), so a departed payer is no payer: execPay's 255 arm takes the
// decline orientation, as gorge does. At two seats a loss ends the game and
// out is always false.
func (s *State) execPayer(k *prep, r kop.Ref) uint8 {
	p := s.payerOf(k, r)
	if p != 255 && s.out(p) {
		return 255
	}
	return p
}
