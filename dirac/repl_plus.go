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
// op-resume shape does not model. With seq (a damage batch's call), a pass
// whose one competing event is not its first deals its hits one at a time
// instead of stalling when that event's competition is the depleting
// PreventNext shields' ask (a pnx park): gorge deals the earlier events
// before its ask and the rest after the answer (each event's replacements
// are collected as the event happens, CR 616.1e), so the seq deals the hits
// before at now, the asked hit parks the ask and the seq resumes with the
// rest after the answer (batchSeqStart / damageBatchNext). A later plus-N or
// other competition keeps the loud stall. A park
// whose answer cannot re-execute the op is left to dmgSrcOrderStalls, which
// caps it loudly. The source is the pass's already-read one (controller
// ctrl, permanent srcSl or its card srcCard, keywords kw): parkNC is the
// collector the answer will use, so it is probed per event with the ask state
// saved and restored, and re-parked for the one competing event. handled
// reports that the pass was consumed (parked or stalled) and must not deal.
func (s *State) replPlusSrcPass(k *prep, ctrl uint8, srcSl Slot, srcCard CardID, kw kop.KwMask, codes []TCode, amts []int, seq bool) bool {
	if s.g.ro.n != 0 || s.g.ro.on || s.g.nc.n != 0 || s.g.plusBatchAsked {
		return false
	}
	saveRO, saveNC := s.g.ro, s.g.nc
	n, at, shape, pnx := 0, -1, uint8(0), false
	for i := range codes {
		if amts[i] <= 0 {
			continue
		}
		if s.parkNC(k, ctrl, srcSl, srcCard, codes[i], amts[i], kw) {
			n, at, shape = n+1, i, s.parkShape()
			pnx = pnx || s.g.ro.pnx != 0
		}
		s.g.ro, s.g.nc = saveRO, saveNC
	}
	switch {
	case n >= 2:
		// gorge asks once per event; a multi-event pass is not modelled
		s.capFull(StallReplPlus)
		return true
	case n == 1 && at > 0 && (!seq || !pnx) || n == 1 && at > 0 && shape == replParkOther:
		// gorge deals the events before the competing one before its ask (each
		// event's replacements are collected as the event happens, CR 616.1e),
		// and the answer's re-execution would deal them again: the op-resume
		// shape cannot shrink the pass, so the loud stall stays. The seq
		// exception (below) is the depleting shields' ask only: the
		// plus-N/other competitions at a later event keep the stall.
		s.capFull(StallReplPlus)
		return true
	case n == 1 && at > 0 && pnx:
		// a damage batch's seq: gorge deals the events before the competing
		// one before its ask and the rest after the answer; the seq deals its
		// hits one at a time (damageTo, the batch bracket spanning it like a
		// sweep's), hit at parks the ask and the seq resumes with the rest
		// after the answer (interp_ask's raGenReplOrder damageSeq resume,
		// dealNC). The op is re-executed on the resume, so no hit is dealt
		// twice and no earlier hit is held back.
		s.batchSeqStart(k, codes, amts)
		return true
	case n == 1 && shape != replParkOther:
		s.parkNC(k, ctrl, srcSl, srcCard, codes[at], amts[at], kw)
		s.g.plusBatchAsked = true
		return true
	}
	return false
}

// batchSeqStart starts a damage batch's parked seq (replPlusSrcPass's at>0
// arm): the hits deal one at a time (damageBatchNext), the asked one parks
// the CR 616.1 ask and the seq resumes with the rest after the answer.
func (s *State) batchSeqStart(k *prep, codes []TCode, amts []int) {
	s.g.dmgSeqOn, s.g.dmgSeqN, s.g.dmgSeqI, s.g.dmgSeqPl = true, uint8(len(codes)), 0, 0
	s.dmgBatchOpen()
	s.damageBatchNext(k, codes, amts)
}

// parkShape classifies the ask parkNC just parked (probed state): the mixed
// competition walked as picked (ncPend), every candidate a commuting plus-N
// addition (the sum), a depleting PreventNext shields' ask walked as picked
// whose record the re-executed op deals from (prevNextNC, replDamagePend), or
// one the op-resume shape cannot re-execute.
func (s *State) parkShape() uint8 {
	ro := &s.g.ro
	switch {
	case s.g.nc.n == ncPend:
		return replParkPend
	case ro.n != 0 && ro.plus != 0 && ro.plus == (1<<ro.n)-1 && // the low mask only: no raise body above them
		ro.prev == 0 && ro.shld == 0 && ro.pnx == 0 && ro.pcol == 0 && s.g.nc.n > 0:
		return replParkPlus
	case ro.n != 0 && ro.pnx != 0 && ro.plus == 0 && ro.shld == 0 && ro.pcol == 0 && s.g.nc.n > 0:
		return replParkPend // the shields are spent as picked; the record's amount is the picked order's
	}
	return replParkOther
}
