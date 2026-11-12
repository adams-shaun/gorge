package dirac

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
// step clears it: echoSnap copies it into the arena mask mechEchoOwed as the untap step
// begins, before the clearing, and CondEchoOwed reads that copy at the
// upkeep that follows. Both stamps are written by an entry and by a control
// change, and the copy is rewritten at every untap, so a slot an earlier
// permanent left holds nothing the gate can read.
//
// Phasing changes no control (CR 702.26d) and leaves gorge's stamp alone, so
// a permanent phased out since it came under its controller's control still
// owes echo at the upkeep after it phases back in (CR 702.26a: during that
// player's untap step). A phased-out permanent is off ctrlM, and phaseUntap
// clears its pfSick as it phases it in, so echoSnap reads the active player's
// phased-out slots too, before phaseUntap runs (kw_phasing.go).

// echoSnap records which of the active player's permanents, phased out ones
// included, are still sick as their untap step begins (arena mask
// mechEchoOwed).
func (s *State) echoSnap(k *prep) {
	s.mSlZero(k, mechEchoOwed)
	m := s.ctrlM[s.active]
	if k.phasing {
		for h := s.phasedSlots(); h.any(); h = h.rest() {
			if sl := h.first(); s.perms[sl].ctrl == s.active {
				m.set(sl)
			}
		}
	}
	for ; m.any(); m = m.rest() {
		if sl := m.first(); s.perms[sl].flags&pfSick != 0 {
			s.mSlSet(k, mechEchoOwed, sl)
		}
	}
}
