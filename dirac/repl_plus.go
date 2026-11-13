package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// replPlusSrcPass is the direct replDamage sites' shape of replPlusBatch
// (trig_batches.go): a resolving op's damage pass whose exactly one event is
// matched by two or more pure plus-N replacements parks gorge's no-op CR
// 616.1 order ask before any dealing; the answer (interp_ask.go
// raGenReplOrder, plusBatchAsked) drops the parked event and re-executes the
// op, which deals every event with the summed amount (trig_oneormore.go's
// exemption). Two or more competing events would need one ask per event,
// which the op-resume shape does not model: the loud stall stays, as does
// a pass whose one competing event is not its first (gorge deals the earlier
// events before its ask and the answer's re-execution would deal them again).
// the answerDivide caller's (interp_gen.go), which passes park=false. The
// source is the pass's already-read one (controller ctrl, permanent srcSl or
// its card srcCard, keywords kw): parkNC is the collector the answer will
// use, so it is probed per event with the ask state saved and restored, and
// re-parked for the one competing event. handled reports that the pass was
// consumed (parked or stalled) and must not deal.
func (s *State) replPlusSrcPass(k *prep, ctrl uint8, srcSl Slot, srcCard CardID, kw kop.KwMask, codes []TCode, amts []int) bool {
	if !replPlusOnly(k.gen) || s.g.ro.n != 0 || s.g.ro.on || s.g.nc.n != 0 || s.g.plusBatchAsked {
		return false
	}
	saveRO, saveNC := s.g.ro, s.g.nc
	n, at := 0, -1
	for i := range codes {
		if amts[i] <= 0 {
			continue
		}
		if s.parkNC(k, ctrl, srcSl, srcCard, codes[i], amts[i], kw) {
			n, at = n+1, i
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
	case n == 1:
		s.parkNC(k, ctrl, srcSl, srcCard, codes[at], amts[at], kw)
		s.g.plusBatchAsked = true
		return true
	}
	return false
}
