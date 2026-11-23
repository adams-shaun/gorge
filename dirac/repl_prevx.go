package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// genState.prevX holds one rider byte per prevCol record: what the record's
// controller gets from the damage the record prevents (gorge's
// PreventionEffect$ True replacement with a body, damageReplacementPreventsWithRider).
// Layout of the byte:
//
//	bits 0-1  pxRider: the rider kind (pxRidGain, pxRidTok); 0 is a bare prevention
//	bit  2    pxEOC:     the record ends with the combat (Duration$
//	             UntilEndOfCombat: Suppressor Skyguard); repl_prevscope.go's
//	             prevEndCombat drops it on the way out of the end-of-combat step
//	bit  3    pxFrom:    reserved for a later rider
//	bit  4    pxPlayers: reserved for a later rider
//	bits 5-7  the rider's player (pxSeatShift)
const (
	pxRider   = 3
	pxRidGain = 1 // the player gains life equal to the damage prevented
	pxRidTok  = 2 // reserved: a token rider
	pxEOC     = 1 << 2
	pxFrom    = 1 << 3
	pxPlayers = 1 << 4

	pxSeatShift = 5
)

// the rider's player lives in three bits
const _ = uint(8 - MaxSeats)

// StallPrevRider is a rider prevention (gain the prevented life) on an event
// beside anything the kernel does not model: damage that can't be prevented
// (gorge runs the rider and keeps the damage), a parked or paused ask, another
// prevention record, a live PreventNext shield or another damage replacement
// (gorge asks the CR 616.1 order, and a body is never a bare prevention).
const StallPrevRider = 193

func init() { stallNames[StallPrevRider] = "prevrider" }

// DebugNoPrevRider (the rider lockstep mutation): the rider is skipped, and
// the prevention still applies.
var DebugNoPrevRider bool

// DebugNoPrevEndCombat (the lockstep mutation): an end-of-combat scoped
// record is not dropped on the way out of the end-of-combat step.
var DebugNoPrevEndCombat bool

// prevEndCombat drops every scoped record whose rider byte carries pxEOC
// (kop.PrevScopeEndCombat, Duration$ UntilEndOfCombat): gorge's
// expireUntilEndOfCombat runs on the transition that leaves StepEndCombat
// (CR 511.2/511.3; turn.go advance), so a second combat phase in the turn
// starts without it. It frees the slot, it does not spend it, so the next
// combat's record can reuse it.
func (s *State) prevEndCombat() {
	if DebugNoPrevEndCombat {
		return
	}
	for i := range s.g.prevCol {
		if s.g.prevX[i]&pxEOC != 0 {
			s.g.prevCol[i] = prevRec{}
			s.g.prevX[i] = 0
		}
	}
}

// prevXAny reports whether any record carries a rider.
func (s *State) prevXAny() bool {
	for _, x := range s.g.prevX {
		if x&pxRider != 0 {
			return true
		}
	}
	return false
}

// prevRiderMatch reports whether a rider record matches the damage event: a
// scoped record's gain rider (prevRecHit) or a source-keyed PreventNext
// shield's (kop.PrevNextFrom, Awe Strike, matched by the source at sl;
// pnxMatches, which no recipient-keyed record reaches here).
func (s *State) prevRiderMatch(k *prep, sl Slot, c CardID, t prevTgt, combat bool) bool {
	for i := range s.g.prevCol {
		if s.g.prevX[i]&(pxFrom|pxRidGain) == pxFrom|pxRidGain {
			if s.pnxMatches(i, TCode(NoTarget), sl, combat) {
				return true
			}
			continue
		}
		if s.g.prevX[i]&pxRider != 0 && s.prevRecHit(k, i, sl, c, t, combat) {
			return true
		}
	}
	return false
}

// prevRiderStall stalls StallPrevRider when the event has two or more
// candidates (count) and a rider record matches it: a body is never a bare
// prevention, so gorge asks the order. It reports whether it stalled.
func (s *State) prevRiderStall(k *prep, count int, sl Slot, c CardID, t prevTgt, combat bool) bool {
	if count < 2 || k.gen.anyRepl&rbPrevCol == 0 || !s.prevRiderMatch(k, sl, c, t, combat) {
		return false
	}
	s.capFull(StallPrevRider)
	return true
}

// protBeforeRepl reports whether the recipient (the permanent at sl, a player
// when NoSlot, controlled by rcp) is protected from the source: protection
// prevents the damage before any replacement (CR 702.16e; gorge emit).
func (s *State) protBeforeRepl(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, rcp uint8, sl Slot, combat bool) bool {
	g := k.gen
	return sl != NoSlot && (g.anyRepl&rbProtAll != 0 && s.kw(k, sl)&kop.KwProtectionAll != 0 ||
		g.anyRepl&rbProtCol != 0 && s.protFrom(k, sl, srcSl, srcCard, s.srcColors(k, srcSl, srcCard), combat)) ||
		sl == NoSlot && g.anyRepl&rbPlProt != 0 && s.plProtected(k, srcCtrl, rcp, combat)
}

// pnxOn reports a live PreventNext shield on the recipient code.
func (s *State) pnxOn(code TCode, combat bool) bool {
	for i := range s.g.prevCol {
		if r := &s.g.prevCol[i]; r.sc&prevNextSc != 0 && r.col != 0 && r.ctl == code && (combat || r.sc&prevNextCmb == 0) {
			return true
		}
	}
	return false
}

// pnxFromOn reports a live source-keyed PreventNext shield (kop.PrevNextFrom,
// Awe Strike) on the permanent at srcSl: beside a rider record it is another
// candidate of the event (gorge asks the CR 616.1 order, and a body is never
// a bare prevention).
func (s *State) pnxFromOn(srcSl Slot, combat bool) bool {
	if srcSl == NoSlot {
		return false
	}
	code := TCode(s.perms[srcSl].obj)
	for i := range s.g.prevCol {
		if r := &s.g.prevCol[i]; r.sc&prevNextSc != 0 && r.col != 0 && s.g.prevX[i]&pxFrom != 0 && r.ctl == code && (combat || r.sc&prevNextCmb == 0) {
			return true
		}
	}
	return false
}

// prevRiderApply runs the rider of the prevention record that matches a
// damage event of n (srcCtrl's source, to the recipient code): it gains the
// prevented life for the record's player. The caller prevents the event as for
// a bare record. Protection prevents the damage first and nothing is gained.
// Anything the kernel does not model beside the record stalls StallPrevRider.
func (s *State) prevRiderApply(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, code TCode, rcp uint8, sl Slot, combat bool, n int) {
	if k.gen.anyRepl&rbPrevCol == 0 || !s.prevXAny() {
		return
	}
	t := prevTgt{rcp, sl}
	at, hits := -1, 0
	for i := range s.g.prevCol {
		if s.prevRecHit(k, i, srcSl, srcCard, t, combat) {
			hits++
			if s.g.prevX[i]&pxRider != 0 {
				at = i
			}
		}
	}
	if at < 0 || n <= 0 {
		return
	}
	if s.protBeforeRepl(k, srcCtrl, srcSl, srcCard, rcp, sl, combat) {
		return // prevented before the replacement: no rider
	}
	if s.noPrev(k, combat) || s.g.ro.on || s.parkedAny() || hits > 1 || s.pnxOn(code, combat) || s.pnxFromOn(srcSl, combat) ||
		s.replOthersCompete(k, srcCtrl, srcSl, srcCard, rcp, sl, combat, false) {
		s.capFull(StallPrevRider) // loud: gorge runs the rider where the kernel cannot
		return
	}
	if DebugNoPrevRider {
		return
	}
	if x := s.g.prevX[at]; x&pxRider == pxRidGain {
		s.gainLife(k, x>>pxSeatShift, n)
	}
}
