package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The probe's park shapes (parkShape).
const (
	replParkOther uint8 = iota // a park the op-resume shape cannot re-execute (a shield candidate is spent as picked, replNCPickShield)
	replParkPend               // the mixed competition walked as picked (repl_pluswalk.go)
	replParkPlus               // every candidate a commuting plus-N addition: the sum is the amount in every order
)

// replPlusSrcPass is the direct replDamage sites' shape of replPlusBatch
// (trig_batches.go): a resolving op's damage pass whose exactly one event is
// matched by two or more damage replacements parks gorge's CR 616.1 order ask
// before any dealing; the answer re-executes the op (interp_ask.go,
// plusBatchAsked), which deals every event — the parked event from the
// answer's record when its competition is the mixed one the picked order
// decides (a plus-N addition or a raise beside a doubling: parkNC parked it
// with the running amount and a sentinel ncDmg.n, replDamagePend deals it and
// consumes the record), else from replDamage's fixed order (a pure plus-N
// competition's sum, a prevention's 0: every order lands the same amount).
// Two or more competing events would need one ask per event, which the
// op-resume shape does not model, as does a pass whose one competing event is
// not its first (gorge deals the earlier events before its ask and the
// answer's re-execution would deal them again): the loud stall stays. A park
// whose answer cannot re-execute the op is left to dmgSrcOrderStalls, which
// caps it loudly. The source is the pass's already-read one (controller
// ctrl, permanent srcSl or its card srcCard, keywords kw): parkNC is the
// collector the answer will use, so it is probed per event with the ask state
// saved and restored, and re-parked for the one competing event. handled
// reports that the pass was consumed (parked or stalled) and must not deal.
func (s *State) replPlusSrcPass(k *prep, ctrl uint8, srcSl Slot, srcCard CardID, kw kop.KwMask, codes []TCode, amts []int) bool {
	if s.g.ro.n != 0 || s.g.ro.on || s.g.nc.n != 0 || s.g.plusBatchAsked {
		return false
	}
	saveRO, saveNC := s.g.ro, s.g.nc
	n, at, shape := 0, -1, uint8(0)
	for i := range codes {
		if amts[i] <= 0 {
			continue
		}
		if s.parkNC(k, ctrl, srcSl, srcCard, codes[i], amts[i], kw) {
			n, at, shape = n+1, i, s.parkShape()
		}
		s.g.ro, s.g.nc = saveRO, saveNC
	}
	switch {
	case n >= 2:
		// gorge asks once per event; a multi-event pass is not modelled
		s.capFull(StallReplPlus)
		return true
	case n == 1 && at > 0:
		// gorge deals the events before the competing one before its ask (each
		// event's replacements are collected as the event happens, CR 616.1e),
		// and the answer's re-execution would deal them again: the op-resume
		// shape cannot shrink the pass, so the loud stall stays
		s.capFull(StallReplPlus)
		return true
	case n == 1 && shape != replParkOther:
		s.parkNC(k, ctrl, srcSl, srcCard, codes[at], amts[at], kw)
		s.g.plusBatchAsked = true
		return true
	}
	return false
}

// parkShape classifies the ask parkNC just parked (probed state): the mixed
// competition walked as picked (ncPend), every candidate a commuting plus-N
// addition (the sum), or one the op-resume shape cannot re-execute.
func (s *State) parkShape() uint8 {
	ro := &s.g.ro
	switch {
	case s.g.nc.n == ncPend:
		return replParkPend
	case ro.n != 0 && ro.plus != 0 && ro.plus == (1<<ro.n)-1 && // the low mask only: no raise body above them
		ro.prev == 0 && ro.shld == 0 && ro.pnx == 0 && ro.pcol == 0 && s.g.nc.n > 0:
		return replParkPlus
	}
	return replParkOther
}
