package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Echo (CR 702.30a; Cradle Guard, Avalanche Riders, Mogg War Marshal).
// gorge kwEcho expands it to an upkeep trigger of ValidPlayer$ You carrying
// Echo$ True, a gate checked as the trigger fires only (engine/rules/echo.go
// echoGateHolds: the permanent's control-acquisition stamp against its
// controller's most recent upkeep), and the body DB$ Echo | Cost$ <cost>.
// startEcho asks the controller to pay the cost or sacrifice the source
// (echo_pay / echo_sac, over the same resPay window as a trigger's own cost).
//
// kgen lowers the trigger to Trig.Cond = CondEchoOwed with AfCheckOnTrigger,
// and the body to
//
//	ExtCumulative B=EchoNoCounter C=2   skips the rest when the source is gone
//	UnlessPaid You Mana cost            the pay-or-decline ask, skipping the sacrifice when paid
//	Sacrifice self                      a decline
//
// The gate. gorge stamps a permanent's acquisition (entry or control change)
// and compares it with the controller's last recorded upkeep. A permanent is
// owed echo at a controller's upkeep exactly when it came under their control
// since the beginning of their previous turn (an acquisition during that
// upkeep included), which is what pfSick holds until that player's untap
// step clears it: echoSnap copies it into genState.echoOwed as the untap step
// begins, before the clearing, and CondEchoOwed reads that copy at the
// upkeep that follows. Both stamps are written by an entry and by a control
// change, and the copy is rewritten at every untap, so a slot an earlier
// permanent left holds nothing the gate can read.

// echoSnap records which of the active player's permanents are still sick as
// their untap step begins.
func (s *State) echoSnap() {
	var owed slotMask
	for m := s.ctrlM[s.active]; m.any(); m = m.rest() {
		if sl := m.first(); s.perms[sl].flags&pfSick != 0 {
			owed.set(sl)
		}
	}
	s.g.echoOwed = owed
}

// tableHasEcho reports whether some trigger of t is gated by CondEchoOwed.
func tableHasEcho(t *Table) bool {
	for i := range t.Conds {
		if t.Conds[i].Kind == kop.CondEchoOwed {
			return true
		}
	}
	return false
}
