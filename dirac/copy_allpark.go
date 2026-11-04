package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A copy-all sweep (CR 303.4f, gorge CopyPermanent's Defined arm) parks on a
// copy's as-enters ask: the sweep's record lives in genState.sweep and the
// raGenTokChoice / raGenEnchant answer arms resume it here. Gorge's zone
// batch is carried across the parked ask (its Ctx clone, clone_gen.go), so
// the copies before and after the answer share one batch: the kernel holds
// the sweep's enters checks across the answer (genState.held, the holdBatch
// of execChangeZoneAll, interp_gen.go) and fires them once, in entry order,
// when the sweep ends (fireHeld) -- the answered copy's check waits there
// too (its entry, interp_ask.go).

// sweepSusp is a copy-all sweep parked on a copy's as-enters ask.
type sweepSusp struct {
	on     bool  // the sweep is parked on an ask
	i, n   uint8 // the source whose copy asked; the sweep's source count
	j      int16 // the next copy of source i to mint
	minted int16 // the copies entered in earlier segments
	ord    [MaxPerm]Slot
	cnt    [MaxPerm]int16
}

// sweepTmpl is the answered copy's template: its source's slot while that is
// still on the battlefield (CR 707.9b), NoSlot once it left.
func (s *State) sweepTmpl() Slot {
	w := &s.g.sweep
	if w.on && s.occ.has(w.ord[w.i]) {
		return w.ord[w.i]
	}
	return NoSlot
}

// defValidMulti counts a copyAllEach resolution that entered two or more
// copies (TestDefvalidLockstep's minimum).
func (s *State) defValidMulti(n int) {
	if DebugDefValidMulti != nil && n >= 2 {
		*DebugDefValidMulti++
	}
}

// sweepRun mints a copy-all sweep's copies over the set ord/cnt from source
// i0, copy j0 on, entering each under e.ctrl with eb (the op's copy
// exceptions ride in, as copyTokens' do). It returns true when a copy's
// as-enters ask parked the sweep (the pose in w) or the board filled.
//
//go:noinline
func (s *State) sweepRun(k *prep, in *kop.Instr, w *sweepSusp, eb *entBatch, seg *int, ord *[MaxPerm]Slot, cnt *[MaxPerm]int16, i0, j0 int) bool {
	e := &s.res.e
	for i := i0; i < int(w.n); i++ {
		sl := ord[i]
		if !s.occ.has(sl) {
			continue // the source left the battlefield mid-sweep
		}
		// j0 counts the answered copy of the parked source only; every
		// later source mints all of its copies
		js := j0
		j0 = 0
		c := s.cardAt(sl)
		aura := k.cards[c].types&kop.TAura != 0 && k.t.Cards[c].Enchant.Kind == kop.TgtObject
		for j := js; j < int(cnt[i]); j++ {
			if s.nearCapacity() {
				s.stall(StallBoard)
				return true
			}
			host := NoSlot
			if aura {
				var buf [MaxOptions]uint8
				switch s.enchantCands(k, c, e.ctrl, &buf) {
				case 0:
					continue // CR 303.4g: the Aura copy isn't created
				case 1:
					host = s.objs[buf[0]].slot
				default:
					t := s.newToken(c, e.ctrl)
					s.objs[t].of |= ofCopy | ofPending
					s.res.lastTok = t
					w.on, w.i, w.j = true, uint8(i), int16(j+1)
					s.res.ask, s.res.askP = raGenEnchant, e.ctrl
					return true
				}
			}
			kind := s.enterChoiceKind(k, c)
			if kind == 0 && k.gen.anyBecomeCopy {
				kind = s.cloneAskKind(k, c) // ctr_entryperkind.go: a copy of a clone card is asked the clone election
				if kind != 0 && s.perms[sl].ovr.flags&ovCopy != 0 {
					// a token copy of a permanent that is itself a copy is
					// minted on that permanent's printed card while it waits
					// for the answer (copyTokenBasis): not modelled
					s.capFull(StallCopyAllAsk)
					return true
				}
			}
			if kind != 0 {
				// an as-enters choice: the copy waits outside every zone
				// (zone 0) for it, as a permanent spell does on the stack
				// (preEnterAsk)
				t := s.newToken(c, e.ctrl)
				s.objs[t].of |= ofCopy | ofPending
				s.res.lastTok = t
				w.on, w.i, w.j = true, uint8(i), int16(j+1)
				s.g.pre = preAsk{obj: t, kind: kind}
				s.res.ask, s.res.askP = raGenTokChoice, e.ctrl
				return true
			}
			t := s.newToken(c, e.ctrl)
			s.objs[t].of |= ofCopy
			s.copyTokenEnterFrom(k, in, t, host, sl, eb)
			*seg++
		}
	}
	return false
}

// sweepResume finishes a copy-all sweep parked on the answered ask: the
// answered copy has just entered, the copies after it mint now. The sweep's
// enters checks (the earlier segments' and this one's, genState.held) fire
// once, in entry order, at the sweep's end -- gorge's zone batch spans the
// parked ask (copy_allpark.go). True when the sweep parked again or stopped
// at a stall.
func (s *State) sweepResume(k *prep, in *kop.Instr) bool {
	w := &s.g.sweep
	if !w.on {
		return false
	}
	i, j, entered := int(w.i), int(w.j), int(w.minted)
	w.on = false
	w.minted++ // the answered copy entered
	var eb entBatch
	seg := 0
	parked := s.sweepRun(k, in, w, &eb, &seg, &w.ord, &w.cnt, i, j)
	if parked && s.g.stalled == 0 {
		w.minted = int16(entered + 1 + seg)
		s.holdBatch(&eb) // this segment's checks join the held ones
		return true
	}
	s.defValidMulti(entered + 1 + seg)
	s.holdBatch(&eb)
	if s.g.held != 0 {
		s.fireHeld(k, s.g.retTS) // the whole sweep's checks, in entry order
	} else {
		s.fireEntries(k, &eb)
	}
	return parked
}
